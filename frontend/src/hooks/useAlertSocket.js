import { useEffect, useRef, useState } from 'react';
import { WS_URL } from '../config';

const FIRST_RETRY_MS = 1000;
const MAX_RETRY_MS = 15000;

// useAlertSocket keeps one WebSocket open to the backend for the life of the
// page and calls onAlert(alert) for every alert the server pushes.
//
// If the connection drops (server restart, laptop sleep, network change) it
// reconnects by itself, waiting 1s, 2s, 4s ... up to 15s between attempts so
// a server that is down is not hammered.
//
// token: when login is required, the JWT. Browsers cannot add an
// Authorization header to a WebSocket, so it is sent as ?token=...
// A new token closes the old connection and opens a new one.
//
// Returns the connection status: 'connecting' | 'open' | 'closed'.
export function useAlertSocket(onAlert, token = null) {
  const [status, setStatus] = useState('connecting');

  // Keep the newest onAlert in a ref. The effect below runs only once, so
  // without the ref it would keep calling the very first onAlert forever
  // (a "stale closure") and miss later state.
  const onAlertRef = useRef(onAlert);
  useEffect(() => {
    onAlertRef.current = onAlert;
  }, [onAlert]);

  useEffect(() => {
    let socket = null;
    let retryTimer = null;
    let retryDelay = FIRST_RETRY_MS;
    let stopped = false; // set when the component unmounts
    const url = token ? `${WS_URL}?token=${encodeURIComponent(token)}` : WS_URL;

    function connect() {
      setStatus('connecting');
      socket = new WebSocket(url);

      socket.onopen = () => {
        retryDelay = FIRST_RETRY_MS; // a good connection resets the backoff
        setStatus('open');
      };

      socket.onmessage = (event) => {
        try {
          onAlertRef.current(JSON.parse(event.data));
        } catch (err) {
          console.error('Ignoring a message that is not valid JSON', err);
        }
      };

      socket.onclose = () => {
        if (stopped) return;
        setStatus('closed');
        retryTimer = setTimeout(connect, retryDelay);
        retryDelay = Math.min(retryDelay * 2, MAX_RETRY_MS);
      };

      // An error is always followed by onclose, which does the reconnect.
      socket.onerror = () => socket.close();
    }

    connect();

    // Cleanup: runs when the component unmounts.
    return () => {
      stopped = true;
      clearTimeout(retryTimer);
      if (socket) socket.close();
    };
  }, [token]);

  return status;
}
