// All calls to the backend go through this file, so components never build
// URLs or parse responses themselves.
import { API_URL } from './config';

// The login token, set by App.js. null when not logged in or not needed.
let authToken = null;

// Called when the backend answers 401 (token missing, invalid or expired),
// so App.js can show the login page.
let unauthorizedHandler = null;

export function setAuthToken(token) {
  authToken = token || null;
}

export function setUnauthorizedHandler(handler) {
  unauthorizedHandler = handler;
}

// request sends one HTTP call and returns the parsed JSON body.
// On any failure it throws an Error whose message is safe to show the user.
async function request(path, { method = 'GET', body, params } = {}) {
  const url = new URL(API_URL + path);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      // Skip empty filters instead of sending "?vehicle_id=".
      if (value !== undefined && value !== null && value !== '') {
        url.searchParams.set(key, value);
      }
    }
  }

  let response;
  try {
    const headers = {};
    if (body) headers['Content-Type'] = 'application/json';
    // Every request carries the token when we have one:
    // "Authorization: Bearer <token>". The backend ignores it when login is off.
    if (authToken) headers.Authorization = `Bearer ${authToken}`;

    response = await fetch(url, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
  } catch {
    // fetch only rejects when the request never got a response at all.
    throw new Error('Cannot reach the server. Check that the backend is running.');
  }

  let data = null;
  try {
    data = await response.json();
  } catch {
    // Not JSON; handled below.
  }

  if (!response.ok) {
    // A 401 from any endpoint except login means "you need to log in (again)".
    // A 401 from login itself just means wrong email or password.
    if (response.status === 401 && path !== '/auth/login' && unauthorizedHandler) {
      unauthorizedHandler();
    }
    throw new Error((data && data.error) || `Request failed (${response.status})`);
  }
  return data;
}

export const api = {
  // Authentication
  authStatus: () => request('/auth/status'),
  login: (email, password) => request('/auth/login', { method: 'POST', body: { email, password } }),

  // Geofences
  listGeofences: (category) => request('/geofences', { params: { category } }),
  createGeofence: (geofence) => request('/geofences', { method: 'POST', body: geofence }),

  // Vehicles
  listVehicles: () => request('/vehicles'),
  createVehicle: (vehicle) => request('/vehicles', { method: 'POST', body: vehicle }),

  // Locations
  updateLocation: (update) => request('/vehicles/location', { method: 'POST', body: update }),
  getVehicleLocation: (vehicleId) => request(`/vehicles/location/${encodeURIComponent(vehicleId)}`),

  // Alert rules
  listAlerts: (filters) => request('/alerts', { params: filters }),
  configureAlert: (rule) => request('/alerts/configure', { method: 'POST', body: rule }),
  deleteAlert: (alertId) => request(`/alerts/${encodeURIComponent(alertId)}`, { method: 'DELETE' }),

  // History
  listViolations: (filters) => request('/violations/history', { params: filters }),
};
