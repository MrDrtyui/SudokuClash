import { useState } from "react";
import { Button } from "../components/Button";
import { Card } from "../components/Card";
import { Input } from "../components/Input";

export function AuthPage({ onLogin, error }) {
  const [mode, setMode] = useState("login");
  const [form, setForm] = useState({
    username: "",
    email: "",
    password: ""
  });
  const [loading, setLoading] = useState(false);
  const [localError, setLocalError] = useState("");

  async function submit(event) {
    event.preventDefault();
    setLoading(true);
    setLocalError("");
    try {
      await onLogin(mode, form);
    } catch (submitError) {
      setLocalError(submitError.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-4 py-8">
      <div className="mb-8">
        <p className="text-xs uppercase tracking-[0.35em] text-slatewarm">Sudoku Clash</p>
        <h1 className="mt-3 text-5xl font-black leading-none text-ink">Race the grid.</h1>
        <p className="mt-4 max-w-sm text-base text-slatewarm">
          Mobile-first ranked Sudoku with daily runs, live duels and player progression.
        </p>
      </div>

      <Card className="overflow-hidden p-0">
        <div className="grid grid-cols-2 bg-white/50 p-2">
          <button
            className={`rounded-full px-4 py-3 text-sm font-bold ${mode === "login" ? "bg-ink text-sand" : "text-slatewarm"}`}
            onClick={() => setMode("login")}
          >
            Login
          </button>
          <button
            className={`rounded-full px-4 py-3 text-sm font-bold ${mode === "register" ? "bg-coral text-white" : "text-slatewarm"}`}
            onClick={() => setMode("register")}
          >
            Register
          </button>
        </div>

        <form className="space-y-4 p-5" onSubmit={submit}>
          {mode === "register" ? (
            <Input
              label="Username"
              placeholder="sudokuqueen"
              value={form.username}
              onChange={(event) => setForm((current) => ({ ...current, username: event.target.value }))}
            />
          ) : null}
          <Input
            label="Email"
            type="email"
            placeholder="you@example.com"
            value={form.email}
            onChange={(event) => setForm((current) => ({ ...current, email: event.target.value }))}
          />
          <Input
            label="Password"
            type="password"
            placeholder="••••••••"
            value={form.password}
            onChange={(event) => setForm((current) => ({ ...current, password: event.target.value }))}
          />
          {error || localError ? (
            <div className="rounded-3xl bg-coral/10 px-4 py-3 text-sm font-semibold text-coral">
              {localError || error}
            </div>
          ) : null}
          <Button className="w-full" variant={mode === "register" ? "accent" : "primary"} loading={loading}>
            {mode === "register" ? "Create Account" : "Enter Arena"}
          </Button>
        </form>
      </Card>
    </div>
  );
}

