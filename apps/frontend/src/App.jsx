import { useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { AuthPage } from "./pages/AuthPage";
import { DailyPage } from "./pages/DailyPage";
import { HomePage } from "./pages/HomePage";
import { LeaderboardsPage } from "./pages/LeaderboardsPage";
import { PlayPage } from "./pages/PlayPage";
import { ProfilePage } from "./pages/ProfilePage";
import { api, configureApiSession } from "./lib/api";
import { clearSession, loadEquippedSkin, loadSession, saveEquippedSkin, saveSession } from "./lib/storage";
import { skinThemeFor } from "./lib/skins";

export default function App() {
  const [session, setSession] = useState(() => loadSession());
  const [user, setUser] = useState(null);
  const [subscription, setSubscription] = useState(null);
  const [recentMatches, setRecentMatches] = useState([]);
  const [equippedSkinId, setEquippedSkinId] = useState(() => loadEquippedSkin());
  const [authError, setAuthError] = useState("");
  const [booting, setBooting] = useState(Boolean(loadSession()));
  const skinTheme = skinThemeFor({ id: equippedSkinId });

  useEffect(() => {
    configureApiSession({
      getSession: () => session,
      onSession: (nextSession) => {
        saveSession(nextSession);
        setSession(nextSession);
      },
      onLogout: () => {
        clearSession();
        setSession(null);
        setUser(null);
        setSubscription(null);
        setRecentMatches([]);
      }
    });
  }, [session]);

  async function refreshAppData(accessToken = session?.accessToken) {
    if (!accessToken) {
      return;
    }
    const [me, subscriptionData, history] = await Promise.all([
      api.me(accessToken),
      api.subscription(accessToken),
      api.matchHistory(accessToken)
    ]);
    setUser(me);
    if (me?.activeSkin) {
      setEquippedSkinId(me.activeSkin);
      saveEquippedSkin(me.activeSkin);
    }
    setSubscription(subscriptionData);
    setRecentMatches(Array.isArray(history) ? history.slice(0, 3) : []);
  }

  useEffect(() => {
    async function bootstrap() {
      if (!session?.accessToken) {
        setBooting(false);
        return;
      }

      try {
        await refreshAppData(session.accessToken);
      } catch (error) {
        if (session?.refreshToken) {
          try {
            const refreshed = await api.refresh(session.refreshToken);
            const nextSession = {
              accessToken: refreshed.accessToken,
              refreshToken: refreshed.refreshToken
            };
            saveSession(nextSession);
            setSession(nextSession);
            await refreshAppData(nextSession.accessToken);
            return;
          } catch {
            clearSession();
            setSession(null);
            setUser(null);
          }
        }
      } finally {
        setBooting(false);
      }
    }

    bootstrap();
  }, [session?.accessToken, session?.refreshToken]);

  async function handleAuth(mode, form) {
    setAuthError("");
    const payload =
      mode === "register"
        ? await api.register(form)
        : await api.login({ email: form.email, password: form.password });

    const nextSession = {
      accessToken: payload.tokens.accessToken,
      refreshToken: payload.tokens.refreshToken
    };
    saveSession(nextSession);
    setSession(nextSession);
    setUser(payload.user);
    setSubscription({ status: payload.user.subscriptionType });
  }

  async function handleLogout() {
    try {
      if (session?.refreshToken && session?.accessToken) {
        await api.logout(session.refreshToken, session.accessToken);
      }
    } catch {
      // Intentionally ignore logout cleanup errors during client reset.
    } finally {
      clearSession();
      setSession(null);
      setUser(null);
      setSubscription(null);
      setRecentMatches([]);
    }
  }

  if (booting) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="rounded-full bg-ink px-6 py-3 text-sm font-bold text-sand">Loading mobile arena...</div>
      </div>
    );
  }

  if (!session || !user) {
    return (
      <AuthPage
        error={authError}
        onLogin={async (mode, form) => {
          try {
            await handleAuth(mode, form);
          } catch (error) {
            setAuthError(error.message);
            throw error;
          }
        }}
      />
    );
  }

  return (
    <AppShell skinTheme={skinTheme} user={user}>
      <Routes>
        <Route path="/" element={<HomePage user={user} subscription={subscription} recentMatches={recentMatches} />} />
        <Route
          path="/play"
          element={<PlayPage accessToken={session.accessToken} userId={user.id} onMatchComplete={refreshAppData} skinTheme={skinTheme} />}
        />
        <Route path="/daily" element={<DailyPage accessToken={session.accessToken} />} />
        <Route path="/leaderboards" element={<LeaderboardsPage accessToken={session.accessToken} />} />
        <Route
          path="/me"
          element={
            <ProfilePage
              accessToken={session.accessToken}
              user={user}
              equippedSkinId={equippedSkinId}
              onEquipSkin={(skinId) => {
                setEquippedSkinId(skinId);
                saveEquippedSkin(skinId);
              }}
              onProfileUpdate={(nextUser) => {
                setUser(nextUser);
                refreshAppData(session.accessToken).catch(() => {});
              }}
              onLogout={handleLogout}
            />
          }
        />
        <Route path="/profile" element={<Navigate to="/me" replace />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppShell>
  );
}
