// Small presentational pieces used across the panels.
import React from 'react';
import { categoryInfo } from '../config';

export function Spinner({ label }) {
  return (
    <span className="inline-flex items-center gap-2 text-sm text-slate-500" role="status">
      <span className="spinner" aria-hidden="true" />
      {label}
    </span>
  );
}

// A red box for an error message. Renders nothing when there is no message.
export function ErrorNote({ message }) {
  if (!message) return null;
  return (
    <p className="rounded-md border border-red-200 bg-red-50 px-2.5 py-2 text-sm text-red-700" role="alert">
      {message}
    </p>
  );
}

export function EmptyState({ children }) {
  return (
    <p className="rounded-md border border-dashed border-slate-300 px-3 py-6 text-center text-sm text-slate-500">
      {children}
    </p>
  );
}

// A form field: label on top, the input passed as children.
export function Field({ label, hint, children }) {
  return (
    <label className="block">
      <span className="label">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-xs text-slate-500">{hint}</span>}
    </label>
  );
}

// Submit button that shows a spinner and disables itself while busy.
export function SubmitButton({ busy, children, busyLabel = 'Saving…', ...rest }) {
  return (
    <button type="submit" className="btn-primary" disabled={busy} {...rest}>
      {busy && <span className="spinner" aria-hidden="true" />}
      {busy ? busyLabel : children}
    </button>
  );
}

// Coloured pill for a geofence category.
export function CategoryChip({ category }) {
  const info = categoryInfo(category);
  return (
    <span className="chip" style={{ backgroundColor: `${info.color}1a`, color: info.color }}>
      <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: info.color }} />
      {info.label}
    </span>
  );
}

// Pill for an entry / exit / both event type.
export function EventChip({ type }) {
  const styles = {
    entry: 'bg-red-100 text-red-700',
    exit: 'bg-emerald-100 text-emerald-700',
    both: 'bg-slate-200 text-slate-700',
  };
  const labels = { entry: 'Entry', exit: 'Exit', both: 'Entry + exit' };
  return <span className={`chip ${styles[type] || styles.both}`}>{labels[type] || type}</span>;
}
