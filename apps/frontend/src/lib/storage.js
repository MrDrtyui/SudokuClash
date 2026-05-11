const KEY = "sudoku-mobile-auth";
const SKIN_KEY = "sudoku-mobile-skin";

export function loadSession() {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function saveSession(session) {
  localStorage.setItem(KEY, JSON.stringify(session));
}

export function clearSession() {
  localStorage.removeItem(KEY);
}

export function loadEquippedSkin() {
  try {
    return localStorage.getItem(SKIN_KEY) || "classic";
  } catch {
    return "classic";
  }
}

export function saveEquippedSkin(skinId) {
  localStorage.setItem(SKIN_KEY, skinId);
}
