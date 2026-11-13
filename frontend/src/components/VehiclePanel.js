import React, { useEffect, useRef, useState } from 'react';
import { api } from '../api';
import { VEHICLE_TYPES } from '../config';
import { formatCoord, formatDateTime, isValidPosition, localInputToISO, toLocalInputValue } from '../utils';
import { CategoryChip, EmptyState, ErrorNote, Field, Spinner, SubmitButton } from './ui';

const EMPTY_FORM = { vehicle_number: '', driver_name: '', vehicle_type: 'truck', phone: '' };

// Register form, vehicle list with live status, and the location update form.
function VehiclePanel({
  vehicles,
  statuses,
  loading,
  selectedVehicleId,
  onSelectVehicle,
  picked,
  onPickedChange,
  picking,
  onTogglePicking,
  onCreated,
  onLocationUpdated,
  onFocus,
}) {
  return (
    <div className="space-y-4">
      <RegisterForm onCreated={onCreated} />

      <LocationForm
        vehicles={vehicles}
        selectedVehicleId={selectedVehicleId}
        onSelectVehicle={onSelectVehicle}
        picked={picked}
        onPickedChange={onPickedChange}
        picking={picking}
        onTogglePicking={onTogglePicking}
        onLocationUpdated={onLocationUpdated}
      />

      <section className="space-y-2">
        <h2 className="section-title">Vehicles ({vehicles.length})</h2>
        {loading && <Spinner label="Loading vehicles…" />}
        {!loading && vehicles.length === 0 && <EmptyState>No vehicles yet. Register the first one above.</EmptyState>}
        <ul className="space-y-2">
          {vehicles.map((vehicle) => (
            <VehicleRow
              key={vehicle.id}
              vehicle={vehicle}
              status={statuses[vehicle.id]}
              selected={vehicle.id === selectedVehicleId}
              onSelectVehicle={onSelectVehicle}
              onFocus={onFocus}
            />
          ))}
        </ul>
      </section>
    </div>
  );
}

