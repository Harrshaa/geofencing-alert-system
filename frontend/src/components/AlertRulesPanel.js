import React, { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import { EVENT_TYPES } from '../config';
import { formatDateTime } from '../utils';
import { EmptyState, ErrorNote, EventChip, Field, Spinner, SubmitButton } from './ui';

// Configure alert rules and manage the existing ones.
// A rule says: "alert me when <vehicle or any vehicle> does <entry/exit/both> on <geofence>".
function AlertRulesPanel({ geofences, vehicles, onNotify }) {
  const [rules, setRules] = useState([]);
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState('');
  const [filters, setFilters] = useState({ geofence_id: '', vehicle_id: '' });

  const [form, setForm] = useState({ geofence_id: '', vehicle_id: '', event_type: 'entry' });
  const [formError, setFormError] = useState('');
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState('');

  const loadRules = useCallback(async () => {
    setLoading(true);
    setListError('');
    try {
      const data = await api.listAlerts(filters);
      setRules(data.alerts);
    } catch (err) {
      setListError(err.message);
    } finally {
      setLoading(false);
    }
  }, [filters]);

  // Reload whenever the filters change (and once on first render).
  useEffect(() => {
    loadRules();
  }, [loadRules]);

  function updateForm(event) {
    const { name, value } = event.target;
    setForm((current) => ({ ...current, [name]: value }));
  }

  function updateFilter(event) {
    const { name, value } = event.target;
    setFilters((current) => ({ ...current, [name]: value }));
  }

  async function handleSubmit(event) {
    event.preventDefault();
    setFormError('');
    if (!form.geofence_id) {
      setFormError('Choose a geofence.');
      return;
    }

    setSaving(true);
    try {
      const rule = { geofence_id: form.geofence_id, event_type: form.event_type };
      // vehicle_id is optional: leaving it out means "all vehicles".
      if (form.vehicle_id) rule.vehicle_id = form.vehicle_id;

      await api.configureAlert(rule);
      onNotify({ kind: 'success', title: 'Alert rule created' });
      await loadRules();
    } catch (err) {
      setFormError(err.message);
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(alertId) {
    setDeletingId(alertId);
    try {
      await api.deleteAlert(alertId);
      setRules((current) => current.filter((rule) => rule.alert_id !== alertId));
      onNotify({ kind: 'success', title: 'Alert rule removed' });
    } catch (err) {
      onNotify({ kind: 'error', title: 'Could not remove the rule', message: err.message });
    } finally {
      setDeletingId('');
    }
  }

  return (
    <div className="space-y-4">
      <form onSubmit={handleSubmit} className="card space-y-3" noValidate>
        <h2 className="section-title">New alert rule</h2>

        <Field label="Geofence">
          <select
            className="input"
            name="geofence_id"
            value={form.geofence_id}
            onChange={updateForm}
            disabled={geofences.length === 0}
          >
            <option value="">{geofences.length === 0 ? 'Create a geofence first' : 'Choose a geofence…'}</option>
            {geofences.map((geofence) => (
              <option key={geofence.id} value={geofence.id}>
                {geofence.name}
              </option>
            ))}
          </select>
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Vehicle">
            <select className="input" name="vehicle_id" value={form.vehicle_id} onChange={updateForm}>
              <option value="">All vehicles</option>
              {vehicles.map((vehicle) => (
                <option key={vehicle.id} value={vehicle.id}>
                  {vehicle.vehicle_number}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Alert on">
            <select className="input" name="event_type" value={form.event_type} onChange={updateForm}>
              {EVENT_TYPES.map((type) => (
                <option key={type.value} value={type.value}>
                  {type.label}
                </option>
              ))}
            </select>
          </Field>
        </div>

        <ErrorNote message={formError} />
        <SubmitButton busy={saving}>Create rule</SubmitButton>
      </form>

      <section className="space-y-2">
        <h2 className="section-title">Alert rules ({rules.length})</h2>

        <div className="grid grid-cols-2 gap-2">
          <select
            className="input"
            name="geofence_id"
            value={filters.geofence_id}
            onChange={updateFilter}
            aria-label="Filter rules by geofence"
          >
            <option value="">All geofences</option>
            {geofences.map((geofence) => (
              <option key={geofence.id} value={geofence.id}>
                {geofence.name}
              </option>
            ))}
          </select>
          <select
            className="input"
            name="vehicle_id"
            value={filters.vehicle_id}
            onChange={updateFilter}
            aria-label="Filter rules by vehicle"
          >
            <option value="">All vehicles</option>
            {vehicles.map((vehicle) => (
              <option key={vehicle.id} value={vehicle.id}>
                {vehicle.vehicle_number}
              </option>
            ))}
          </select>
        </div>

        {loading && <Spinner label="Loading rules…" />}
        <ErrorNote message={listError} />
        {!loading && !listError && rules.length === 0 && (
          <EmptyState>No alert rules match. Without a rule, events are recorded in history but no alert is sent.</EmptyState>
        )}

        <ul className="space-y-2">
          {rules.map((rule) => (
            <li key={rule.alert_id} className="card flex items-start justify-between gap-2">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{rule.geofence_name}</p>
                <p className="mt-0.5 text-xs text-slate-600">
                  {rule.vehicle_number ? `Vehicle ${rule.vehicle_number}` : 'All vehicles'}
                </p>
                <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                  <EventChip type={rule.event_type} />
                  <span className="text-xs text-slate-500">
                    {rule.alert_id} · {formatDateTime(rule.created_at)}
                  </span>
                </div>
              </div>
              <button
                type="button"
                className="btn-danger btn-small shrink-0"
                onClick={() => handleDelete(rule.alert_id)}
                disabled={deletingId === rule.alert_id}
              >
                {deletingId === rule.alert_id ? 'Removing…' : 'Remove'}
              </button>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

export default AlertRulesPanel;
