# Setup Guide

How to run, test and use the Geofencing & Real-time Alert System.

## 1. Prerequisites

| To do this | You need |
| --- | --- |
| Run everything with Docker (recommended) | Docker Desktop with Docker Compose v2 |
| Run the backend without Docker | Go 1.27+, PostgreSQL 16 with PostGIS 3 |
| Run the frontend without Docker | Node.js 20+ and npm |

Ports used: `3000` (frontend), `8080` (backend), `5433` (database). The database is published on 5433, not 5432, so it does not clash with a PostgreSQL already running on your machine.

## 2. Run with Docker Compose

```bash
git clone <this-repo-url>
cd <repo-folder>
docker compose up --build
```

The first build takes a few minutes. When it finishes:

| Service | Address |
| --- | --- |
| Frontend | http://localhost:3000 |
| Backend API | http://localhost:8080 |
| Health check | http://localhost:8080/health |
| WebSocket | ws://localhost:8080/ws/alerts |
| Database | `postgres://geofence:geofence@localhost:5433/geofence` |

The backend creates its tables and indexes on startup, so there is no separate migration step.

```bash
docker compose down        # stop, keep the data
docker compose down -v     # stop and delete the database volume
```

## 3. Run without Docker (development)

**Database.** Easiest is to run only the database container:

```bash
docker compose up -d db
```

Or use your own PostgreSQL with PostGIS installed, and set `DATABASE_URL` to it.

**Backend.**

```bash
cd backend
go run .
```

**Frontend.**

```bash
cd frontend
npm install
cp .env.example .env     # only needed if the backend is not on http://localhost:8080
npm start                # or: npx parcel index.html
```

The dev server runs at http://localhost:1234.

## 4. Configuration

**Backend** (environment variables):

