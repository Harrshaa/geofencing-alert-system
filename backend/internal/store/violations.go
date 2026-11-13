package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"geofence-alerts/internal/models"
)

// ViolationFilter holds the optional filters for the history query.
// Empty strings and nil times mean "do not filter on this".
type ViolationFilter struct {
	VehicleID  string
	GeofenceID string
	Start      *time.Time
	End        *time.Time
	Limit      int
	Offset     int
}

// ListViolations returns one page of entry/exit events, newest first, plus
// the total number of events that match the filters (for pagination).
func (s *Store) ListViolations(ctx context.Context, filter ViolationFilter) ([]models.Violation, int, error) {
	// Build the WHERE clause from only the filters that were given.
	// Values always go through $1, $2... placeholders, never into the SQL
	// text itself, so user input cannot change the query (no SQL injection).
	var conditions []string
	var args []any
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if filter.VehicleID != "" {
		add("vi.vehicle_id = $%d", filter.VehicleID)
	}
	if filter.GeofenceID != "" {
		add("vi.geofence_id = $%d", filter.GeofenceID)
	}
	if filter.Start != nil {
		add("vi.occurred_at >= $%d", *filter.Start)
	}
	if filter.End != nil {
		add("vi.occurred_at <= $%d", *filter.End)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM violations vi `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT vi.id, vi.vehicle_id, v.vehicle_number, vi.geofence_id, g.name,
		       vi.event_type, vi.latitude, vi.longitude, vi.occurred_at
		FROM violations vi
		JOIN vehicles v ON v.id = vi.vehicle_id
		JOIN geofences g ON g.id = vi.geofence_id
		` + where + fmt.Sprintf(`
		ORDER BY vi.occurred_at DESC, vi.created_at DESC
		LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)

	rows, err := s.pool.Query(ctx, query, append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	violations := []models.Violation{}
	for rows.Next() {
		var v models.Violation
		if err := rows.Scan(&v.ID, &v.VehicleID, &v.VehicleNumber, &v.GeofenceID, &v.GeofenceName,
			&v.EventType, &v.Latitude, &v.Longitude, &v.Timestamp); err != nil {
			return nil, 0, err
		}
		v.Timestamp = v.Timestamp.UTC()
		violations = append(violations, v)
	}
	return violations, total, rows.Err()
}
