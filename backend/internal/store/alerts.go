package store

import (
	"context"

	"geofence-alerts/internal/models"
)

// CreateAlertRule saves a rule. vehicleID nil means "all vehicles".
func (s *Store) CreateAlertRule(ctx context.Context, geofenceID string, vehicleID *string, eventType string) (id, status string, err error) {
	// Check the references first so the caller gets a clear 404
	// instead of a raw foreign-key error.
	var exists bool
	if err = s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM geofences WHERE id = $1)`, geofenceID).Scan(&exists); err != nil {
		return "", "", err
	}
	if !exists {
		return "", "", ErrGeofenceNotFound
	}
	if vehicleID != nil {
		if err = s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM vehicles WHERE id = $1)`, *vehicleID).Scan(&exists); err != nil {
			return "", "", err
		}
		if !exists {
			return "", "", ErrVehicleNotFound
		}
	}

	// Same idea as in CreateVehicle: check first so a rejected duplicate does
	// not use up an ID. The unique index is still the real guarantee.
	if err = s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM alert_rules
			WHERE geofence_id = $1
			  AND COALESCE(vehicle_id, '') = COALESCE($2, '')
			  AND event_type = $3
		)`, geofenceID, vehicleID, eventType).Scan(&exists); err != nil {
		return "", "", err
	}
	if exists {
		return "", "", ErrDuplicate
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO alert_rules (geofence_id, vehicle_id, event_type)
		VALUES ($1, $2, $3)
		RETURNING id, status`, geofenceID, vehicleID, eventType).Scan(&id, &status)
	if isUniqueViolation(err) {
		return "", "", ErrDuplicate
	}
	return id, status, err
}

// ListAlertRules returns rules, newest first. Empty filters mean "all".
func (s *Store) ListAlertRules(ctx context.Context, geofenceID, vehicleID string, page Page) ([]models.AlertRule, error) {
	// LEFT JOIN on vehicles because a rule for all vehicles has no vehicle row.
	const query = `
		SELECT r.id, r.geofence_id, g.name, r.vehicle_id, v.vehicle_number,
		       r.event_type, r.status, r.created_at
		FROM alert_rules r
		JOIN geofences g ON g.id = r.geofence_id
		LEFT JOIN vehicles v ON v.id = r.vehicle_id
		WHERE ($1 = '' OR r.geofence_id = $1)
		  AND ($2 = '' OR r.vehicle_id = $2)
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT NULLIF($3::int, 0) OFFSET $4::int`

	rows, err := s.pool.Query(ctx, query, geofenceID, vehicleID, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []models.AlertRule{}
	for rows.Next() {
		var r models.AlertRule
		if err := rows.Scan(&r.AlertID, &r.GeofenceID, &r.GeofenceName, &r.VehicleID, &r.VehicleNumber,
			&r.EventType, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.CreatedAt = r.CreatedAt.UTC()
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// DeleteAlertRule removes a rule. It returns ErrNotFound if the ID does not exist.
func (s *Store) DeleteAlertRule(ctx context.Context, alertID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, alertID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
