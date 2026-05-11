import { useEffect, useRef, useState } from "react";
import { Button } from "../components/Button";
import { Card } from "../components/Card";
import { SudokuBoard } from "../components/SudokuBoard";
import { api } from "../lib/api";
import { skinThemeFor } from "../lib/skins";

function cloneBoard(board) {
  return board.map((row) => [...row]);
}

export function PlayPage({ accessToken, userId, onMatchComplete, skinTheme }) {
  const [status, setStatus] = useState("idle");
  const [message, setMessage] = useState("");
  const [match, setMatch] = useState(null);
  const [puzzle, setPuzzle] = useState(null);
  const [board, setBoard] = useState(null);
  const [progress, setProgress] = useState({ me: 0, opponent: 0 });
  const [moveResult, setMoveResult] = useState(null);
  const [remainingSeconds, setRemainingSeconds] = useState(null);
  const [result, setResult] = useState(null);
  const [analysis, setAnalysis] = useState(null);
  const [analysisLoading, setAnalysisLoading] = useState(false);
  const [opponentTheme, setOpponentTheme] = useState(skinThemeFor({ id: "classic" }));
  const socketRef = useRef(null);
  const pollRef = useRef(null);
  const timerRef = useRef(null);
  const analysisRequestedRef = useRef("");

  useEffect(
    () => () => {
      socketRef.current?.close();
      if (pollRef.current) {
        window.clearInterval(pollRef.current);
      }
      if (timerRef.current) {
        window.clearInterval(timerRef.current);
      }
    },
    []
  );

  async function enterMatch(result) {
    setMatch({ id: result.matchId, opponentId: result.opponentId });
    setPuzzle(result.puzzle);
    setBoard(cloneBoard(result.puzzle.initialBoard));
    setStatus("matched");
    setMessage("Match found. Enter digits and submit live moves.");
    setResult(null);
    setAnalysis(null);
    analysisRequestedRef.current = "";
    startTimer(result.startedAt, result.durationSeconds ?? 120);
    connectSocket(result.matchId);
    try {
      const opponent = await api.publicUser(result.opponentId, accessToken);
      setOpponentTheme(skinThemeFor({ id: opponent.activeSkin || "classic" }));
    } catch {
      setOpponentTheme(skinThemeFor({ id: "classic" }));
    }
  }

  function startTimer(startedAt, durationSeconds) {
    if (timerRef.current) {
      window.clearInterval(timerRef.current);
    }

    const startMs = new Date(startedAt).getTime();
    const endMs = startMs + durationSeconds * 1000;
    const tick = () => {
      const next = Math.max(0, Math.ceil((endMs - Date.now()) / 1000));
      setRemainingSeconds(next);
      if (next <= 0 && timerRef.current) {
        window.clearInterval(timerRef.current);
        timerRef.current = null;
      }
    };

    tick();
    timerRef.current = window.setInterval(tick, 1000);
  }

  function startPolling() {
    if (pollRef.current) {
      window.clearInterval(pollRef.current);
    }
    pollRef.current = window.setInterval(async () => {
      try {
        const result = await api.joinMatchmaking("ranked", accessToken);
        if (result.status === "matched") {
          window.clearInterval(pollRef.current);
          pollRef.current = null;
          void enterMatch(result);
        }
      } catch (error) {
        setMessage(error.message);
      }
    }, 2000);
  }

  async function joinQueue() {
    setStatus("queueing");
    setMessage("");
    try {
      const result = await api.joinMatchmaking("ranked", accessToken);
      if (result.status === "queued") {
        setStatus("queued");
        setMessage("Queued. Waiting for an opponent...");
        startPolling();
        return;
      }
      void enterMatch(result);
    } catch (error) {
      setStatus("idle");
      setMessage(error.message);
    }
  }

  async function leaveQueue() {
    if (pollRef.current) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
    if (status === "matched" && socketRef.current?.readyState === WebSocket.OPEN) {
      socketRef.current.send(
        JSON.stringify({
          event: "match:surrender",
          data: {}
        })
      );
    }
    await api.leaveMatchmaking("ranked", accessToken);
    socketRef.current?.close();
    socketRef.current = null;
    if (timerRef.current) {
      window.clearInterval(timerRef.current);
      timerRef.current = null;
    }
    setStatus("idle");
    setMatch(null);
    setPuzzle(null);
    setBoard(null);
    setProgress({ me: 0, opponent: 0 });
    setMoveResult(null);
    setRemainingSeconds(null);
    setResult(null);
    setAnalysis(null);
    analysisRequestedRef.current = "";
    setOpponentTheme(skinThemeFor({ id: "classic" }));
    setMessage("Left matchmaking.");
  }

  function connectSocket(matchId) {
    socketRef.current?.close();
    const wsBase = api.wsBaseUrl();
    const socket = new WebSocket(`${wsBase}/ws?token=${accessToken}&matchId=${matchId}`);
    socketRef.current = socket;

    socket.onmessage = (event) => {
      const payload = JSON.parse(event.data);
      if (payload.event === "move:result") {
        setMoveResult(payload.data);
      }
      if (payload.event === "match:update") {
        setProgress({
          me: payload.data.player2Progress ?? payload.data.player1Progress ?? 0,
          opponent: payload.data.player1Progress ?? payload.data.player2Progress ?? 0
        });
      }
      if (payload.event === "match:finished") {
        if (timerRef.current) {
          window.clearInterval(timerRef.current);
          timerRef.current = null;
        }
        setStatus("finished");
        setRemainingSeconds(0);
        setMessage(`Match finished. Winner: ${payload.data.winnerId || "draw"}`);
        setResult(payload.data);
        onMatchComplete?.();
      }
      if (payload.event === "match:error") {
        const errorMessage = String(payload?.data?.message || "");
        if (errorMessage.toLowerCase().includes("live match not found") || status === "finished") {
          return;
        }
        if (timerRef.current) {
          window.clearInterval(timerRef.current);
          timerRef.current = null;
        }
        setStatus("idle");
        setMatch(null);
        setPuzzle(null);
        setBoard(null);
        setProgress({ me: 0, opponent: 0 });
        setMoveResult(null);
        setRemainingSeconds(null);
        setResult(null);
        setAnalysis(null);
        analysisRequestedRef.current = "";
        setOpponentTheme(skinThemeFor({ id: "classic" }));
        setMessage(errorMessage);
      }
    };
  }

  async function analyzeDemo() {
    if (!match?.id) {
      return;
    }
    setAnalysisLoading(true);
    try {
      const response = await api.analysis(match.id, accessToken);
      setAnalysis(response.analysis || response);
    } catch (error) {
      setMessage(error.message);
    } finally {
      setAnalysisLoading(false);
    }
  }

  useEffect(() => {
    if (status !== "finished" || !match?.id || analysisLoading || analysis || analysisRequestedRef.current === match.id) {
      return;
    }
    analysisRequestedRef.current = match.id;
    void analyzeDemo();
  }, [status, match?.id, analysisLoading, analysis]);

  const myAnalysis = analysis?.players?.[userId] || null;

  function onCellChange(row, col, value) {
    const numeric = Number(value);
    setBoard((current) => {
      const next = cloneBoard(current);
      next[row][col] = Number.isNaN(numeric) ? 0 : numeric;
      return next;
    });
    socketRef.current?.send(
      JSON.stringify({
        event: "move:submit",
        data: {
          row,
          col,
          value: numeric
        }
      })
    );
  }

  return (
    <div className="space-y-4">
      <Card title="Ranked Matchmaking" subtitle="Built for one-handed mobile sessions.">
        <div className="flex gap-3">
          <Button className="flex-1" onClick={joinQueue} disabled={status === "queueing" || status === "queued"}>
            {status === "queueing" ? "Finding..." : "Join Ranked"}
          </Button>
          <Button className="flex-1" variant="ghost" onClick={leaveQueue}>
            Reset
          </Button>
        </div>
        {message ? <p className="mt-3 text-sm text-slatewarm">{message}</p> : null}
      </Card>

      {puzzle && board ? (
        <Card title="Live Board" subtitle={`Match ${match?.id || ""}`}>
          <div className="mb-4 flex items-center justify-between rounded-3xl bg-white/80 px-4 py-3">
            <div>
              <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Round timer</div>
              <div className="mt-1 text-xl font-black">{formatTimer(remainingSeconds)}</div>
            </div>
            <div className="rounded-full bg-coral px-3 py-2 text-sm font-bold text-white">2 min</div>
          </div>
          <SudokuBoard board={board} initialBoard={puzzle.initialBoard} onChange={onCellChange} skinTheme={skinTheme} />
          <div className="mt-4 grid grid-cols-2 gap-3">
            <div className={`rounded-3xl px-4 py-3 ${skinTheme?.boardEditable || "bg-white text-coral"}`}>
              <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Your progress</div>
              <div className="mt-1 text-xl font-black">{progress.me}</div>
            </div>
            <div className={`rounded-3xl px-4 py-3 ${opponentTheme?.boardEditable || "bg-white text-coral"}`}>
              <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Opponent</div>
              <div className="mt-1 text-xl font-black">{progress.opponent}</div>
            </div>
          </div>
          {moveResult ? (
            <div className={`mt-4 rounded-3xl px-4 py-3 text-sm font-semibold ${moveResult.correct ? "bg-olive/10 text-olive" : "bg-coral/10 text-coral"}`}>
              {moveResult.correct ? "Correct move" : moveResult.message || "Incorrect move"} · Progress {moveResult.newProgress ?? progress.me}
            </div>
          ) : null}
        </Card>
      ) : null}

      {result ? (
        <Card title="Results" subtitle="Post-game recap.">
          <div className="space-y-3">
            <div className="rounded-3xl bg-white/80 px-4 py-4">
              <div className="text-lg font-black">{resultLabel(result)}</div>
              <div className="mt-2 text-sm text-slatewarm">
                Duration {Math.round((result.matchDurationMs || 0) / 1000)}s · Final progress {result.player1Progress}-{result.player2Progress}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="rounded-3xl bg-white/75 px-4 py-3">
                <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Player 1 ELO</div>
                <div className="mt-1 text-xl font-black">{formatDelta(result.player1EloAfter - result.player1EloBefore)}</div>
              </div>
              <div className="rounded-3xl bg-white/75 px-4 py-3">
                <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Player 2 ELO</div>
                <div className="mt-1 text-xl font-black">{formatDelta(result.player2EloAfter - result.player2EloBefore)}</div>
              </div>
            </div>

            <Button className="w-full" variant="accent" loading={analysisLoading} onClick={analyzeDemo}>
              {analysis ? "Re-run Analysis" : "Analyze Demo"}
            </Button>

            {myAnalysis ? (
              <div className="rounded-3xl bg-white/80 px-4 py-4 text-sm">
                <div className="font-black">Neural Review</div>
                <div className="mt-2 text-slatewarm">
                  Accuracy: {myAnalysis.summary?.accuracy ?? "-"}% · Avg move time: {myAnalysis.summary?.avg_move_time_ms ?? "-"} ms
                </div>
                <div className="mt-3 grid grid-cols-2 gap-3">
                  <div className="rounded-2xl bg-sand px-3 py-3">
                    <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Mistakes</div>
                    <div className="mt-1 text-xl font-black">{myAnalysis.summary?.incorrect_moves ?? 0}</div>
                  </div>
                  <div className="rounded-2xl bg-sand px-3 py-3">
                    <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Best Streak</div>
                    <div className="mt-1 text-xl font-black">{myAnalysis.summary?.longest_correct_streak ?? 0}</div>
                  </div>
                </div>

                {Array.isArray(myAnalysis.strengths) && myAnalysis.strengths.length > 0 ? (
                  <div className="mt-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Strengths</div>
                    <div className="mt-2 space-y-2">
                      {myAnalysis.strengths.map((insight, index) => (
                        <div key={`${insight}-${index}`} className="rounded-2xl bg-olive/10 px-3 py-2 text-olive">
                          {insight}
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}

                {Array.isArray(myAnalysis.improvements) && myAnalysis.improvements.length > 0 ? (
                  <div className="mt-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">How To Improve</div>
                    <div className="mt-2 space-y-2">
                      {myAnalysis.improvements.map((insight, index) => (
                        <div key={`${insight}-${index}`} className="rounded-2xl bg-coral/10 px-3 py-2 text-ink">
                          {insight}
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}

                {Array.isArray(myAnalysis.mistakes) && myAnalysis.mistakes.length > 0 ? (
                  <div className="mt-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Mistake Breakdown</div>
                    <div className="mt-2 space-y-2">
                      {myAnalysis.mistakes.map((mistake, index) => (
                        <div key={`${mistake.time}-${index}`} className="rounded-2xl bg-sand px-3 py-3">
                          <div className="font-semibold">
                            {formatReplayTime(mistake.time)} · Row {mistake.row + 1}, Col {mistake.col + 1} · {mistake.phase}
                          </div>
                          <div className="mt-1 text-slatewarm">{mistake.message}</div>
                          <div className="mt-1 text-xs text-slatewarm">
                            Think time {mistake.think_time_ms} ms{mistake.pressure ? " · under pressure" : ""}
                          </div>
                          <div className="mt-2 rounded-xl bg-white/70 px-3 py-2 text-ink">{mistake.recommendation}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}

                {Array.isArray(myAnalysis.insights) && myAnalysis.insights.length > 0 ? (
                  <div className="mt-4">
                    <div className="text-xs uppercase tracking-[0.2em] text-slatewarm">Coach Notes</div>
                    <div className="mt-2 space-y-2">
                      {myAnalysis.insights.map((insight, index) => (
                        <div key={`${insight}-${index}`} className="rounded-2xl bg-white/70 px-3 py-2">
                          {insight}
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        </Card>
      ) : null}
    </div>
  );
}

function formatTimer(value) {
  if (value == null) {
    return "02:00";
  }
  const minutes = Math.floor(value / 60)
    .toString()
    .padStart(2, "0");
  const seconds = (value % 60).toString().padStart(2, "0");
  return `${minutes}:${seconds}`;
}

function formatDelta(value) {
  if (value > 0) {
    return `+${value}`;
  }
  return `${value}`;
}

function formatReplayTime(value) {
  const seconds = Math.max(0, Math.floor((value || 0) / 1000));
  const minutes = Math.floor(seconds / 60)
    .toString()
    .padStart(2, "0");
  const remainder = (seconds % 60).toString().padStart(2, "0");
  return `${minutes}:${remainder}`;
}

function resultLabel(result) {
  if (!result.winnerId) {
    return "Draw";
  }
  return "Match finished";
}
