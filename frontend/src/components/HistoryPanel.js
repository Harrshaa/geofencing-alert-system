import React, { useEffect, useState } from 'react';
import { api } from '../api';
import { formatCoord, formatDateTime, localInputToISO } from '../utils';
import { EmptyState, ErrorNote, EventChip, Field, Spinner } from './ui';

const PAGE_SIZES = [10, 25, 50, 100];
const EMPTY_FILTERS = { vehicle_id: '', geofence_id: '', start: '', end: '' };

// Every entry/exit event ever recorded, with filters and pagination.
//
// Pagination is done by the server: we ask for one page (limit + offset) and
// the server also returns total_count, so we know how many pages exist
// without downloading all the rows.
function HistoryPanel({ geofences, vehicles, refreshKey, onShowOnMap }) {
  const [filters, setFilters] = useState(EMPTY_FILTERS);
  const [pageSize, setPageSize] = useState(10);
  const [page, setPage] = useState(0); // 0 = first page

  const [rows, setRows] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [manualRefresh, setManualRefresh] = useState(0);

  useEffect(() => {
    // If the filters change while a request is still in flight, its answer
    // is out of date. `cancelled` makes us ignore it (avoids a race where an
    // old, slow response overwrites a newer one).
    let cancelled = false;

    async function load() {
      setLoading(true);
      setError('');
      try {
        const data = await api.listViolations({
          vehicle_id: filters.vehicle_id,
          geofence_id: filters.geofence_id,
          start_date: localInputToISO(filters.start),
          end_date: localInputToISO(filters.end),
          limit: pageSize,
          offset: page * pageSize,
        });
        if (cancelled) return;
        setRows(data.violations);
        setTotal(data.total_count);
      } catch (err) {
        if (!cancelled) setError(err.message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, [filters, pageSize, page, refreshKey, manualRefresh]);

  // Changing a filter or the page size goes back to the first page.
  function updateFilter(event) {
    const { name, value } = event.target;
    setFilters((current) => ({ ...current, [name]: value }));
    setPage(0);
  }

  function changePageSize(event) {
    setPageSize(Number(event.target.value));
    setPage(0);
  }

  function resetFilters() {
    setFilters(EMPTY_FILTERS);
    setPage(0);
  }

  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const firstRow = total === 0 ? 0 : page * pageSize + 1;
  const lastRow = Math.min(total, (page + 1) * pageSize);
  const hasFilters = Object.values(filters).some(Boolean);

  return (
    <div className="space-y-3">
      <div className="card space-y-3">
        <div className="flex items-center justify-between gap-2">
          <h2 className="section-title">Violation history</h2>
          <div className="flex gap-1.5">
            <button type="button" className="btn-secondary btn-small" onClick={resetFilters} disabled={!hasFilters}>
              Reset
            </button>
            <button
              type="button"
              className="btn-secondary btn-small"
              onClick={() => setManualRefresh((n) => n + 1)}
              disabled={loading}
            >
              Refresh
            </button>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Vehicle">
            <select className="input" name="vehicle_id" value={filters.vehicle_id} onChange={updateFilter}>
              <option value="">All vehicles</option>
              {vehicles.map((vehicle) => (
                <option key={vehicle.id} value={vehicle.id}>
                  {vehicle.vehicle_number}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Geofence">
            <select className="input" name="geofence_id" value={filters.geofence_id} onChange={updateFilter}>
              <option value="">All geofences</option>
              {geofences.map((geofence) => (
                <option key={geofence.id} value={geofence.id}>
                  {geofence.name}
                </option>
              ))}
            </select>
          </Field>
          <Field label="From">
            <input className="input" type="datetime-local" name="start" value={filters.start} onChange={updateFilter} />
          </Field>
          <Field label="To">
            <input className="input" type="datetime-local" name="end" value={filters.end} onChange={updateFilter} />
          </Field>
        </div>
      </div>

      <ErrorNote message={error} />

      <div className="flex items-center justify-between gap-2 text-xs text-slate-600">
        <span>
          {loading ? <Spinner label="Loading…" /> : `Showing ${firstRow}–${lastRow} of ${total}`}
        </span>
        <label className="flex items-center gap-1.5">
          Per page
          <select className="input w-auto py-1" value={pageSize} onChange={changePageSize}>
            {PAGE_SIZES.map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </label>
      </div>

      {!loading && !error && rows.length === 0 && (
        <EmptyState>{hasFilters ? 'No events match these filters.' : 'No entry or exit events recorded yet.'}</EmptyState>
      )}

      <ul className={`space-y-2 ${loading ? 'opacity-60' : ''}`}>
        {rows.map((row) => (
          <HistoryRow key={row.id} row={row} onShowOnMap={onShowOnMap} />
        ))}
      </ul>

      {total > pageSize && (
        <div className="flex items-center justify-between gap-2">
          <button
            type="button"
            className="btn-secondary btn-small"
            onClick={() => setPage((p) => p - 1)}
            disabled={page === 0 || loading}
          >
            ← Previous
          </button>
          <span className="text-xs text-slate-600">
            Page {page + 1} of {pageCount}
          </span>
          <button
            type="button"
            className="btn-secondary btn-small"
            onClick={() => setPage((p) => p + 1)}
            disabled={page + 1 >= pageCount || loading}
          >
            Next →
          </button>
        </div>
      )}
    </div>
  );
}

const HistoryRow = React.memo(function HistoryRow({ row, onShowOnMap }) {
  return (
    <li className="card">
      <div className="flex items-start justify-between gap-2">
        <p className="min-w-0 text-sm">
          <span className="font-semibold">{row.vehicle_number}</span>{' '}
          {row.event_type === 'entry' ? 'entered' : 'left'}{' '}
          <span className="font-semibold">{row.geofence_name}</span>
        </p>
        <EventChip type={row.event_type} />
      </div>
      <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-slate-500">
        <span>{formatDateTime(row.timestamp)}</span>
        <button
          type="button"
          className="underline decoration-dotted hover:text-slate-800"
          onClick={() => onShowOnMap([row.latitude, row.longitude])}
        >
          {formatCoord(row.latitude)}, {formatCoord(row.longitude)}
        </button>
        <span>{row.id}</span>
      </div>
    </li>
  );
});

export default HistoryPanel;
