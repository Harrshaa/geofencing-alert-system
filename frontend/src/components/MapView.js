import React, { useEffect, useMemo } from 'react';
import L from 'leaflet';
import {
  CircleMarker,
  MapContainer,
  Marker,
  Polygon,
  Polyline,
  Popup,
  TileLayer,
  Tooltip,
  useMap,
  useMapEvents,
} from 'react-leaflet';
import 'leaflet/dist/leaflet.css';

import { CATEGORIES, DEFAULT_CENTER, DEFAULT_ZOOM, categoryInfo } from '../config';
import { formatCoord, formatDateTime, isValidPosition } from '../utils';

const VEHICLE_EMOJI = { truck: '🚚', van: '🚐', car: '🚗', motorcycle: '🏍️' };

// Leaflet's default marker images do not survive bundling, so vehicles use a
// divIcon: a plain HTML element styled in styles.css.
function vehicleIcon(vehicleType, { selected, restricted }) {
  const emoji = VEHICLE_EMOJI[String(vehicleType).toLowerCase()] || '📍';
  const classes = ['vehicle-pin', selected ? 'is-selected' : '', restricted ? 'is-restricted' : ''].join(' ');
  return L.divIcon({
    html: `<div class="${classes}">${emoji}</div>`,
    className: '', // remove Leaflet's default white square
    iconSize: [34, 34],
    iconAnchor: [17, 17],
    popupAnchor: [0, -18],
  });
}

const pickedIcon = L.divIcon({
  html: '<div class="picked-pin"></div>',
  className: '',
  iconSize: [18, 18],
  iconAnchor: [9, 9],
});

// Forwards map clicks to the parent. Renders nothing.
function ClickHandler({ onMapClick }) {
  useMapEvents({
    click: (event) => {
      // wrap() brings the longitude back into -180..180 if the user has
      // scrolled the map sideways past the edge of the world.
      const { lat, lng } = event.latlng.wrap();
      onMapClick(lat, lng);
    },
  });
  return null;
}

// Moves the map when `focus` changes. focus is either
// { bounds: [[lat, lng], ...] } to fit an area or { point: [lat, lng] }.
function FocusController({ focus }) {
  const map = useMap();
  useEffect(() => {
    if (!focus) return;
    if (focus.bounds && focus.bounds.length > 0) {
      map.fitBounds(focus.bounds, { padding: [40, 40], maxZoom: 16 });
    } else if (focus.point) {
      map.setView(focus.point, Math.max(map.getZoom(), 15));
    }
  }, [focus, map]);
  return null;
}

// The map container changes size when the layout does (window resize, phone
// rotation). Leaflet has to be told, or it leaves grey gaps.
function ResizeWatcher() {
  const map = useMap();
  useEffect(() => {
    const observer = new ResizeObserver(() => map.invalidateSize());
    observer.observe(map.getContainer());
    return () => observer.disconnect();
  }, [map]);
  return null;
}

// All geofence polygons. React.memo: this layer only re-renders when the
// geofence list itself changes, not on every map click or incoming alert.
const GeofenceLayer = React.memo(function GeofenceLayer({ geofences }) {
  return geofences.map((geofence) => {
    const { color, label } = categoryInfo(geofence.category);
    return (
      <Polygon
        key={geofence.id}
        positions={geofence.coordinates}
        pathOptions={{ color, weight: 2, fillColor: color, fillOpacity: 0.18 }}
      >
        <Tooltip sticky>
          <strong>{geofence.name}</strong>
          <br />
          {label}
        </Tooltip>
      </Polygon>
    );
  });
});

// One marker per vehicle that has reported a position.
const VehicleLayer = React.memo(function VehicleLayer({ vehicles, statuses, selectedVehicleId }) {
  return vehicles.map((vehicle) => {
    const status = statuses[vehicle.id];
    const location = status && status.current_location;
    if (!location) return null;

    const inside = status.current_geofences || [];
    const restricted = inside.some((g) => g.category === 'restricted_zone');

    return (
      <Marker
        key={vehicle.id}
        position={[location.latitude, location.longitude]}
        icon={vehicleIcon(vehicle.vehicle_type, { selected: vehicle.id === selectedVehicleId, restricted })}
        zIndexOffset={vehicle.id === selectedVehicleId ? 1000 : 0}
      >
        <Popup>
          <strong>{vehicle.vehicle_number}</strong>
          <br />
          {vehicle.driver_name} · {vehicle.vehicle_type}
          <br />
          {formatCoord(location.latitude)}, {formatCoord(location.longitude)}
          <br />
          {formatDateTime(location.timestamp)}
          <br />
          {inside.length > 0 ? `Inside: ${inside.map((g) => g.geofence_name).join(', ')}` : 'Not inside any geofence'}
        </Popup>
      </Marker>
    );
  });
});

// The polygon being drawn: a dot per corner, and a dashed outline.
const DraftLayer = React.memo(function DraftLayer({ points }) {
  if (points.length === 0) return null;
  const style = { color: '#0f172a', weight: 2, dashArray: '6 6' };
  return (
    <>
      {points.length >= 3 ? (
        <Polygon positions={points} pathOptions={{ ...style, fillColor: '#0f172a', fillOpacity: 0.1 }} />
      ) : (
        <Polyline positions={points} pathOptions={style} />
      )}
      {points.map((point, index) => (
        <CircleMarker
          // Corners have no ID, and they are only ever added or removed at the end.
          key={index}
          center={point}
          radius={5}
          pathOptions={{ color: '#0f172a', weight: 2, fillColor: '#ffffff', fillOpacity: 1 }}
        />
      ))}
    </>
  );
});

function Legend() {
  return (
    <div className="pointer-events-none absolute bottom-6 left-2 z-[1000] hidden rounded-md border border-slate-200 bg-white/95 px-2.5 py-2 text-xs shadow sm:block">
      {CATEGORIES.map((category) => (
        <div key={category.value} className="flex items-center gap-1.5 py-0.5">
          <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: category.color }} />
          {category.label}
        </div>
      ))}
    </div>
  );
}

const MODE_HINTS = {
  draw: 'Drawing a geofence: click the map to add corners.',
  pick: 'Click the map to set the vehicle position.',
};

function MapView({ geofences, vehicles, statuses, draft, picked, mode, selectedVehicleId, focus, onMapClick }) {
  // Only build the marker position when the typed values are a real position.
  const pickedPosition = useMemo(
    () => (isValidPosition(picked.lat, picked.lng) ? [Number(picked.lat), Number(picked.lng)] : null),
    [picked.lat, picked.lng],
  );

  return (
    <div className={`relative h-full w-full ${mode ? 'map-picking' : ''}`}>
      <MapContainer center={DEFAULT_CENTER} zoom={DEFAULT_ZOOM} scrollWheelZoom>
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <ClickHandler onMapClick={onMapClick} />
        <FocusController focus={focus} />
        <ResizeWatcher />

        <GeofenceLayer geofences={geofences} />
        <DraftLayer points={draft} />
        <VehicleLayer vehicles={vehicles} statuses={statuses} selectedVehicleId={selectedVehicleId} />
        {pickedPosition && <Marker position={pickedPosition} icon={pickedIcon} interactive={false} />}
      </MapContainer>

      {mode && (
        <div className="pointer-events-none absolute left-1/2 top-2 z-[1000] -translate-x-1/2 rounded-full bg-slate-900 px-3 py-1.5 text-center text-xs font-medium text-white shadow-lg">
          {MODE_HINTS[mode]}
        </div>
      )}
      <Legend />
    </div>
  );
}

export default React.memo(MapView);
