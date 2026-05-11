import { NavLink } from "react-router-dom";

const links = [
  { to: "/", label: "Home" },
  { to: "/play", label: "Play" },
  { to: "/daily", label: "Daily" },
  { to: "/leaderboards", label: "Ranks" },
  { to: "/me", label: "Me" }
];

export function AppShell({ user, skinTheme, children }) {
  return (
    <div className="mx-auto flex min-h-screen w-full max-w-md flex-col px-4 pb-28 pt-4 text-ink">
      <header className="glass sticky top-4 z-10 rounded-4xl border border-white/70 px-5 py-4 shadow-card">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-[0.3em] text-slatewarm">Sudoku Clash</p>
            <h1 className="mt-2 text-2xl font-black leading-none">Mobile Arena</h1>
          </div>
          <div className={`rounded-3xl px-3 py-2 text-right text-sm ${skinTheme?.shellAccent || "bg-coral text-white"}`}>
            <div className="font-bold">{user?.username || "Guest"}</div>
            <div className="text-xs opacity-85">ELO {user?.eloRating ?? 1000}</div>
          </div>
        </div>
      </header>

      <main className="flex-1 pt-4">{children}</main>

      <nav className="glass fixed bottom-4 left-1/2 z-20 flex w-[calc(100%-2rem)] max-w-md -translate-x-1/2 items-center justify-between rounded-full border border-white/70 px-3 py-3 shadow-card">
        {links.map((link) => (
          <NavLink
            key={link.to}
            to={link.to}
            className={({ isActive }) =>
              `rounded-full px-3 py-2 text-sm font-semibold transition ${
                isActive ? skinTheme?.navActive || "bg-ink text-sand" : "text-slatewarm"
              }`
            }
          >
            {link.label}
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
