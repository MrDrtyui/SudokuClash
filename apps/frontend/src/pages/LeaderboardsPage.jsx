import { useEffect, useState } from "react";
import { Card } from "../components/Card";
import { Button } from "../components/Button";
import { api } from "../lib/api";

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

export function LeaderboardsPage({ accessToken }) {
  const [mode, setMode] = useState("global");
  const [players, setPlayers] = useState([]);
  const [filters, setFilters] = useState({ countries: [], cities: [] });
  const [selected, setSelected] = useState({ country: "", city: "" });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([api.countries(accessToken), api.cities(accessToken)])
      .then(([countries, cities]) =>
        setFilters({
          countries: Array.isArray(countries) ? countries : [],
          cities: Array.isArray(cities) ? cities : []
        })
      )
      .catch((loadError) => setError(loadError.message));
  }, [accessToken]);

  useEffect(() => {
    if (mode === "country" && !selected.country && filters.countries.length > 0) {
      setSelected((current) => ({ ...current, country: filters.countries[0] }));
    }
    if (mode === "city" && !selected.city && filters.cities.length > 0) {
      setSelected((current) => ({ ...current, city: filters.cities[0] }));
    }
  }, [mode, filters.countries, filters.cities, selected.country, selected.city]);

  useEffect(() => {
    async function loadPlayers() {
      setLoading(true);
      setError("");
      if (mode === "global") {
        setPlayers(await api.globalLeaderboard(accessToken));
        setLoading(false);
        return;
      }
      if (mode === "country" && selected.country) {
        setPlayers(await api.countryLeaderboard(selected.country, accessToken));
        setLoading(false);
        return;
      }
      if (mode === "city" && selected.city) {
        setPlayers(await api.cityLeaderboard(selected.city, accessToken));
        setLoading(false);
        return;
      }
      setPlayers([]);
      setLoading(false);
    }

    loadPlayers().catch((loadError) => setError(loadError.message));
  }, [accessToken, mode, selected.country, selected.city]);

  return (
    <div className="space-y-4">
      <Card title="Leaderboards" subtitle="Global first, local pride second.">
        <div className="flex flex-wrap gap-2">
          <Button variant={mode === "global" ? "primary" : "soft"} onClick={() => setMode("global")}>Global</Button>
          <Button variant={mode === "country" ? "accent" : "soft"} onClick={() => setMode("country")}>Country</Button>
          <Button variant={mode === "city" ? "accent" : "soft"} onClick={() => setMode("city")}>City</Button>
        </div>

        {mode === "country" ? (
          filters.countries.length > 0 ? (
            <select
              className="mt-4 w-full rounded-3xl border border-white bg-white/80 px-4 py-3"
              value={selected.country}
              onChange={(event) => setSelected((current) => ({ ...current, country: event.target.value }))}
            >
              {filters.countries.map((country) => (
                <option key={country} value={country}>
                  {country}
                </option>
              ))}
            </select>
          ) : (
            <div className="mt-4 rounded-3xl bg-white/80 px-4 py-4 text-sm text-slatewarm">
              No countries yet. Add `country` in the Me tab first.
            </div>
          )
        ) : null}

        {mode === "city" ? (
          filters.cities.length > 0 ? (
            <select
              className="mt-4 w-full rounded-3xl border border-white bg-white/80 px-4 py-3"
              value={selected.city}
              onChange={(event) => setSelected((current) => ({ ...current, city: event.target.value }))}
            >
              {filters.cities.map((city) => (
                <option key={city} value={city}>
                  {city}
                </option>
              ))}
            </select>
          ) : (
            <div className="mt-4 rounded-3xl bg-white/80 px-4 py-4 text-sm text-slatewarm">
              No cities yet. Add `city` in the Me tab first.
            </div>
          )
        ) : null}

        {error ? <p className="mt-3 text-sm text-coral">{error}</p> : null}
      </Card>

      <Card title="Top Players" subtitle="Sorted by ELO and wins.">
        {loading ? (
          <div className="rounded-3xl bg-white/75 px-4 py-4 text-sm text-slatewarm">Loading leaderboard...</div>
        ) : players.length === 0 ? (
          <div className="rounded-3xl bg-white/75 px-4 py-4 text-sm text-slatewarm">
            No players found for this filter yet.
          </div>
        ) : (
          <div className="space-y-3">
            {players.map((player, index) => (
              <div key={player.id} className="flex items-center justify-between rounded-3xl bg-white/75 px-4 py-3">
                <div className="flex items-center gap-3">
                  <div className="relative">
                    <img
                      alt={player.username}
                      className="h-12 w-12 rounded-[1.1rem] object-cover shadow-card"
                      src={player.avatarUrl || fallbackAvatar(player.username)}
                    />
                    <div className="absolute -bottom-1 -right-1 flex h-6 min-w-6 items-center justify-center rounded-full bg-ink px-1 text-[10px] font-black text-sand">
                      {index + 1}
                    </div>
                  </div>
                  <div>
                    <div className="font-black">{player.username}</div>
                    <div className="text-xs text-slatewarm">{player.countryCode || "Global"} · {player.city || "Arena"}</div>
                  </div>
                </div>
                <div className="rounded-full bg-coral px-3 py-2 text-sm font-bold text-white">{player.eloRating}</div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
