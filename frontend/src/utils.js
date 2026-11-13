// Small helpers shared by several components.

// "11 Nov 2026, 15:35:00" in the viewer's own time zone.
export function formatDateTime(iso) {
  if (!iso) return '';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString(undefined, {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

export function formatCoord(value) {
  return Number(value).toFixed(5);
}

// Value for an <input type="datetime-local"> showing the given date in local time.
export function toLocalInputValue(date = new Date()) {
  const pad = (n) => String(n).padStart(2, '0');
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  );
}

// Turns a datetime-local value (local time, no zone) into the ISO 8601 UTC
// string the API expects. Returns '' for an empty or invalid value.
export function localInputToISO(value) {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : date.toISOString();
}

// A polygon must end on its first point. If the user did not close it, do it for them.
export function closeRing(points) {
  if (points.length === 0) return points;
  const first = points[0];
  const last = points[points.length - 1];
  if (first[0] === last[0] && first[1] === last[1]) return points;
  return [...points, [first[0], first[1]]];
}

// Parses coordinates typed as JSON, e.g. [[12.97, 77.59], [12.98, 77.59], ...].
// Returns { points } on success or { error } with a message for the user.
export function parseCoordinates(text) {
  const trimmed = text.trim();
  if (!trimmed) return { points: [] };

  let parsed;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return { error: 'Coordinates must be valid JSON, e.g. [[12.97, 77.59], [12.98, 77.59], [12.98, 77.60]]' };
  }
  if (!Array.isArray(parsed)) {
    return { error: 'Coordinates must be a list of [latitude, longitude] pairs' };
  }
  for (let i = 0; i < parsed.length; i += 1) {
    const point = parsed[i];
    const ok =
      Array.isArray(point) &&
      point.length === 2 &&
      typeof point[0] === 'number' &&
      typeof point[1] === 'number';
    if (!ok) return { error: `Point ${i + 1} must be a [latitude, longitude] pair of numbers` };
    if (point[0] < -90 || point[0] > 90) return { error: `Point ${i + 1}: latitude must be between -90 and 90` };
    if (point[1] < -180 || point[1] > 180) return { error: `Point ${i + 1}: longitude must be between -180 and 180` };
  }
  return { points: parsed };
}

// True when lat/lng (strings from inputs, or numbers) form a valid position.
export function isValidPosition(lat, lng) {
  if (lat === '' || lng === '' || lat === null || lng === null) return false;
  const latNum = Number(lat);
  const lngNum = Number(lng);
  return (
    Number.isFinite(latNum) && Number.isFinite(lngNum) && latNum >= -90 && latNum <= 90 && lngNum >= -180 && lngNum <= 180
  );
}
