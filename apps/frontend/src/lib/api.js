const API_BASE = import.meta.env.VITE_API_URL || "http://localhost:8080";

let sessionResolver = () => null;
let sessionUpdater = () => {};
let sessionResetter = () => {};
let refreshPromise = null;

export function configureApiSession({ getSession, onSession, onLogout }) {
  sessionResolver = getSession || (() => null);
  sessionUpdater = onSession || (() => {});
  sessionResetter = onLogout || (() => {});
}

function buildUrl(path) {
  return `${API_BASE}${path}`;
}

function websocketBaseUrl() {
  if (API_BASE.startsWith("http://")) {
    return API_BASE.replace("http://", "ws://");
  }
  if (API_BASE.startsWith("https://")) {
    return API_BASE.replace("https://", "wss://");
  }
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}`;
}

async function parseResponse(response) {
  const text = await response.text();
  return text ? JSON.parse(text) : null;
}

async function refreshAccessToken() {
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = (async () => {
    const currentSession = sessionResolver();
    if (!currentSession?.refreshToken) {
      throw new Error("missing refresh token");
    }

    const response = await fetch(buildUrl("/auth/refresh"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken: currentSession.refreshToken })
    });
    const data = await parseResponse(response);
    if (!response.ok) {
      throw new Error(data?.error || data?.message || "refresh failed");
    }

    const nextSession = {
      accessToken: data.accessToken,
      refreshToken: data.refreshToken
    };
    sessionUpdater(nextSession);
    return nextSession;
  })().finally(() => {
    refreshPromise = null;
  });

  return refreshPromise;
}

async function request(path, options = {}, accessToken, retry = true) {
  const response = await fetch(buildUrl(path), {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      ...(options.headers || {})
    }
  });

  const data = await parseResponse(response);
  if (response.status === 401 && retry && accessToken) {
    const message = String(data?.error || data?.message || "").toLowerCase();
    if (message.includes("invalid token") || message.includes("missing bearer token")) {
      try {
        const nextSession = await refreshAccessToken();
        return request(path, options, nextSession.accessToken, false);
      } catch (error) {
        sessionResetter();
        throw error;
      }
    }
  }
  if (!response.ok) {
    throw new Error(data?.error || data?.message || "Request failed");
  }
  return data;
}

export const api = {
  baseUrl: API_BASE,
  wsBaseUrl: websocketBaseUrl,
  register(payload) {
    return request("/auth/register", {
      method: "POST",
      body: JSON.stringify(payload)
    });
  },
  login(payload) {
    return request("/auth/login", {
      method: "POST",
      body: JSON.stringify(payload)
    });
  },
  refresh(refreshToken) {
    return request("/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refreshToken })
    });
  },
  logout(refreshToken, accessToken) {
    return request(
      "/auth/logout",
      {
        method: "POST",
        body: JSON.stringify({ refreshToken })
      },
      accessToken
    );
  },
  me(accessToken) {
    return request("/users/me", {}, accessToken);
  },
  updateMe(payload, accessToken) {
    return request(
      "/users/me",
      {
        method: "PATCH",
        body: JSON.stringify(payload)
      },
      accessToken
    );
  },
  publicUser(id, accessToken) {
    return request(`/users/${id}`, {}, accessToken);
  },
  mySkins(accessToken) {
    return request("/users/me/skins", {}, accessToken);
  },
  joinMatchmaking(mode, accessToken) {
    return request(
      "/matchmaking/join",
      {
        method: "POST",
        body: JSON.stringify({ mode })
      },
      accessToken
    );
  },
  leaveMatchmaking(mode, accessToken) {
    return request(
      "/matchmaking/leave",
      {
        method: "POST",
        body: JSON.stringify({ mode })
      },
      accessToken
    );
  },
  matchHistory(accessToken) {
    return request("/matches/history", {}, accessToken);
  },
  matchById(id, accessToken) {
    return request(`/matches/${id}`, {}, accessToken);
  },
  replay(id, accessToken) {
    return request(`/matches/${id}/replay`, {}, accessToken);
  },
  analysis(id, accessToken) {
    return request(`/matches/${id}/analysis`, {}, accessToken);
  },
  daily(accessToken) {
    return request("/daily/", {}, accessToken);
  },
  submitDaily(payload, accessToken) {
    return request(
      "/daily/submit",
      {
        method: "POST",
        body: JSON.stringify(payload)
      },
      accessToken
    );
  },
  dailyLeaderboard(accessToken) {
    return request("/daily/leaderboard", {}, accessToken);
  },
  globalLeaderboard(accessToken) {
    return request("/leaderboards/global", {}, accessToken);
  },
  countries(accessToken) {
    return request("/leaderboards/countries", {}, accessToken);
  },
  countryLeaderboard(country, accessToken) {
    return request(`/leaderboards/countries/${country}`, {}, accessToken);
  },
  cities(accessToken) {
    return request("/leaderboards/cities", {}, accessToken);
  },
  cityLeaderboard(city, accessToken) {
    return request(`/leaderboards/cities/${encodeURIComponent(city)}`, {}, accessToken);
  },
  skins(accessToken) {
    return request("/skins/", {}, accessToken);
  },
  purchaseSkin(skinId, accessToken) {
    return request(
      "/skins/purchase",
      {
        method: "POST",
        body: JSON.stringify({ skinId })
      },
      accessToken
    );
  },
  createCheckoutSession(skinId, accessToken) {
    return request(
      "/payments/create-checkout-session",
      {
        method: "POST",
        body: JSON.stringify({ skinId })
      },
      accessToken
    );
  },
  checkoutSessionStatus(sessionId, accessToken) {
    return request(`/payments/checkout-session/${sessionId}`, {}, accessToken);
  },
  subscription(accessToken) {
    return request("/subscription/me", {}, accessToken);
  },
  cancelSubscription(accessToken) {
    return request(
      "/subscription/cancel",
      { method: "POST", body: JSON.stringify({}) },
      accessToken
    );
  }
};
