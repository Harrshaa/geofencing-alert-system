package api

import (
	"net/http"
	"strconv"
	"time"

	"geofence-alerts/internal/store"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 500
)

// GET /violations/history?vehicle_id=&geofence_id=&start_date=&end_date=&limit=&offset=
func (a *API) listViolations(r *http.Request) (int, Response) {
	query := r.URL.Query()

	filter := store.ViolationFilter{
		VehicleID:  query.Get("vehicle_id"),
		GeofenceID: query.Get("geofence_id"),
		Limit:      defaultHistoryLimit,
	}

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return fail(http.StatusBadRequest, "limit must be a positive whole number")
		}
		if limit > maxHistoryLimit {
			limit = maxHistoryLimit
		}
		filter.Limit = limit
	}
	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return fail(http.StatusBadRequest, "offset must be zero or a positive whole number")
		}
		filter.Offset = offset
	}

	if raw := query.Get("start_date"); raw != "" {
		start, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return fail(http.StatusBadRequest, "start_date must be ISO 8601, e.g. 2025-01-01T00:00:00Z")
		}
		filter.Start = &start
	}
	if raw := query.Get("end_date"); raw != "" {
		end, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return fail(http.StatusBadRequest, "end_date must be ISO 8601, e.g. 2025-01-31T23:59:59Z")
		}
		filter.End = &end
	}
	if filter.Start != nil && filter.End != nil && filter.Start.After(*filter.End) {
		return fail(http.StatusBadRequest, "start_date must not be after end_date")
	}

	violations, total, err := a.store.ListViolations(r.Context(), filter)
	if err != nil {
		return serverError(r, err)
	}

	return http.StatusOK, Response{
		"violations":  violations,
		"total_count": total,
	}
}
