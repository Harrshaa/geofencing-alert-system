package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"geofence-alerts/internal/geo"
	"geofence-alerts/internal/models"
)

// CreateGeofence stores a polygon and returns its new ID and status.
// coords are [latitude, longitude] pairs, already validated by geo.ValidatePolygon.
func (s *Store) CreateGeofence(ctx context.Context, name, description, category string, coords [][]float64) (id, status string, err error) {
	// ST_IsValid rejects shapes PostGIS cannot reason about, such as a
	// polygon whose edges cross each other or that has no area. When it is
	// false the INSERT adds no row, which we report as ErrInvalidPolygon.
	const query = `
		WITH input AS (
			SELECT ST_GeomFromText($4, 4326) AS geom
		)
		INSERT INTO geofences (name, description, category, boundary)
		SELECT $1::text, $2::text, $3::text, geom
		FROM input
		WHERE ST_IsValid(geom)
		RETURNING id, status`

	err = s.pool.QueryRow(ctx, query, name, description, category, geo.ToWKT(coords)).Scan(&id, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrInvalidPolygon
	}
	return id, status, err
}

// ListGeofences returns geofences, newest first. An empty category means all.
func (s *Store) ListGeofences(ctx context.Context, category string, page Page) ([]models.Geofence, error) {
	// LIMIT NULL means "no limit" in PostgreSQL, so NULLIF(0) turns our
	// "0 = no limit" convention into that.
	const query = `
		SELECT id, name, description, category, ST_AsGeoJSON(boundary), created_at
		FROM geofences
		WHERE status = 'active' AND ($1 = '' OR category = $1)
		ORDER BY created_at DESC, id DESC
		LIMIT NULLIF($2::int, 0) OFFSET $3::int`

	rows, err := s.pool.Query(ctx, query, category, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	geofences := []models.Geofence{} // not nil, so JSON shows [] instead of null
	for rows.Next() {
		var g models.Geofence
		var boundaryJSON []byte
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Category, &boundaryJSON, &g.CreatedAt); err != nil {
			return nil, err
		}
		if g.Coordinates, err = geo.FromGeoJSON(boundaryJSON); err != nil {
			return nil, err
		}
		g.CreatedAt = g.CreatedAt.UTC()
		geofences = append(geofences, g)
	}
	return geofences, rows.Err()
}
