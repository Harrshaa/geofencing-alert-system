package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"geofence-alerts/internal/geo"
	"geofence-alerts/internal/models"
)

// LocationResult is what a location update produced.
type LocationResult struct {
	Current []models.GeofenceRef  // geofences the vehicle is inside now
	Alerts  []models.AlertMessage // alerts to push to connected clients
}

// UpdateLocation is the core of the system. In ONE transaction it:
//
//  1. locks the vehicle row, so two updates for the same vehicle cannot
//     interleave and produce duplicate or missing events
//  2. saves the position (history table + current position on the vehicle)
//  3. asks PostGIS which geofences cover the new point
//  4. compares that with the geofences the vehicle was in before (entry/exit)
//  5. records each entry/exit, and builds an alert when a rule matches
//
// Because it is one transaction, a failure half-way leaves nothing behind.
// The alerts are only returned here; the caller sends them after the commit.
func (s *Store) UpdateLocation(ctx context.Context, vehicleID string, lat, lng float64, at time.Time) (*LocationResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) // does nothing if Commit already succeeded

	// 1. Lock the vehicle. FOR UPDATE makes a second concurrent update for
	//    this vehicle wait here until the first one commits.
	var vehicle models.AlertVehicle
	vehicle.VehicleID = vehicleID
	err = tx.QueryRow(ctx, `
		SELECT vehicle_number, driver_name
		FROM vehicles
		WHERE id = $1
		FOR UPDATE`, vehicleID).Scan(&vehicle.VehicleNumber, &vehicle.DriverName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVehicleNotFound
	}
	if err != nil {
		return nil, err
	}

	// 2. Save the position.
	if _, err = tx.Exec(ctx, `
		INSERT INTO vehicle_locations (vehicle_id, latitude, longitude, recorded_at)
		VALUES ($1, $2, $3, $4)`, vehicleID, lat, lng, at); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `
		UPDATE vehicles
		SET last_latitude = $2, last_longitude = $3, last_seen_at = $4
		WHERE id = $1`, vehicleID, lat, lng, at); err != nil {
		return nil, err
	}

	// 3. Which geofences cover the new point?
	//    ST_MakePoint takes (x, y) = (longitude, latitude).
	//    ST_Covers counts a point exactly on the boundary as inside.
	//    The GiST index on boundary keeps this fast with many geofences.
	rows, err := tx.Query(ctx, `
		SELECT id, name, category
		FROM geofences
		WHERE status = 'active'
		  AND ST_Covers(boundary, ST_SetSRID(ST_MakePoint($1, $2), 4326))
		ORDER BY name`, lng, lat)
	if err != nil {
		return nil, err
	}
	current, err := pgx.CollectRows(rows, scanGeofenceRef)
	if err != nil {
		return nil, err
	}

	// 4. Which geofences was the vehicle inside before this update?
	rows, err = tx.Query(ctx, `
		SELECT g.id, g.name, g.category
		FROM vehicle_geofence_state st
		JOIN geofences g ON g.id = st.geofence_id
		WHERE st.vehicle_id = $1`, vehicleID)
	if err != nil {
		return nil, err
	}
	previous, err := pgx.CollectRows(rows, scanGeofenceRef)
	if err != nil {
		return nil, err
	}

	entered, exited := geo.DiffGeofences(refIDs(previous), refIDs(current))

	// Look up name/category by ID for the events below.
	byID := make(map[string]models.GeofenceRef)
	for _, ref := range previous {
		byID[ref.ID] = ref
	}
	for _, ref := range current {
		byID[ref.ID] = ref
	}

	result := &LocationResult{Current: current}
	position := models.AlertLocation{Latitude: lat, Longitude: lng}

	// 5. Record entries and exits, and update the stored state.
	for _, geofenceID := range entered {
		if _, err = tx.Exec(ctx, `
			INSERT INTO vehicle_geofence_state (vehicle_id, geofence_id, entered_at)
			VALUES ($1, $2, $3)`, vehicleID, geofenceID, at); err != nil {
			return nil, err
		}
		alert, err := recordEvent(ctx, tx, geo.EventEntry, vehicle, byID[geofenceID], position, at)
		if err != nil {
			return nil, err
		}
		if alert != nil {
			result.Alerts = append(result.Alerts, *alert)
		}
	}
	for _, geofenceID := range exited {
		if _, err = tx.Exec(ctx, `
			DELETE FROM vehicle_geofence_state
			WHERE vehicle_id = $1 AND geofence_id = $2`, vehicleID, geofenceID); err != nil {
			return nil, err
		}
		alert, err := recordEvent(ctx, tx, geo.EventExit, vehicle, byID[geofenceID], position, at)
		if err != nil {
			return nil, err
		}
		if alert != nil {
			result.Alerts = append(result.Alerts, *alert)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// recordEvent saves one entry/exit event to the violations table. If an
// active alert rule matches, it also saves an alert_events row and returns
// the alert message to broadcast. It returns nil when no rule matches.
func recordEvent(ctx context.Context, tx pgx.Tx, eventType string, vehicle models.AlertVehicle, geofence models.GeofenceRef, position models.AlertLocation, at time.Time) (*models.AlertMessage, error) {
	var violationID string
	err := tx.QueryRow(ctx, `
		INSERT INTO violations (vehicle_id, geofence_id, event_type, latitude, longitude, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		vehicle.VehicleID, geofence.ID, eventType, position.Latitude, position.Longitude, at).Scan(&violationID)
	if err != nil {
		return nil, err
	}

	ruleID, err := matchingRule(ctx, tx, eventType, vehicle.VehicleID, geofence.ID)
	if err != nil || ruleID == "" {
		return nil, err
	}

	var eventID string
	err = tx.QueryRow(ctx, `
		INSERT INTO alert_events (violation_id, alert_rule_id)
		VALUES ($1, $2)
		RETURNING id`, violationID, ruleID).Scan(&eventID)
	if err != nil {
		return nil, err
	}

	return &models.AlertMessage{
		EventID:   eventID,
		EventType: eventType,
		Timestamp: at.UTC(),
		Vehicle:   vehicle,
		Geofence: models.AlertGeofence{
			GeofenceID:   geofence.ID,
			GeofenceName: geofence.Name,
			Category:     geofence.Category,
		},
		Location: position,
	}, nil
}

// matchingRule returns the ID of the first active alert rule that matches
// this event, or "" if none does. A rule matches when:
//   - it is for this geofence, and
//   - it is for this vehicle OR for all vehicles (vehicle_id IS NULL), and
//   - its event type is this event's type or "both".
//
// One event produces at most one alert, even if several rules match.
func matchingRule(ctx context.Context, tx pgx.Tx, eventType, vehicleID, geofenceID string) (string, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, event_type
		FROM alert_rules
		WHERE geofence_id = $1
		  AND (vehicle_id IS NULL OR vehicle_id = $2)
		  AND status = 'active'
		ORDER BY created_at, id`, geofenceID, vehicleID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	matched := ""
	for rows.Next() {
		var ruleID, ruleEventType string
		if err := rows.Scan(&ruleID, &ruleEventType); err != nil {
			return "", err
		}
		if matched == "" && geo.RuleMatchesEvent(ruleEventType, eventType) {
			matched = ruleID
		}
	}
	return matched, rows.Err()
}

func refIDs(refs []models.GeofenceRef) []string {
	ids := make([]string, len(refs))
	for i, ref := range refs {
		ids[i] = ref.ID
	}
	return ids
}