function RegisterForm({ onCreated }) {
  const [form, setForm] = useState(EMPTY_FORM);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  function updateField(event) {
    const { name, value } = event.target;
    setForm((current) => ({ ...current, [name]: value }));
  }

  async function handleSubmit(event) {
    event.preventDefault();
    setError('');

    const missing = ['vehicle_number', 'driver_name', 'vehicle_type', 'phone'].find((key) => !form[key].trim());
    if (missing) {
      setError(`Fill in ${missing.replace('_', ' ')}.`);
      return;
    }

    setSaving(true);
    try {
      const created = await api.createVehicle({
        vehicle_number: form.vehicle_number.trim(),
        driver_name: form.driver_name.trim(),
        vehicle_type: form.vehicle_type.trim(),
        phone: form.phone.trim(),
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
    <form onSubmit={handleSubmit} className="card space-y-3" noValidate>
      <h2 className="section-title">Register vehicle</h2>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Vehicle number">
          <input
            className="input"
            name="vehicle_number"
            value={form.vehicle_number}
            onChange={updateField}
            placeholder="KA-01-AB-1234"
          />
        </Field>
        <Field label="Vehicle type">
          <select className="input" name="vehicle_type" value={form.vehicle_type} onChange={updateField}>
            {VEHICLE_TYPES.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Driver name">
          <input
            className="input"
            name="driver_name"
            value={form.driver_name}
            onChange={updateField}
            placeholder="John Doe"
          />
        </Field>
        <Field label="Phone">
          <input
            className="input"
            name="phone"
            type="tel"
            value={form.phone}
            onChange={updateField}
            placeholder="+1234567890"
          />
        </Field>
      </div>
      <ErrorNote message={error} />
      <SubmitButton busy={saving}>Register vehicle</SubmitButton>
    </form>
  );
}

function LocationForm({
  vehicles,
  selectedVehicleId,
  onSelectVehicle,
  picked,
  onPickedChange,
  picking,
  onTogglePicking,
  onLocationUpdated,
}) {
  const [useNow, setUseNow] = useState(true);
  const [timestamp, setTimestamp] = useState(() => toLocalInputValue());
  const [sendOnClick, setSendOnClick] = useState(false);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [lastResult, setLastResult] = useState(null);

  async function send(lat, lng) {
    setError('');
    if (!selectedVehicleId) {
      setError('Choose a vehicle first.');
      return;
    }
    if (!isValidPosition(lat, lng)) {
      setError('Enter a latitude between -90 and 90 and a longitude between -180 and 180, or click the map.');
      return;
    }
    const isoTimestamp = useNow ? new Date().toISOString() : localInputToISO(timestamp);
    if (!isoTimestamp) {
      setError('Enter a valid date and time.');
      return;
    }

    setSaving(true);
    try {
      const result = await api.updateLocation({
        vehicle_id: selectedVehicleId,
        latitude: Number(lat),
        longitude: Number(lng),
        timestamp: isoTimestamp,
      });
      setLastResult(result);
      onLocationUpdated(result);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  // "Send on map click": App increases picked.clickId on every map click.
  // When it changes (and the option is on) we send that position straight away.
  // The ref remembers the last click we handled so typing in the inputs or
  // toggling the checkbox never triggers a send.
  const handledClickRef = useRef(picked.clickId);
  useEffect(() => {
    if (picked.clickId === handledClickRef.current) return;
    handledClickRef.current = picked.clickId;
    if (sendOnClick) send(picked.lat, picked.lng);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [picked.clickId]);

  // A result belongs to one vehicle; hide it when another is chosen.
  useEffect(() => {
    setLastResult(null);
    setError('');
  }, [selectedVehicleId]);

  function handleSubmit(event) {
    event.preventDefault();
    send(picked.lat, picked.lng);
  }

  return (
    <form onSubmit={handleSubmit} className="card space-y-3" noValidate>
      <h2 className="section-title">Update vehicle location</h2>

      <Field label="Vehicle">
        <select
          className="input"
          value={selectedVehicleId}
          onChange={(event) => onSelectVehicle(event.target.value)}
          disabled={vehicles.length === 0}
        >
          <option value="">{vehicles.length === 0 ? 'Register a vehicle first' : 'Choose a vehicle…'}</option>
          {vehicles.map((vehicle) => (
            <option key={vehicle.id} value={vehicle.id}>
              {vehicle.vehicle_number} ({vehicle.driver_name})
            </option>
          ))}
        </select>
      </Field>

      <div className="grid grid-cols-2 gap-3">
        <Field label="Latitude">
          <input
            className="input"
            inputMode="decimal"
            value={picked.lat}
            onChange={(event) => onPickedChange({ lat: event.target.value })}
            placeholder="12.9716"
          />
        </Field>
        <Field label="Longitude">
          <input
            className="input"
            inputMode="decimal"
            value={picked.lng}
            onChange={(event) => onPickedChange({ lng: event.target.value })}
            placeholder="77.5946"
          />
        </Field>
      </div>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <button
          type="button"
          className={picking ? 'btn-primary btn-small' : 'btn-secondary btn-small'}
          onClick={onTogglePicking}
          aria-pressed={picking}
        >
          {picking ? 'Stop picking' : 'Pick on map'}
        </button>
        <label className="flex items-center gap-1.5 text-xs text-slate-600">
          <input type="checkbox" checked={sendOnClick} onChange={(event) => setSendOnClick(event.target.checked)} />
          Send as soon as I click the map
        </label>
      </div>

      <div className="space-y-2">
        <label className="flex items-center gap-1.5 text-xs text-slate-600">
          <input type="checkbox" checked={useNow} onChange={(event) => setUseNow(event.target.checked)} />
          Use the current time as the timestamp
        </label>
        {!useNow && (
          <Field label="Timestamp (your local time)">
            <input
              className="input"
              type="datetime-local"
              step="1"
              value={timestamp}
              onChange={(event) => setTimestamp(event.target.value)}
            />
          </Field>
        )}
      </div>

      <ErrorNote message={error} />
      <SubmitButton busy={saving} busyLabel="Sending…">
        Update location
      </SubmitButton>

      {lastResult && (
        <div className="rounded-md bg-slate-50 px-2.5 py-2 text-sm">
          <p className="font-medium text-slate-700">
            {lastResult.current_geofences.length > 0 ? 'Vehicle is now inside:' : 'Vehicle is not inside any geofence.'}
          </p>
          {lastResult.current_geofences.length > 0 && (
            <ul className="mt-1 list-inside list-disc text-slate-600">
              {lastResult.current_geofences.map((geofence) => (
                <li key={geofence.geofence_id}>{geofence.geofence_name}</li>
              ))}
            </ul>
          )}
        </div>
      )}
    </form>
  );
}

const VehicleRow = React.memo(function VehicleRow({ vehicle, status, selected, onSelectVehicle, onFocus }) {
  const location = status && status.current_location;
  const inside = (status && status.current_geofences) || [];

  return (
    <li className={`card ${selected ? 'border-blue-400 ring-1 ring-blue-400' : ''}`}>
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">{vehicle.vehicle_number}</p>
          <p className="truncate text-xs text-slate-500">
            {vehicle.driver_name} · {vehicle.vehicle_type} · {vehicle.phone}
          </p>
        </div>
        <div className="flex shrink-0 gap-1.5">
          <button
            type="button"
            className="btn-secondary btn-small"
            onClick={() => onFocus(vehicle.id)}
            disabled={!location}
            title={location ? 'Centre the map on this vehicle' : 'No position reported yet'}
          >
            Show
          </button>
          <button type="button" className="btn-secondary btn-small" onClick={() => onSelectVehicle(vehicle.id)}>
            {selected ? 'Selected' : 'Select'}
          </button>
        </div>
      </div>

      <div className="mt-2 text-xs text-slate-600">
        {!status && <Spinner label="Loading position…" />}
        {status && !location && <span>No position reported yet.</span>}
        {location && (
          <span>
            {formatCoord(location.latitude)}, {formatCoord(location.longitude)} · {formatDateTime(location.timestamp)}
          </span>
        )}
      </div>

      {location && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {inside.length === 0 && <span className="chip bg-slate-100 text-slate-600">Outside all geofences</span>}
          {inside.map((geofence) => (
            <span key={geofence.geofence_id} className="inline-flex items-center gap-1 text-xs text-slate-700">
              <CategoryChip category={geofence.category} />
              {geofence.geofence_name}
            </span>
          ))}
        </div>
      )}
    </li>
  );
});

export default VehiclePanel;
