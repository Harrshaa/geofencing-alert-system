# Geofencing & Real-time Alert System

A full-stack system for defining geofences, tracking vehicles, and sending real-time alerts when a vehicle enters or leaves a geofence. Built for the MapUp full-stack developer assessment.

## Links

| | |
| --- | --- |
| Frontend (deployed) | https://geofencing-alert-system-topaz.vercel.app |
| Backend API (deployed) | https://geofence-alerts-api.onrender.com |
| Backend image (Docker Hub) | [`harrshaa/geofence-alerts-backend:latest`](https://hub.docker.com/r/harrshaa/geofence-alerts-backend) |
| Video walkthrough | _TODO: add link_ |

Setup, API examples and the architecture overview are in [SETUP.md](SETUP.md).

## Quick start

```bash
docker compose up --build
```

Then open http://localhost:3000. The API is at http://localhost:8080.

## Tech stack

| Layer | Choice |
| --- | --- |
| Backend | Go (standard library `net/http` router), `pgx` database driver |
| Database | PostgreSQL 16 + PostGIS, GiST spatial index |
| Real-time | WebSocket (`gorilla/websocket`) at `/ws/alerts` |
| Frontend | React 18 (JavaScript), Parcel, Tailwind CSS, Leaflet |
| Infrastructure | Docker (multi-stage builds), Docker Compose |

## What is implemented

**Backend**

- All nine endpoints from the brief, with the documented paths, methods and field names
- `time_ns` on every response, including errors
- Geofence validation: closed polygon, at least 4 points, coordinate ranges, allowed categories, and a PostGIS validity check that rejects self-crossing shapes
- Point-in-polygon with PostGIS `ST_Covers` and a GiST index
- Entry/exit detection by comparing the vehicle's previous and current set of geofences, inside one transaction with a row lock per vehicle
- Alert rules for one vehicle or all vehicles, for entry, exit or both
- WebSocket hub: multiple clients, ping/pong to detect dead connections, slow clients dropped, alert delivery that never blocks the HTTP response
- Every entry/exit stored in `violations`; every alert sent stored in `alert_events`
- History with vehicle, geofence and date filters, `limit`/`offset` and `total_count`

**Frontend**

- Interactive map: geofences as coloured polygons, vehicles as markers (red when inside a restricted zone)
- Create geofences by drawing on the map or pasting coordinates
- Register vehicles; update a location by clicking the map or typing coordinates
- Configure, filter and remove alert rules
- Live alerts as pop-up notifications and a feed, over a WebSocket that reconnects automatically
- Violation history with filters and pagination
- Loading states, clear error messages, and a layout that works on phones

**Bonus items**

- Rate limiting on `POST /vehicles/location` (token bucket per client IP)
- Pagination (`limit`/`offset`) on all list endpoints
- Unit tests for the critical functions, run with the race detector
- JWT authentication (HS256, bcrypt-hashed passwords, login page). It is **off by default** so the API matches the brief exactly; set `AUTH_REQUIRED=true` to turn it on. See [SETUP.md](SETUP.md#authentication-optional-jwt)

## Known gaps and what I would do next

| Gap | Plan |
| --- | --- |
| Authentication is basic: one admin account from settings, no sign-up or user management, no roles, no refresh tokens, and the token is kept in `localStorage` | Add user management and roles, short-lived access tokens with refresh tokens, and an httpOnly cookie instead of `localStorage` |
| Location updates are processed in arrival order | A late GPS point with an older timestamp moves the vehicle back. Compare the timestamp with the last one and store late points as history only |
| Geofences cannot be edited or deleted | Add `PUT`/`DELETE`, and emit exit events for vehicles inside a deleted geofence |
| A client that was offline misses alerts | Alerts are already stored in `alert_events`; let a client send the last `event_id` it saw on reconnect and replay the rest |
| One backend instance only | The hub and rate limiter live in memory. To run several instances, publish alerts through Redis pub/sub or PostgreSQL `LISTEN/NOTIFY` so every instance can reach its own clients, and move rate-limit counters to Redis |
| The map shows other vehicles' movements only when an alert fires | Broadcast position updates on a second WebSocket message type |
| `vehicle_locations` grows without limit | Partition by month and drop or archive old partitions |
| No integration tests against a real database | Add tests that start PostGIS in a container and exercise the store layer |
| Geometry is planar (degrees) | Correct for containment. Distance or radius features would need the `geography` type |

## Scaling to thousands of vehicles and geofences

- **Geofences.** The GiST index means a location update only tests polygons whose bounding box contains the point, so cost grows slowly with the number of geofences.
- **Vehicles.** Each update is one short transaction that locks only its own vehicle row, so different vehicles do not block each other. Next steps would be batching history inserts and caching the geofence set in memory for very high update rates.
- **Reads.** History queries use indexes on `(vehicle_id, occurred_at)`, `(geofence_id, occurred_at)` and `occurred_at`, and are always paginated.
- **Real-time.** One hub goroutine handles thousands of connections; beyond one instance, see the pub/sub note above.
