import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { Button } from "../components/Button";
import { Card } from "../components/Card";
import { SudokuBoard } from "../components/SudokuBoard";
import { api } from "../lib/api";

function cloneBoard(board) {
  return board.map((row) => [...row]);
}

function countEmptyCells(board) {
  return board.flat().filter((value) => value === 0).length;
}

function fallbackAvatar(label = "?") {
  const safe = (label || "?").slice(0, 1).toUpperCase();
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
      <rect width="64" height="64" rx="22" fill="#161616" />
      <text x="50%" y="54%" text-anchor="middle" dominant-baseline="middle"
        font-family="Avenir Next, Segoe UI, sans-serif" font-size="28" font-weight="700" fill="#fffaf0">
        ${safe}
      </text>
    </svg>
  `;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

function formatTimer(seconds) {
  const minutes = Math.floor(seconds / 60)
    .toString()
    .padStart(2, "0");
  const remainder = (seconds % 60).toString().padStart(2, "0");
  return `${minutes}:${remainder}`;
}

function difficultyLabel(value) {
  switch (value) {
    case "easy":
      return "Easy";
    case "medium":
      return "Medium";
    case "hard":
      return "Hard";
    default:
      return "Daily";
  }
}

export function DailyPage({ accessToken }) {
  const [daily, setDaily] = useState(null);
  const [leaderboard, setLeaderboard] = useState([]);
  const [board, setBoard] = useState(null);
  const [mistakes, setMistakes] = useState(0);
  const [filledCorrect, setFilledCorrect] = useState(0);
  const [completed, setCompleted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(true);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);
  const timerRef = useRef(null);
  const startRef = useRef(Date.now());
  const submitLockRef = useRef(false);

  const totalTarget = useMemo(
    () => (daily?.puzzle?.initialBoard ? countEmptyCells(daily.puzzle.initialBoard) : 0),
    [daily]
  );

  function resetRun(puzzle) {
    setBoard(cloneBoard(puzzle.initialBoard));
    setMistakes(0);
    setFilledCorrect(0);
    setCompleted(false);
    setElapsedSeconds(0);
    submitLockRef.current = false;
    startRef.current = Date.now();
    if (timerRef.current) {
      window.clearInterval(timerRef.current);
    }
    timerRef.current = window.setInterval(() => {
      setElapsedSeconds(Math.floor((Date.now() - startRef.current) / 1000));
    }, 1000);
  }

  async function enrichLeaderboard(entries) {
    const enriched = await Promise.all(
      entries.map(async (entry) => {
        try {
          const user = await api.publicUser(entry.userId, accessToken);
          return { ...entry, user };
        } catch {
          return entry;
        }
      })
    );
    setLeaderboard(enriched);
  }

  async function loadDaily() {
    setLoading(true);
    setNotice("");
    try {
      const [dailyPayload, leaderboardPayload] = await Promise.all([
        api.daily(accessToken),
        api.dailyLeaderboard(accessToken)
      ]);
      setDaily(dailyPayload);
      const completedResult = dailyPayload?.myResult;
      if (completedResult?.completed) {
        if (timerRef.current) {
          window.clearInterval(timerRef.current);
          timerRef.current = null;
        }
        setBoard(null);
        setCompleted(true);
        setMistakes(completedResult.mistakesCount || 0);
        setElapsedSeconds(Math.floor((completedResult.completionTimeMs || 0) / 1000));
        setFilledCorrect(countEmptyCells(dailyPayload.puzzle.initialBoard));
        submitLockRef.current = true;
        setNotice(`You already cleared today's daily. Score ${completedResult.score}.`);
      } else {
        resetRun(dailyPayload.puzzle);
      }
      await enrichLeaderboard(Array.isArray(leaderboardPayload) ? leaderboardPayload : []);
    } catch (error) {
      setNotice(error.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadDaily();
    return () => {
      if (timerRef.current) {
        window.clearInterval(timerRef.current);
      }
    };
  }, [accessToken]);

  async function submitDailyResult(nextMistakes, completionTimeMs) {
    if (!daily?.challenge?.id || submitLockRef.current) {
      return;
    }
    submitLockRef.current = true;
    setSubmitting(true);
    try {
      const result = await api.submitDaily(
        {
          completionTimeMs,
          mistakes: nextMistakes,
          hintsUsed: 0
        },
        accessToken
      );
      setNotice(`Daily complete. Score ${result.score}.`);
      const latestLeaderboard = await api.dailyLeaderboard(accessToken);
      await enrichLeaderboard(Array.isArray(latestLeaderboard) ? latestLeaderboard : []);
    } catch (error) {
      submitLockRef.current = false;
      setNotice(error.message);
    } finally {
      setSubmitting(false);
    }
  }

  function onCellChange(row, col, value) {
    if (!daily?.puzzle || completed) {
      return;
    }

    const numeric = Number(value);
    const nextValue = Number.isNaN(numeric) ? 0 : numeric;
    const nextBoard = cloneBoard(board);
    nextBoard[row][col] = nextValue;
    setBoard(nextBoard);

    let nextMistakes = mistakes;
    if (nextValue !== 0 && nextValue !== daily.puzzle.solution[row][col]) {
      nextMistakes += 1;
      setMistakes(nextMistakes);
    }

    let correct = 0;
    for (let rowIndex = 0; rowIndex < 9; rowIndex += 1) {
      for (let colIndex = 0; colIndex < 9; colIndex += 1) {
        if (daily.puzzle.initialBoard[rowIndex][colIndex] !== 0) {
          continue;
        }
        if (nextBoard[rowIndex][colIndex] === daily.puzzle.solution[rowIndex][colIndex]) {
          correct += 1;
        }
      }
    }
    setFilledCorrect(correct);

    if (correct >= totalTarget) {
      setCompleted(true);
      if (timerRef.current) {
        window.clearInterval(timerRef.current);
        timerRef.current = null;
      }
      const completionTimeMs = Date.now() - startRef.current;
      void submitDailyResult(nextMistakes, completionTimeMs);
    }
  }

  return (
    <div className="space-y-4">
      <Card title="Daily Challenge" subtitle={daily?.challenge?.challengeDate || "Today’s race"}>
        <div className="grid grid-cols-3 gap-3">
          <div className="rounded-3xl bg-white/80 px-4 py-4 text-center">
            <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Timer</div>
            <div className="mt-1 text-xl font-black">{formatTimer(elapsedSeconds)}</div>
          </div>
          <div className="rounded-3xl bg-white/80 px-4 py-4 text-center">
            <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Mistakes</div>
            <div className="mt-1 text-xl font-black">{mistakes}</div>
          </div>
          <div className="rounded-3xl bg-white/80 px-4 py-4 text-center">
            <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Progress</div>
            <div className="mt-1 text-xl font-black">
              {filledCorrect}/{totalTarget}
            </div>
          </div>
        </div>

        <div className="mt-4 rounded-3xl bg-white/80 px-4 py-4">
          <div className="text-sm text-slatewarm">Today&apos;s board</div>
          <div className="mt-1 text-xl font-black">{loading ? "Loading..." : difficultyLabel(daily?.puzzle?.difficulty)}</div>
        </div>

        {completed ? (
          <div className="mt-4 overflow-hidden rounded-[2rem] border border-ink/10 bg-[linear-gradient(180deg,rgba(255,250,240,0.98),rgba(247,239,220,0.98))] shadow-card">
            <div className="border-b border-ink/10 px-5 py-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="text-[11px] uppercase tracking-[0.3em] text-slatewarm">Daily Cleared</div>
                  <div className="mt-2 text-2xl font-black text-ink">Today&apos;s board is solved.</div>
                  <div className="mt-2 max-w-[17rem] text-sm leading-6 text-slatewarm">
                    This square is closed for today. Come back tomorrow for the next shared sudoku, or keep climbing in ranked.
                  </div>
                </div>
                <div className="grid grid-cols-3 gap-[3px] rounded-[1.35rem] border-2 border-ink bg-white p-2 shadow-sm">
                  {Array.from({ length: 9 }).map((_, index) => {
                    const filled = [0, 1, 2, 4, 6, 7].includes(index);
                    return (
                      <div
                        key={index}
                        className={`h-5 w-5 rounded-[0.35rem] ${filled ? "bg-gold/80" : "bg-sand"}`}
                      />
                    );
                  })}
                </div>
              </div>
            </div>

            <div className="grid grid-cols-3 gap-[1px] bg-ink/10">
              <div className="bg-white/80 px-4 py-4 text-center">
                <div className="text-[11px] uppercase tracking-[0.22em] text-slatewarm">Final Time</div>
                <div className="mt-1 text-2xl font-black text-ink">{formatTimer(elapsedSeconds)}</div>
              </div>
              <div className="bg-white/70 px-4 py-4 text-center">
                <div className="text-[11px] uppercase tracking-[0.22em] text-slatewarm">Mistakes</div>
                <div className="mt-1 text-2xl font-black text-ink">{mistakes}</div>
              </div>
              <div className="bg-white/80 px-4 py-4 text-center">
                <div className="text-[11px] uppercase tracking-[0.22em] text-slatewarm">Score</div>
                <div className="mt-1 text-2xl font-black text-ink">{daily?.myResult?.score ?? 0}</div>
              </div>
            </div>

            <div className="flex items-center gap-2 border-t border-ink/10 bg-white/70 px-5 py-3 text-sm text-slatewarm">
              <span className="inline-flex h-2.5 w-2.5 rounded-full bg-coral" />
              Your run is locked into today&apos;s leaderboard.
            </div>

            <div className="px-5 py-5">
              <div className="flex gap-3">
              <Link className="flex-1" to="/play">
                <Button className="w-full" type="button" variant="accent">
                  Play Ranked
                </Button>
              </Link>
              <Button className="flex-1" type="button" variant="soft" onClick={loadDaily}>
                Come Tomorrow
              </Button>
              </div>
            </div>
          </div>
        ) : board && daily?.puzzle ? (
          <div className="mt-4">
            <SudokuBoard board={board} initialBoard={daily.puzzle.initialBoard} onChange={onCellChange} />
          </div>
        ) : null}

        {!completed ? (
          <div className="mt-4 flex gap-3">
            <Button className="flex-1" variant="accent" loading={submitting} onClick={() => resetRun(daily.puzzle)} disabled={!daily?.puzzle}>
              Restart Daily
            </Button>
            <Button className="flex-1" variant="soft" onClick={loadDaily}>
              Refresh
            </Button>
          </div>
        ) : null}
        {notice && !completed ? <p className="mt-3 text-sm text-slatewarm">{notice}</p> : null}
      </Card>

      <Card title="Daily Leaders" subtitle="Best runs on today’s exact sudoku.">
        {leaderboard.length === 0 ? (
          <div className="rounded-3xl bg-white/75 px-4 py-4 text-sm text-slatewarm">
            No daily runs yet. Solve today’s board and claim the first spot.
          </div>
        ) : (
          <div className="space-y-3">
            {leaderboard.map((entry, index) => {
              const username = entry.user?.username || `Player ${String(entry.userId || "unknown").slice(0, 8)}`;
              return (
                <div key={`${entry.id}-${index}`} className="flex items-center justify-between rounded-3xl bg-white/75 px-4 py-3">
                  <div className="flex items-center gap-3">
                    <div className="relative">
                      <img
                        alt={username}
                        className="h-12 w-12 rounded-[1.1rem] object-cover shadow-card"
                        src={entry.user?.avatarUrl || fallbackAvatar(username)}
                      />
                      <div className="absolute -bottom-1 -right-1 flex h-6 min-w-6 items-center justify-center rounded-full bg-ink px-1 text-[10px] font-black text-sand">
                        {index + 1}
                      </div>
                    </div>
                    <div>
                      <div className="font-black">{username}</div>
                      <div className="text-xs text-slatewarm">
                        {Math.round((entry.completionTimeMs || 0) / 1000)}s · {entry.mistakesCount} mistakes
                      </div>
                    </div>
                  </div>
                  <div className="rounded-full bg-ink px-3 py-2 text-sm font-bold text-sand">{entry.score}</div>
                </div>
              );
            })}
          </div>
        )}
      </Card>
    </div>
  );
}
