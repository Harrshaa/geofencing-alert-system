import React from 'react';
import { formatCoord, formatDateTime } from '../utils';
import { CategoryChip, EmptyState, EventChip } from './ui';

const STATUS_TEXT = {
  open: 'Connected. Alerts appear here the moment they happen.',
  connecting: 'Connecting to the alert stream…',
  closed: 'Disconnected. Trying to reconnect…',
};

// The live feed: every alert received over the WebSocket since the page loaded.
function AlertFeed({ alerts, socketStatus, onClear, onShowOnMap }) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <h2 className="section-title">Live alerts ({alerts.length})</h2>
        <button type="button" className="btn-secondary btn-small" onClick={onClear} disabled={alerts.length === 0}>
          Clear
        </button>
      </div>

      <p className="text-xs text-slate-500">{STATUS_TEXT[socketStatus]}</p>

      {alerts.length === 0 && (
        <EmptyState>
          No alerts yet. Create an alert rule, then move a vehicle into or out of that geofence.
        </EmptyState>
      )}

      <ul className="space-y-2">
        {alerts.map((alert) => (
          <AlertRow key={alert.event_id} alert={alert} onShowOnMap={onShowOnMap} />
        ))}
      </ul>
    </div>
  );
}

// React.memo: when a new alert arrives, the rows already on screen do not
// re-render, because their own alert object has not changed.
const AlertRow = React.memo(function AlertRow({ alert, onShowOnMap }) {
  const entered = alert.event_type === 'entry';
  return (
    <li className={`card border-l-4 ${entered ? 'border-l-red-500' : 'border-l-emerald-500'}`}>
      <div className="flex items-start justify-between gap-2">
        <p className="text-sm">
          <span className="font-semibold">{alert.vehicle.vehicle_number}</span>{' '}
          {entered ? 'entered' : 'left'} <span className="font-semibold">{alert.geofence.geofence_name}</span>
        </p>
        <EventChip type={alert.event_type} />
      </div>
      <p className="mt-1 text-xs text-slate-600">Driver: {alert.vehicle.driver_name}</p>
      <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-slate-500">
        <CategoryChip category={alert.geofence.category} />
        <span>{formatDateTime(alert.timestamp)}</span>
        <button
          type="button"
          className="underline decoration-dotted hover:text-slate-800"
          onClick={() => onShowOnMap([alert.location.latitude, alert.location.longitude])}
        >
          {formatCoord(alert.location.latitude)}, {formatCoord(alert.location.longitude)}
        </button>
      </div>
    </li>
  );
});

export default React.memo(AlertFeed);
