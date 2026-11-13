package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"geofence-alerts/internal/models"
)

// CreateVehicle registers a vehicle. vehicle_number must be unique.
func (s *Store) CreateVehicle(ctx context.Context, number, driver, vehicleType, phone string) (id, status string, err error) {
	// Check for a duplicate first. A failed INSERT still uses up a sequence
	// number, which would leave a gap in the IDs (veh_1, veh_3, ...).
	// The UNIQUE constraint below remains the real guarantee: if two requests
	// race past this check, the database rejects the second one.
	var exists bool
	if err = s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM vehicles WHERE vehicle_number = $1)`, number).Scan(&exists); err != nil {
		return "", "", err
	}
	if exists {
		return "", "", ErrDuplicate
	}

	const query = `
		INSERT INTO vehicles (vehicle_number, driver_name, vehicle_type, phone)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status`

	err = s.pool.QueryRow(ctx, query, number, driver, vehicleType, phone).Scan(&id, &status)
	if isUniqueViolation(err) {
		return "", "", ErrDuplicate
	}
	return id, status, err
}

// ListVehicles returns all vehicles, newest first.
func (s *Store) ListVehicles(ctx context.Context, page Page) ([]models.Vehicle, error) {
	const query = `
		SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at
		FROM vehicles
		ORDER BY created_at DESC, id DESC
		LIMIT NULLIF($1::int, 0) OFFSET $2::int`

	rows, err := s.pool.Query(ctx, query, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vehicles := []models.Vehicle{}
	for rows.Next() {
		var v models.Vehicle
		if err := rows.Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.CreatedAt = v.CreatedAt.UTC()
		vehicles = append(vehicles, v)
	}
	return vehicles, rows.Err()
}

// VehicleLocation is a vehicle's current position and the geofences it is inside.
type VehicleLocation struct {
	VehicleID     string
	VehicleNumber string
	Location      *models.Location // nil until the first location update
	Geofences     []models.GeofenceRef
}

// GetVehicleLocation returns the current position and geofence status of one vehicle.
func (s *Store) GetVehicleLocation(ctx context.Context, vehicleID string) (*VehicleLocation, error) {
	result := &VehicleLocation{VehicleID: vehicleID}

	var lat, lng *float64
	var seenAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT vehicle_number, last_latitude, last_longitude, last_seen_at
		FROM vehicles
		WHERE id = $1`, vehicleID).Scan(&result.VehicleNumber, &lat, &lng, &seenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVehicleNotFound
	}
	if err != nil {
		return nil, err
	}

	if lat != nil && lng != nil && seenAt != nil {
		result.Location = &models.Location{Latitude: *lat, Longitude: *lng, Timestamp: seenAt.UTC()}
	}

	// Read the stored state instead of re-running the spatial check:
	// it is exactly what the last location update computed.
	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.name, g.category
		FROM vehicle_geofence_state st
		JOIN geofences g ON g.id = st.geofence_id
		WHERE st.vehicle_id = $1
		ORDER BY g.name`, vehicleID)
	if err != nil {
		return nil, err
	}
	result.Geofences, err = pgx.CollectRows(rows, scanGeofenceRef)
	return result, err
}

func scanGeofenceRef(row pgx.CollectableRow) (models.GeofenceRef, error) {
	var ref models.GeofenceRef
	err := row.Scan(&ref.ID, &ref.Name, &ref.Category)
	return ref, err
}
