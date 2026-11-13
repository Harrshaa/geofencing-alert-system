import React, { useCallback, useEffect, useRef, useState } from 'react';

import { api } from './api';
import { categoryInfo } from './config';
import { useAlertSocket } from './hooks/useAlertSocket';
import { useToasts } from './hooks/useToasts';
import { formatDateTime } from './utils';

import AlertFeed from './components/AlertFeed';
import AlertRulesPanel from './components/AlertRulesPanel';
import GeofencePanel from './components/GeofencePanel';
import HistoryPanel from './components/HistoryPanel';
import MapView from './components/MapView';
import Toasts from './components/Toasts';
import VehiclePanel from './components/VehiclePanel';

const TABS = [
  { id: 'geofences', label: 'Geofences' },
  { id: 'vehicles', label: 'Vehicles' },
  { id: 'rules', label: 'Rules' },
  { id: 'live', label: 'Live' },
  { id: 'history', label: 'History' },
];

const MAX_FEED_ALERTS = 200; // the feed keeps the newest 200 alerts in memory

// Dashboard is the main screen. It owns the data that more than one part of
// the screen needs: the map and the side panels both read geofences, vehicles
// and positions. State that only one panel needs (form fields, history
// filters) stays in that panel.
//
// session is { token, email } when the user logged in, or null when the
// backend does not require login. onLogout signs the user out.
export default function Dashboard({ session, onLogout }) {
  const [tab, setTab] = useState('geofences');

  // ----- data from the server -----
  const [geofences, setGeofences] = useState([]); // all geofences
  const [categoryFilter, setCategoryFilter] = useState('');
  const [filteredGeofences, setFilteredGeofences] = useState(null); // result of ?category=, or null when no filter
  const [vehicles, setVehicles] = useState([]);
  const [statuses, setStatuses] = useState({}); // vehicle id -> { current_location, current_geofences }
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');

  // ----- map interaction -----
  const [mode, setMode] = useState(null); // null | 'draw' (geofence corners) | 'pick' (vehicle position)
  const [draft, setDraft] = useState([]); // corners of the geofence being drawn, as [lat, lng]
  const [picked, setPicked] = useState({ lat: '', lng: '', clickId: 0 }); // position for the next location update
  const [selectedVehicleId, setSelectedVehicleId] = useState('');
  const [focus, setFocus] = useState(null); // tells the map where to move

  // ----- live alerts -----
  const [alerts, setAlerts] = useState([]);
  const [unread, setUnread] = useState(0);
  const [historyKey, setHistoryKey] = useState(0); // bumped to make the history tab reload
  const { toasts, pushToast, dismissToast } = useToasts();

  // The WebSocket callback is created once but needs to know the current tab.
  const tabRef = useRef(tab);
  tabRef.current = tab;

  // Same idea for the lists: the callback needs the latest ones to tell
  // whether an alert mentions a vehicle or geofence this tab has not loaded.
  const knownRef = useRef({ geofences, vehicles });
  knownRef.current = { geofences, vehicles };

  // ---------- loading ----------

  const loadGeofences = useCallback(async () => {
    const data = await api.listGeofences();
    setGeofences(data.geofences);
    return data.geofences;
  }, []);

  const loadVehicles = useCallback(async () => {
    const data = await api.listVehicles();
    setVehicles(data.vehicles);

    // Fetch every vehicle's position at the same time, not one after another.
    // A failure for one vehicle must not hide the others, hence the catch.
    const results = await Promise.all(
      data.vehicles.map((vehicle) => api.getVehicleLocation(vehicle.id).catch(() => null)),
    );
    const next = {};
    data.vehicles.forEach((vehicle, index) => {
      next[vehicle.id] = results[index] || { current_location: null, current_geofences: [] };
    });
    setStatuses(next);
  }, []);

  const refreshStatus = useCallback(async (vehicleId) => {
    try {
      const status = await api.getVehicleLocation(vehicleId);
      setStatuses((current) => ({ ...current, [vehicleId]: status }));
    } catch {
      // Keep showing the last known position.
    }
  }, []);

  const loadAll = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const [allGeofences] = await Promise.all([loadGeofences(), loadVehicles()]);
      if (allGeofences.length > 0) {
        // Zoom the map so every geofence is visible.
        setFocus({ bounds: allGeofences.flatMap((geofence) => geofence.coordinates) });
      }
    } catch (err) {
      setLoadError(err.message);
    } finally {
      setLoading(false);
    }
  }, [loadGeofences, loadVehicles]);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  // Category filter: ask the server (GET /geofences?category=...). It runs
  // again when the full list changes, so a new geofence shows up here too.
  useEffect(() => {
    if (!categoryFilter) {
      setFilteredGeofences(null);
      return undefined;
    }
    let cancelled = false; // ignore the answer if the filter changed meanwhile
    api
      .listGeofences(categoryFilter)
      .then((data) => {
        if (!cancelled) setFilteredGeofences(data.geofences);
      })
      .catch((err) => {
        if (!cancelled) pushToast({ kind: 'error', title: 'Could not filter geofences', message: err.message });
      });
    return () => {
      cancelled = true;
    };
  }, [categoryFilter, geofences, pushToast]);

  const visibleGeofences = filteredGeofences || geofences;

  // ---------- live alerts ----------

  const handleAlert = useCallback(
    (alert) => {
      setAlerts((current) =>
        // Newest first; drop a repeat of the same event; cap the list length.
        [alert, ...current.filter((existing) => existing.event_id !== alert.event_id)].slice(0, MAX_FEED_ALERTS),
      );
      if (tabRef.current !== 'live') setUnread((count) => count + 1);

      const entered = alert.event_type === 'entry';
      pushToast({
        kind: entered ? 'entry' : 'exit',
        title: `${alert.vehicle.vehicle_number} ${entered ? 'entered' : 'left'} ${alert.geofence.geofence_name}`,
        message: `${categoryInfo(alert.geofence.category).label} · ${alert.vehicle.driver_name} · ${formatDateTime(
          alert.timestamp,
        )}`,
      });

      // The alert may come from another browser tab or a direct API call.
      // If it mentions a vehicle or geofence this tab has never loaded,
      // reload that list; otherwise just refresh the vehicle's position.
      const known = knownRef.current;
      if (known.vehicles.some((vehicle) => vehicle.id === alert.vehicle.vehicle_id)) {
        refreshStatus(alert.vehicle.vehicle_id);
      } else {
        loadVehicles().catch(() => {});
      }
      if (!known.geofences.some((geofence) => geofence.id === alert.geofence.geofence_id)) {
        loadGeofences().catch(() => {});
      }
      setHistoryKey((key) => key + 1);
    },
    [pushToast, refreshStatus, loadVehicles, loadGeofences],
  );

  const socketStatus = useAlertSocket(handleAlert, session ? session.token : null);

  // ---------- map ----------

  const handleMapClick = useCallback(
    (lat, lng) => {
      // 6 decimal places is about 10 cm: plenty, and it keeps the numbers readable.
      const point = [Number(lat.toFixed(6)), Number(lng.toFixed(6))];
      if (mode === 'draw') {
        setDraft((current) => [...current, point]);
      } else if (mode === 'pick') {
        setPicked((current) => ({ lat: String(point[0]), lng: String(point[1]), clickId: current.clickId + 1 }));
      }
    },
    [mode],
  );

  const showPointOnMap = useCallback((point) => setFocus({ point }), []);

  const focusGeofence = useCallback((geofence) => setFocus({ bounds: geofence.coordinates }), []);

  const focusVehicle = useCallback(
    (vehicleId) => {
      const location = statuses[vehicleId] && statuses[vehicleId].current_location;
      if (location) setFocus({ point: [location.latitude, location.longitude] });
    },
    [statuses],
  );

  // ---------- actions from the panels ----------

  const handleGeofenceCreated = useCallback(
    async (created) => {
      pushToast({ kind: 'success', title: 'Geofence created', message: created.name });
      setMode(null);
      try {
        const all = await loadGeofences();
        const fresh = all.find((geofence) => geofence.id === created.id);
        if (fresh) setFocus({ bounds: fresh.coordinates });
        setDraft([]);
      } catch (err) {
        pushToast({ kind: 'error', title: 'Could not reload geofences', message: err.message });
      }
    },
    [loadGeofences, pushToast],
  );

  const handleVehicleCreated = useCallback(
    async (created) => {
      pushToast({ kind: 'success', title: 'Vehicle registered', message: created.vehicle_number });
      setSelectedVehicleId(created.id);
      try {
        await loadVehicles();
      } catch (err) {
        pushToast({ kind: 'error', title: 'Could not reload vehicles', message: err.message });
      }
    },
    [loadVehicles, pushToast],
  );

  const handleLocationUpdated = useCallback(
    (result) => {
      refreshStatus(result.vehicle_id);
      setHistoryKey((key) => key + 1);
    },
    [refreshStatus],
  );

  const handlePickedChange = useCallback((change) => {
    setPicked((current) => ({ ...current, ...change }));
  }, []);

  const toggleDrawing = useCallback(() => setMode((current) => (current === 'draw' ? null : 'draw')), []);
  const togglePicking = useCallback(() => setMode((current) => (current === 'pick' ? null : 'pick')), []);
  const clearAlerts = useCallback(() => setAlerts([]), []);

  function changeTab(nextTab) {
    setTab(nextTab);
    setMode(null); // leaving a tab cancels drawing/picking
    if (nextTab === 'live') setUnread(0);
  }

  return (
    <div className="flex h-full flex-col">
      <header className="flex shrink-0 items-center justify-between gap-3 border-b border-slate-200 bg-white px-4 py-2.5">
        <div className="min-w-0">
          <h1 className="truncate text-base font-semibold">Geofence Alerts</h1>
          <p className="hidden truncate text-xs text-slate-500 sm:block">
            Geofencing, vehicle tracking and real-time alerts
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <ConnectionBadge status={socketStatus} />
          {session && (
            <>
              <span className="hidden max-w-[12rem] truncate text-xs text-slate-500 md:inline" title={session.email}>
                {session.email}
              </span>
              <button type="button" className="btn-secondary btn-small" onClick={onLogout}>
                Sign out
              </button>
            </>
          )}
        </div>
      </header>

      {loadError && (
        <div className="flex shrink-0 items-center justify-between gap-3 border-b border-red-200 bg-red-50 px-4 py-2 text-sm text-red-800">
          <span>{loadError}</span>
          <button type="button" className="btn-danger btn-small" onClick={loadAll}>
            Try again
          </button>
        </div>
      )}

      {/* Phone: map on top, panel below. Desktop (lg): panel on the left, map fills the rest. */}
      <main className="flex min-h-0 flex-1 flex-col lg:flex-row">
        <section className="h-[42vh] shrink-0 lg:order-2 lg:h-auto lg:flex-1" aria-label="Map">
          <MapView
            geofences={visibleGeofences}
            vehicles={vehicles}
            statuses={statuses}
            draft={draft}
            picked={picked}
            mode={mode}
            selectedVehicleId={selectedVehicleId}
            focus={focus}
            onMapClick={handleMapClick}
          />
        </section>

        <aside className="flex min-h-0 flex-1 flex-col border-t border-slate-200 bg-slate-50 lg:order-1 lg:w-[460px] lg:flex-none lg:border-r lg:border-t-0">
          <nav className="flex shrink-0 overflow-x-auto border-b border-slate-200 bg-white" aria-label="Sections">
            {TABS.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => changeTab(item.id)}
                aria-current={tab === item.id ? 'page' : undefined}
                className={`flex flex-1 items-center justify-center gap-1.5 whitespace-nowrap border-b-2 px-2 py-2.5 text-sm font-medium ${
                  tab === item.id
                    ? 'border-slate-900 text-slate-900'
                    : 'border-transparent text-slate-500 hover:text-slate-800'
                }`}
              >
                {item.label}
                {item.id === 'live' && unread > 0 && (
                  <span className="rounded-full bg-red-600 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white">
                    {unread > 99 ? '99+' : unread}
                  </span>
                )}
              </button>
            ))}
          </nav>

          <div className="min-h-0 flex-1 overflow-y-auto p-3">
            {tab === 'geofences' && (
              <GeofencePanel
                geofences={visibleGeofences}
                loading={loading}
                categoryFilter={categoryFilter}
                onCategoryFilterChange={setCategoryFilter}
                draft={draft}
                onDraftChange={setDraft}
                drawing={mode === 'draw'}
                onToggleDrawing={toggleDrawing}
                onCreated={handleGeofenceCreated}
                onFocus={focusGeofence}
              />
            )}
            {tab === 'vehicles' && (
              <VehiclePanel
                vehicles={vehicles}
                statuses={statuses}
                loading={loading}
                selectedVehicleId={selectedVehicleId}
                onSelectVehicle={setSelectedVehicleId}
                picked={picked}
                onPickedChange={handlePickedChange}
                picking={mode === 'pick'}
                onTogglePicking={togglePicking}
                onCreated={handleVehicleCreated}
                onLocationUpdated={handleLocationUpdated}
                onFocus={focusVehicle}
              />
            )}
            {tab === 'rules' && <AlertRulesPanel geofences={geofences} vehicles={vehicles} onNotify={pushToast} />}
            {tab === 'live' && (
              <AlertFeed alerts={alerts} socketStatus={socketStatus} onClear={clearAlerts} onShowOnMap={showPointOnMap} />
            )}
            {tab === 'history' && (
              <HistoryPanel
                geofences={geofences}
                vehicles={vehicles}
                refreshKey={historyKey}
                onShowOnMap={showPointOnMap}
              />
            )}
          </div>
        </aside>
      </main>

      <Toasts toasts={toasts} onDismiss={dismissToast} />
    </div>
  );
}

function ConnectionBadge({ status }) {
  const styles = {
    open: { dot: 'bg-emerald-500', text: 'Live', box: 'border-emerald-200 bg-emerald-50 text-emerald-800' },
    connecting: { dot: 'bg-amber-500', text: 'Connecting…', box: 'border-amber-200 bg-amber-50 text-amber-800' },
    closed: { dot: 'bg-red-500', text: 'Offline, retrying…', box: 'border-red-200 bg-red-50 text-red-800' },
  };
  const style = styles[status] || styles.connecting;
  return (
    <span
      className={`inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium ${style.box}`}
      title="Status of the real-time alert connection"
    >
      <span className={`h-2 w-2 rounded-full ${style.dot}`} />
      {style.text}
    </span>
  );
}