| Variable | Default | Meaning |
| --- | --- | --- |
| `PORT` | `8080` | HTTP port |
| `DATABASE_URL` | `postgres://geofence:geofence@localhost:5433/geofence?sslmode=disable` | PostgreSQL connection string |
| `CORS_ALLOWED_ORIGINS` | `*` | Comma-separated frontend origins allowed to call the API and open the WebSocket, e.g. `https://my-app.vercel.app` |
| `RATE_LIMIT_PER_SECOND` | `20` | Location updates allowed per second, per client IP. `0` disables the limit |
| `RATE_LIMIT_BURST` | `40` | Size of the burst a client may send before the limit applies |
| `AUTH_REQUIRED` | `false` | `true` requires a JWT on every API call and on the WebSocket. See [Authentication](#authentication-optional-jwt) |
| `JWT_SECRET` | (none) | Key that signs tokens. **Required** when `AUTH_REQUIRED=true`; use a long random value |
| `JWT_TTL` | `12h` | How long a token stays valid, e.g. `30m`, `12h` |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | (none) | Login account created (or its password updated) on every start. Password: 8–72 characters |

**Frontend** (read at build time by Parcel):

| Variable | Default | Meaning |
| --- | --- | --- |
| `API_URL` | `http://localhost:8080` | Backend address as seen from the browser. The WebSocket address is derived from it (`http` → `ws`, `https` → `wss`) |

## 5. Tests

```bash
cd backend
go test -race ./...
```

The unit tests cover polygon validation, the latitude/longitude conversion, entry/exit detection, alert rule matching, the `time_ns` wrapper, pagination parsing, the rate limiter, and JWT signing and verification (expiry, tampering, wrong key, `alg: none`). `-race` turns on Go's race detector.

## 6. API testing guide

Every response, including errors, contains `time_ns`: the handler's execution time in nanoseconds, as a string. Errors look like `{"error": "message", "time_ns": "12345"}`.

The commands below can be run in order against a fresh database. They assume the IDs `geo_1` and `veh_1`.

### Create a geofence

```bash
curl -X POST http://localhost:8080/geofences \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Downtown Restricted Zone",
    "description": "No entry for delivery vehicles",
    "coordinates": [
      [37.7749, -122.4194],
      [37.7849, -122.4194],
      [37.7849, -122.4094],
      [37.7749, -122.4094],
      [37.7749, -122.4194]
    ],
    "category": "restricted_zone"
  }'
```

```json
{"id":"geo_1","name":"Downtown Restricted Zone","status":"active","time_ns":"4186791"}
```

An invalid polygon returns `400` with a reason:

```bash
curl -X POST http://localhost:8080/geofences \
  -H 'Content-Type: application/json' \
  -d '{"name":"Open","coordinates":[[0,0],[0,1],[1,1],[1,0]],"category":"toll_zone"}'
```

```json
{"error":"first and last coordinates must be identical (closed polygon)","time_ns":"33541"}
```

### List geofences

```bash
curl http://localhost:8080/geofences
curl 'http://localhost:8080/geofences?category=restricted_zone'
```

### Register a vehicle

```bash
curl -X POST http://localhost:8080/vehicles \
  -H 'Content-Type: application/json' \
  -d '{"vehicle_number":"KA-01-AB-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890"}'
```

```json
{"id":"veh_1","vehicle_number":"KA-01-AB-1234","status":"active","time_ns":"3089041"}
```

### List vehicles

```bash
curl http://localhost:8080/vehicles
```

### Configure an alert rule

`vehicle_id` is optional. Leave it out to cover all vehicles. `event_type` is `entry`, `exit` or `both`.

```bash
curl -X POST http://localhost:8080/alerts/configure \
  -H 'Content-Type: application/json' \
  -d '{"geofence_id":"geo_1","event_type":"both"}'
```

```json
{"alert_id":"alert_1","geofence_id":"geo_1","vehicle_id":null,"event_type":"both","status":"active","time_ns":"2187958"}
```

### List alert rules

```bash
curl http://localhost:8080/alerts
curl 'http://localhost:8080/alerts?geofence_id=geo_1&vehicle_id=veh_1'
```

### Listen for real-time alerts

Open http://localhost:3000 in a browser, or use a WebSocket client in a second terminal:

```bash
npx wscat -c ws://localhost:8080/ws/alerts
```

### Update a vehicle location

Move the vehicle **into** the geofence. This records an entry and pushes an alert:

```bash
curl -X POST http://localhost:8080/vehicles/location \
  -H 'Content-Type: application/json' \
  -d '{"vehicle_id":"veh_1","latitude":37.7799,"longitude":-122.4144,"timestamp":"2025-01-15T10:35:00Z"}'
```

```json
{"vehicle_id":"veh_1","location_updated":true,"current_geofences":[{"geofence_id":"geo_1","geofence_name":"Downtown Restricted Zone","status":"inside"}],"time_ns":"4997750"}
```

Move it **out** again. This records an exit and pushes a second alert:

```bash
curl -X POST http://localhost:8080/vehicles/location \
  -H 'Content-Type: application/json' \
  -d '{"vehicle_id":"veh_1","latitude":37.70,"longitude":-122.50,"timestamp":"2025-01-15T10:40:00Z"}'
```

The WebSocket client receives one message per alert:

```json
{
  "event_id": "evt_1",
  "event_type": "entry",
  "timestamp": "2025-01-15T10:35:00Z",
  "vehicle": {"vehicle_id": "veh_1", "vehicle_number": "KA-01-AB-1234", "driver_name": "John Doe"},
  "geofence": {"geofence_id": "geo_1", "geofence_name": "Downtown Restricted Zone", "category": "restricted_zone"},
  "location": {"latitude": 37.7799, "longitude": -122.4144}
}
```

### Get a vehicle's current location

```bash
curl http://localhost:8080/vehicles/location/veh_1
```

### Violation history

```bash
curl http://localhost:8080/violations/history
curl 'http://localhost:8080/violations/history?vehicle_id=veh_1&limit=100'
curl 'http://localhost:8080/violations/history?geofence_id=geo_1&start_date=2025-01-01T00:00:00Z&end_date=2025-12-31T23:59:59Z'
curl 'http://localhost:8080/violations/history?limit=10&offset=10'
```

`limit` defaults to 50 and is capped at 500. `total_count` is the number of events matching the filters, across all pages.

### Additions beyond the brief

The nine endpoints in the brief are unchanged. These were added:

| Addition | Why |
| --- | --- |
| `DELETE /alerts/{alert_id}` | Lets the frontend remove a rule ("manage existing alerts") |
| `GET /health` | Health check for the hosting platform |
| `offset` on `/violations/history`; optional `limit` and `offset` on `/geofences`, `/vehicles`, `/alerts` | Pagination |
| `429` on `POST /vehicles/location` | Rate limiting per client IP |
| `POST /auth/login`, `GET /auth/me`, `GET /auth/status` | Optional JWT authentication (below) |

### Status codes

| Code | When |
| --- | --- |
| `200` | Successful read, location update or delete |
| `201` | Geofence, vehicle or alert rule created |
| `400` | Invalid input (the `error` field says what is wrong) |
| `401` | Wrong login, or (only with `AUTH_REQUIRED=true`) a missing, invalid or expired token |
| `404` | Unknown vehicle, geofence, alert rule or route |
| `409` | Duplicate vehicle number or identical alert rule |
| `429` | Rate limit exceeded |
| `500` | Unexpected server error (details are logged, not returned) |

### Authentication (optional JWT)

Authentication is **off by default** (`AUTH_REQUIRED=false`), so every endpoint above works without a token, exactly as the brief documents. To turn it on locally:

```bash
AUTH_REQUIRED=true docker compose up -d
```

Docker Compose then creates the login `admin@example.com` / `admin12345` (change it with `ADMIN_EMAIL` and `ADMIN_PASSWORD`). The frontend shows a login page.

Log in to get a token:

```bash
curl -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"admin12345"}'
```

```json
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIi...","token_type":"Bearer","expires_at":"2026-11-14T02:05:17Z","user":{"id":"user_1","email":"admin@example.com"},"time_ns":"52699625"}
```

Send it on every request:

```bash
TOKEN=<token from the login response>
curl http://localhost:8080/geofences -H "Authorization: Bearer $TOKEN"
curl http://localhost:8080/auth/me -H "Authorization: Bearer $TOKEN"
```

Without it, protected endpoints answer `401`:

```json
{"error":"authentication required: send the header Authorization: Bearer <token>","time_ns":"2583"}
```

Browsers cannot add headers to a WebSocket, so the alert stream takes the token as a query parameter: `ws://localhost:8080/ws/alerts?token=<token>`.

| Endpoint | Purpose |
| --- | --- |
| `POST /auth/login` | Email and password in, signed JWT out. Rate limited (1 per second per IP, bursts of 5). Wrong email and wrong password get the same message |
| `GET /auth/me` | Returns the token's user and expiry (always needs a valid token) |
| `GET /auth/status` | `{"auth_required": true/false}`, so the frontend knows whether to show the login page |

How it works: the token is `header.payload.signature`. The signature is HMAC-SHA256 of the header and payload with `JWT_SECRET`, so a token cannot be changed or forged without the key. Only `HS256` is accepted. Passwords are stored as bcrypt hashes. `/health` stays open for the hosting platform.

## 7. Frontend usage guide

The screen is a map plus a side panel with five tabs. The badge in the top-right corner shows whether the real-time connection is live.

1. **Geofences.** Enter a name and category, press **Draw on map**, and click the map once per corner (at least three). The polygon is closed automatically. You can instead paste coordinates as JSON `[[lat, lng], ...]`. Press **Create geofence**. The list below can be filtered by category; **Show on map** zooms to a geofence.
2. **Vehicles.** Register a vehicle with the form. To move one, choose it under **Update vehicle location**, press **Pick on map**, click a position, and press **Update location**. Tick **Send as soon as I click the map** to send on every click, which is the quickest way to drive a vehicle in and out of a geofence. Each vehicle card shows its position and the geofences it is inside. A vehicle inside a restricted zone has a red marker.
3. **Rules.** Choose a geofence, a vehicle (or all vehicles) and the event type, then **Create rule**. Existing rules can be filtered and removed.
4. **Live.** Every alert received since the page loaded, newest first. Alerts also appear as pop-up notifications on any tab, and the tab shows an unread count.
5. **History.** Every recorded entry and exit, with filters for vehicle, geofence and date range, a page-size selector and previous/next paging.

An alert is only sent when a rule matches the event. Entries and exits without a matching rule are still recorded and appear under History.

To see the real-time behaviour, open the app in two browser windows and move a vehicle in one: both receive the alert.

## 8. Architecture overview

```
 Browser (React)                    Go backend                         PostgreSQL + PostGIS
┌──────────────────┐   REST    ┌──────────────────────────┐   SQL    ┌─────────────────────┐
│ Map + panels     │──────────▶│ api      (validation,    │─────────▶│ geofences (polygon, │
│                  │           │           time_ns)       │          │   GiST index)       │
│ useAlertSocket   │◀──────────│ store    (all SQL)       │          │ vehicles            │
└──────────────────┘ WebSocket │ geo      (pure logic)    │          │ vehicle_locations   │
                               │ realtime (hub)           │          │ vehicle_geofence_   │
                               │ middleware (CORS, rate   │          │   state             │
                               │           limit, logs)   │          │ alert_rules         │
                               └──────────────────────────┘          │ violations          │
                                                                     │ alert_events        │
                                                                     └─────────────────────┘
```

### Project structure

```
backend/
  main.go                  startup, wiring, graceful shutdown
  internal/
    config/                environment variables
    database/              connection pool, schema.sql (embedded)
    models/                shared data shapes (JSON field names)
    geo/                   validation, lat/lng conversion, entry/exit diff  (+ tests)
    store/                 all SQL; one file per resource
    api/                   HTTP handlers, routes, time_ns wrapper          (+ tests)
    middleware/            CORS, request log, rate limiter                 (+ tests)
    auth/                  JWT signing and checking, password hashing      (+ tests)
    realtime/              WebSocket hub
frontend/
  index.html
  src/
    App.js                 login gate: shows the login page or the dashboard
    Dashboard.js           shared state, layout, tabs
    auth.js                keeps the login session in localStorage
    api.js                 every backend call
    config.js              API/WebSocket URLs, categories
    hooks/                 useAlertSocket (reconnecting WebSocket), useToasts
    components/            MapView, the five panels, shared UI pieces
database/
  Dockerfile               PostgreSQL 16 + PostGIS
docker-compose.yml
```

### What happens on a location update

`POST /vehicles/location` runs one database transaction:

1. **Lock the vehicle row** (`SELECT ... FOR UPDATE`). Two updates for the same vehicle cannot interleave, so events are never duplicated or lost.
2. **Save the position** in `vehicle_locations` (history) and on the vehicle row (current position).
3. **Find the geofences covering the point** with `ST_Covers(boundary, point)`, served by the GiST index.
4. **Read the geofences the vehicle was in before** from `vehicle_geofence_state`.
5. **Diff the two sets**: in the new set only → entry; in the old set only → exit; in both → no event.
6. **Record each event** in `violations`. If an active alert rule matches, also write an `alert_events` row and build the alert message.
7. **Commit**, then hand the alert messages to the WebSocket hub and return the response.

### Design decisions

- **Coordinate order.** The API uses `[latitude, longitude]`; PostGIS uses `(longitude, latitude)`. The swap happens in exactly two functions, `geo.ToWKT` and `geo.FromGeoJSON`, both unit tested.
- **Boundary rule.** `ST_Covers` is used instead of `ST_Contains`, so a point exactly on a geofence boundary counts as inside.
- **Entry and exit are transitions.** An event fires only when the set of geofences containing the vehicle changes. A vehicle's first update counts as an entry into every geofence it starts inside.
- **Alerts never slow the API.** The handler puts the alert on a buffered channel and returns. A single hub goroutine owns the client list and fans the message out; a slow or dead client is dropped instead of blocking the others.
- **One alert per event.** If several rules match the same event, clients get one alert, not one per rule.
- **`time_ns` in one place.** Handlers return `(status, body)` and a single wrapper adds `time_ns`, so no handler, including error paths, can omit it.
- **IDs** are a prefix plus a database sequence (`geo_1`, `veh_1`, `alert_1`, `viol_1`, `evt_1`).

## 9. Deployment

**Backend image on Docker Hub**

```bash
docker login
docker build -t <dockerhub-username>/geofence-alerts-backend:latest ./backend
docker push <dockerhub-username>/geofence-alerts-backend:latest
```

On an Apple Silicon Mac, build for the server's CPU: add `--platform linux/amd64` to `docker build`.

**Backend and database (Render)**

1. Create a PostgreSQL database. PostGIS is available as an extension and the backend enables it on startup (`CREATE EXTENSION IF NOT EXISTS postgis`).
2. Create a Web Service from the Docker image (or from the repo with root directory `backend`).
3. Set `DATABASE_URL` to the database's internal connection string and `CORS_ALLOWED_ORIGINS` to the frontend URL. The platform provides `PORT`. Set `JWT_SECRET` to a long random value (e.g. from `openssl rand -hex 32`) and `ADMIN_EMAIL`/`ADMIN_PASSWORD` if you want login to work; leave `AUTH_REQUIRED` unset (false) so the endpoints match the brief.
4. Set the health check path to `/health`.

**Frontend (Vercel)**

1. Import the repo and set the root directory to `frontend`. `vercel.json` already sets the build command and output folder.
2. Add the environment variable `API_URL` with the backend's `https://` address, then deploy. Because `API_URL` is compiled into the bundle, redeploy after changing it.
