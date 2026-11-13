import React, { useState } from 'react';
import { api } from '../api';
import { ErrorNote, Field, SubmitButton } from './ui';

// Shown only when the backend requires login (AUTH_REQUIRED=true).
// Sends email + password to POST /auth/login and passes the result
// (token, expiry, user) up to App.
export default function LoginPage({ onLoggedIn, notice }) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function handleSubmit(event) {
    event.preventDefault();
    setError('');
    if (!email.trim() || !password) {
      setError('Enter your email and password.');
      return;
    }

    setBusy(true);
    try {
      const result = await api.login(email.trim(), password);
      onLoggedIn(result);
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-full items-center justify-center px-4 py-10">
      <form onSubmit={handleSubmit} className="card w-full max-w-sm space-y-4 p-6 shadow-sm" noValidate>
        <div>
          <h1 className="text-lg font-semibold">Geofence Alerts</h1>
          <p className="mt-1 text-sm text-slate-500">Sign in to continue.</p>
        </div>

        {notice && (
          <p className="rounded-md border border-amber-200 bg-amber-50 px-2.5 py-2 text-sm text-amber-800">{notice}</p>
        )}

        <Field label="Email">
          <input
            className="input"
            type="email"
            autoComplete="username"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder="admin@example.com"
            autoFocus
          />
        </Field>
        <Field label="Password">
          <input
            className="input"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </Field>

        <ErrorNote message={error} />
        <SubmitButton busy={busy} busyLabel="Signing in…" className="btn-primary w-full">
          Sign in
        </SubmitButton>
      </form>
    </div>
  );
}
