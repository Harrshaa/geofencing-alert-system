import { useCallback, useRef, useState } from 'react';

const TOAST_LIFETIME_MS = 6000;
const MAX_TOASTS = 5;

// useToasts manages the stack of pop-up notifications.
// pushToast({ kind, title, message }) shows one; it disappears after 6 seconds.
// kind is 'success' | 'error' | 'entry' | 'exit'.
export function useToasts() {
  const [toasts, setToasts] = useState([]);
  const nextId = useRef(1);

  const dismissToast = useCallback((id) => {
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }, []);

  const pushToast = useCallback(
    (toast) => {
      const id = nextId.current;
      nextId.current += 1;
      // Keep only the newest few so a burst of alerts cannot cover the screen.
      setToasts((current) => [...current, { ...toast, id }].slice(-MAX_TOASTS));
      setTimeout(() => dismissToast(id), TOAST_LIFETIME_MS);
    },
    [dismissToast],
  );

  return { toasts, pushToast, dismissToast };
}
