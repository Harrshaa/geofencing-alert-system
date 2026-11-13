import React, { useCallback, useEffect, useState } from 'react';

import { api, setAuthToken, setUnauthorizedHandler } from './api';
import { clearSession, loadSession, saveSession } from './auth';
import Dashboard from './Dashboard';
import LoginPage from './components/LoginPage';
import { Spinner } from './components/ui';

// App decides what to show:
//   - asks the backend whether login is required (GET /auth/status)
//   - login required and no valid session  -> LoginPage
//   - otherwise                            -> Dashboard
//
// If any request later gets a 401 (token expired or revoked), api.js calls
// handleUnauthorized and the login page comes back.
export default function App() {
  // Read the saved session once, and hand its token to api.js straight away,
  // before the Dashboard's first requests go out.
  const [session, setSession] = useState(() => {
    const saved = loadSession();
    setAuthToken(saved && saved.token);
    return saved;
  });
  const [authRequired, setAuthRequired] = useState(null); // null = still asking
  const [notice, setNotice] = useState('');

  useEffect(() => {
    let cancelled = false;
    api
      .authStatus()
      .then((data) => {
        if (!cancelled) setAuthRequired(Boolean(data.auth_required));
      })
      .catch(() => {
        // Backend unreachable: show the dashboard, which explains the problem
        // and has a "Try again" button.
        if (!cancelled) setAuthRequired(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const endSession = useCallback((message) => {
    clearSession();
    setAuthToken(null);
    setSession(null);
    setNotice(message);
  }, []);

  const handleUnauthorized = useCallback(() => {
    setAuthRequired(true); // a 401 proves the backend requires login
    endSession('Your session has expired. Please sign in again.');
  }, [endSession]);

  useEffect(() => {
    setUnauthorizedHandler(handleUnauthorized);
    return () => setUnauthorizedHandler(null);
  }, [handleUnauthorized]);

  const handleLogin = useCallback((result) => {
    const next = { token: result.token, email: result.user.email, expiresAt: result.expires_at };
    saveSession(next);
    setAuthToken(next.token);
    setNotice('');
    setSession(next);
  }, []);

  const handleLogout = useCallback(() => endSession(''), [endSession]);

  if (authRequired === null) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner label="Connecting…" />
      </div>
    );
  }

  if (authRequired && !session) {
    return <LoginPage onLoggedIn={handleLogin} notice={notice} />;
  }

  // key: a new login mounts a fresh Dashboard, so nothing from the previous
  // session (data, socket) carries over.
  return (
    <Dashboard
      key={session ? session.token : 'open'}
      session={authRequired ? session : null}
      onLogout={handleLogout}
    />
  );
}
