import React from 'react';

const STYLES = {
  success: 'border-emerald-300 bg-emerald-50 text-emerald-900',
  error: 'border-red-300 bg-red-50 text-red-900',
  entry: 'border-red-400 bg-red-600 text-white',
  exit: 'border-emerald-500 bg-emerald-600 text-white',
};

// Stack of pop-up notifications in the top-right corner.
// aria-live makes screen readers announce new ones.
function Toasts({ toasts, onDismiss }) {
  return (
    <div
      className="pointer-events-none fixed right-3 top-3 z-[2000] flex w-[min(22rem,calc(100vw-1.5rem))] flex-col gap-2"
      aria-live="polite"
    >
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={`toast pointer-events-auto flex items-start gap-2 rounded-lg border px-3 py-2.5 shadow-lg ${
            STYLES[toast.kind] || STYLES.success
          }`}
        >
          <div className="min-w-0 flex-1">
            <p className="text-sm font-semibold">{toast.title}</p>
            {toast.message && <p className="mt-0.5 break-words text-xs opacity-90">{toast.message}</p>}
          </div>
          <button
            type="button"
            onClick={() => onDismiss(toast.id)}
            className="shrink-0 rounded px-1 text-sm opacity-70 hover:opacity-100"
            aria-label="Dismiss notification"
          >
            ✕
          </button>
        </div>
      ))}
    </div>
  );
}

export default React.memo(Toasts);
