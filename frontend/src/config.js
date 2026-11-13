// Where the backend lives. Parcel replaces process.env.API_URL with the real
// value when it builds the bundle (from a .env file or the environment).
export const API_URL = (process.env.API_URL || 'http://localhost:8080').replace(/\/+$/, '');

// The WebSocket address is derived from the API address:
// http://  -> ws://   and   https:// -> wss://
// A page served over HTTPS is only allowed to open wss:// sockets.
export const WS_URL = API_URL.replace(/^http/, 'ws') + '/ws/alerts';

// Geofence categories from the brief, with a label and a colour for the map.
export const CATEGORIES = [
  { value: 'delivery_zone', label: 'Delivery zone', color: '#2563eb' },
  { value: 'restricted_zone', label: 'Restricted zone', color: '#dc2626' },
  { value: 'toll_zone', label: 'Toll zone', color: '#d97706' },
  { value: 'customer_area', label: 'Customer area', color: '#059669' },
];

export const EVENT_TYPES = [
  { value: 'entry', label: 'Entry' },
  { value: 'exit', label: 'Exit' },
  { value: 'both', label: 'Entry and exit' },
];

export const VEHICLE_TYPES = ['truck', 'van', 'car', 'motorcycle'];

export function categoryInfo(value) {
  return CATEGORIES.find((c) => c.value === value) || { value, label: value, color: '#6b7280' };
}

// Map start position (Bengaluru) used until there is data to zoom to.
export const DEFAULT_CENTER = [12.9716, 77.5946];
export const DEFAULT_ZOOM = 12;
