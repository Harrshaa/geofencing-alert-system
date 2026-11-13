import React, { useEffect, useRef, useState } from 'react';
import { api } from '../api';
import { CATEGORIES } from '../config';
import { closeRing, formatDateTime, parseCoordinates } from '../utils';
import { CategoryChip, EmptyState, ErrorNote, Field, Spinner, SubmitButton } from './ui';

const EMPTY_FORM = { name: '', description: '', category: 'delivery_zone' };

// Create form + list of geofences.
//
// The polygon corners ("draft") live in App because the map also needs them:
// clicking the map adds a corner, and the map draws the outline. This panel
// shows the same corners as editable JSON text, so a geofence can be created
// either by drawing or by pasting coordinates.
function GeofencePanel({
  geofences,
  loading,
  categoryFilter,
  onCategoryFilterChange,
  draft,
  onDraftChange,
  drawing,
  onToggleDrawing,
  onCreated,
  onFocus,
}) {
  const [form, setForm] = useState(EMPTY_FORM);
  const [coordText, setCoordText] = useState('');
  const [coordError, setCoordError] = useState('');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  // True while the latest draft change came from typing in the text box.
  // In that case we must not overwrite the text the user is still editing.
  const typedRef = useRef(false);

  // When the draft changes because of a map click, show it in the text box.
  useEffect(() => {
    if (typedRef.current) {
      typedRef.current = false;
      return;
    }
    setCoordText(draft.length > 0 ? JSON.stringify(draft) : '');
    setCoordError('');
  }, [draft]);

  function handleCoordTextChange(event) {
    const text = event.target.value;
    setCoordText(text);

    const { points, error: parseError } = parseCoordinates(text);
    if (parseError) {
      setCoordError(parseError); // keep the old outline on the map until the text is valid
      return;
    }
    setCoordError('');
    typedRef.current = true;
    onDraftChange(points);
  }

  function updateField(event) {
    const { name, value } = event.target;
    setForm((current) => ({ ...current, [name]: value }));
  }

  async function handleSubmit(event) {
    event.preventDefault();
    setError('');

    if (!form.name.trim()) {
      setError('Give the geofence a name.');
      return;
    }
    if (coordError) {
      setError('Fix the coordinates before saving.');
      return;
    }
    if (draft.length < 3) {
      setError('A geofence needs at least 3 corners. Draw them on the map or paste coordinates.');
      return;
    }

    setSaving(true);
    try {
      const created = await api.createGeofence({
        name: form.name.trim(),
        description: form.description.trim(),
        category: form.category,
        coordinates: closeRing(draft), // the API requires first point = last point
      });
      setForm(EMPTY_FORM);
      onCreated(created);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-4">
      <form onSubmit={handleSubmit} className="card space-y-3" noValidate>
        <h2 className="section-title">New geofence</h2>

        <Field label="Name">
          <input
            className="input"
            name="name"
            value={form.name}
            onChange={updateField}
            placeholder="Downtown Delivery Zone"
          />
        </Field>

        <Field label="Description (optional)">
          <input
            className="input"
            name="description"
            value={form.description}
            onChange={updateField}
            placeholder="Main delivery area for downtown customers"
          />
        </Field>

        <Field label="Category">
          <select className="input" name="category" value={form.category} onChange={updateField}>
            {CATEGORIES.map((category) => (
              <option key={category.value} value={category.value}>
                {category.label}
              </option>
            ))}
          </select>
        </Field>

        <div>
          <span className="label">Boundary</span>
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              className={drawing ? 'btn-primary btn-small' : 'btn-secondary btn-small'}
              onClick={onToggleDrawing}
              aria-pressed={drawing}
            >
              {drawing ? 'Stop drawing' : 'Draw on map'}
            </button>
            <button
              type="button"
              className="btn-secondary btn-small"
              onClick={() => onDraftChange(draft.slice(0, -1))}
              disabled={draft.length === 0}
            >
              Undo point
            </button>
            <button
              type="button"
              className="btn-secondary btn-small"
              onClick={() => onDraftChange([])}
              disabled={draft.length === 0}
            >
              Clear
            </button>
            <span className="text-xs text-slate-500">
              {draft.length} {draft.length === 1 ? 'point' : 'points'}
            </span>
          </div>
        </div>

        <Field
          label="Coordinates as [latitude, longitude] pairs"
          hint="Filled in as you draw. You can also paste JSON here. The polygon is closed for you."
        >
          <textarea
            className="input font-mono text-xs"
            rows={3}
            value={coordText}
            onChange={handleCoordTextChange}
            placeholder="[[12.97, 77.59], [12.98, 77.59], [12.98, 77.60]]"
            spellCheck={false}
          />
        </Field>
        <ErrorNote message={coordError} />

        <ErrorNote message={error} />
        <SubmitButton busy={saving}>Create geofence</SubmitButton>
      </form>

      <section className="space-y-2">
        <div className="flex items-center justify-between gap-2">
          <h2 className="section-title">Geofences ({geofences.length})</h2>
          <select
            className="input w-auto"
            value={categoryFilter}
            onChange={(event) => onCategoryFilterChange(event.target.value)}
            aria-label="Filter geofences by category"
          >
            <option value="">All categories</option>
            {CATEGORIES.map((category) => (
              <option key={category.value} value={category.value}>
                {category.label}
              </option>
            ))}
          </select>
        </div>

        {loading && <Spinner label="Loading geofences…" />}
        {!loading && geofences.length === 0 && (
          <EmptyState>
            {categoryFilter ? 'No geofences in this category.' : 'No geofences yet. Create the first one above.'}
          </EmptyState>
        )}

        <ul className="space-y-2">
          {geofences.map((geofence) => (
            <GeofenceRow key={geofence.id} geofence={geofence} onFocus={onFocus} />
          ))}
        </ul>
      </section>
    </div>
  );
}

// React.memo: a row re-renders only when its own geofence changes, not when
// the user types in the form above.
const GeofenceRow = React.memo(function GeofenceRow({ geofence, onFocus }) {
  return (
    <li className="card">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">{geofence.name}</p>
          <p className="text-xs text-slate-500">
            {geofence.id} · created {formatDateTime(geofence.created_at)}
          </p>
        </div>
        <button type="button" className="btn-secondary btn-small shrink-0" onClick={() => onFocus(geofence)}>
          Show on map
        </button>
      </div>
      {geofence.description && <p className="mt-1 text-sm text-slate-600">{geofence.description}</p>}
      <div className="mt-2">
        <CategoryChip category={geofence.category} />
      </div>
    </li>
  );
});

export default GeofencePanel;
