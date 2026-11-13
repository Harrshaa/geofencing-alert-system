// Keeps the login session (token + email) in the browser's localStorage, so
// a page reload does not log the user out.
//
// Trade-off worth knowing: any JavaScript running on this page can read
// localStorage, so a cross-site-scripting bug could steal the token. The
// safer alternative is an httpOnly cookie that JavaScript cannot read; that
// needs cookie handling and CSRF protection on the backend.

const STORAGE_KEY = 'geofence-alerts.session';

// Returns { token, email, expiresAt } or null. Expired sessions are dropped.
export function loadSession() {
  try {
    const session = JSON.parse(localStorage.getItem(STORAGE_KEY));
    if (!session || !session.token) return null;
    if (session.expiresAt && new Date(session.expiresAt).getTime() <= Date.now()) {
      clearSession();
      return null;
    }
    return session;
  } catch {
    return null; // storage blocked or corrupted
  }
}

export function saveSession(session) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
  } catch {
    // Storage blocked (private window): the session lasts until reload.
  }
}

export function clearSession() {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    // Nothing to clear.
  }
}
