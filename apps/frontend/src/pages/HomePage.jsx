import { Card } from "../components/Card";

export function HomePage({ user, subscription, recentMatches = [] }) {
  return (
    <div className="space-y-4">
      <Card
        title={`Welcome back, ${user.username}`}
        subtitle="Fast mobile sessions, ranked climbs and daily puzzles."
        right={<div className="rounded-full bg-olive px-3 py-2 text-xs font-bold text-white">{subscription?.status || "free"}</div>}
      >
        <div className="grid grid-cols-3 gap-3">
          <Stat label="Wins" value={user.wins} />
          <Stat label="Peak ELO" value={user.peakElo} />
          <Stat label="Level" value={user.level} />
        </div>
      </Card>

      <Card title="Momentum" subtitle="Your current progression snapshot.">
        <div className="space-y-3">
          <ProgressRow label="Streak" value={`${user.currentStreak}/${user.maxStreak}`} />
          <ProgressRow label="Experience" value={user.experience} />
          <ProgressRow label="Record" value={`${user.wins}-${user.losses}-${user.draws}`} />
        </div>
      </Card>

      <Card title="Recent Matches" subtitle="Your last ranked sessions.">
        {recentMatches.length > 0 ? (
          <div className="space-y-3">
            {recentMatches.map((match) => (
              <div key={match.id} className="rounded-3xl bg-white/70 px-4 py-4">
                <div className="flex items-center justify-between gap-3">
                  <div className="text-base font-black text-ink">{matchTitle(match)}</div>
                  <div className={`rounded-full px-3 py-1 text-[11px] font-bold ${matchBadge(match.status)}`}>{matchStatus(match.status)}</div>
                </div>
                <div className="mt-2 flex items-center justify-between text-sm text-slatewarm">
                  <span>{formatMatchDate(match.startedAt)}</span>
                  <span>{formatMatchDuration(match.matchDurationMs)}</span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-slatewarm">No matches yet. Jump into ranked play and your recent sessions will appear here.</p>
        )}
      </Card>
    </div>
  );
}

function Stat({ label, value }) {
  return (
    <div className="rounded-3xl bg-white/75 px-3 py-4 text-center">
      <div className="text-xl font-black">{value}</div>
      <div className="mt-1 text-xs uppercase tracking-[0.2em] text-slatewarm">{label}</div>
    </div>
  );
}

function ProgressRow({ label, value }) {
  return (
    <div className="flex items-center justify-between rounded-3xl bg-white/75 px-4 py-3">
      <span className="font-semibold text-slatewarm">{label}</span>
      <span className="font-black">{value}</span>
    </div>
  );
}

function matchTitle(match) {
  if (match.status === "finished") {
    return "Ranked finished";
  }
  if (match.status === "active") {
    return "Ranked in progress";
  }
  return "Ranked match";
}

function matchStatus(status) {
  switch (status) {
    case "finished":
      return "Finished";
    case "active":
      return "Live";
    default:
      return "Match";
  }
}

function matchBadge(status) {
  switch (status) {
    case "finished":
      return "bg-white text-ink";
    case "active":
      return "bg-coral text-white";
    default:
      return "bg-sand text-ink";
  }
}

function formatMatchDate(value) {
  if (!value) {
    return "Just now";
  }
  return new Date(value).toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit"
  });
}

function formatMatchDuration(value) {
  if (!value) {
    return "2 min arena";
  }
  const seconds = Math.max(0, Math.round(value / 1000));
  const minutes = Math.floor(seconds / 60);
  const remainder = seconds % 60;
  if (minutes === 0) {
    return `${remainder}s`;
  }
  return `${minutes}m ${remainder.toString().padStart(2, "0")}s`;
}
