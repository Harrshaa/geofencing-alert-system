package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"geofence-alerts/internal/geo"
	"geofence-alerts/internal/store"
)

// Latitude and longitude are pointers so we can tell "field missing" (nil)
// apart from a real value of 0, which is a valid coordinate.
type updateLocationRequest struct {
	VehicleID string   `json:"vehicle_id"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Timestamp string   `json:"timestamp"`
}

// POST /vehicles/location
func (a *API) updateLocation(r *http.Request) (int, Response) {
	var req updateLocationRequest
	if err := decodeJSON(r, &req); err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}

	req.VehicleID = strings.TrimSpace(req.VehicleID)
	switch {
	case req.VehicleID == "":
		return fail(http.StatusBadRequest, "vehicle_id is required")
	case req.Latitude == nil:
		return fail(http.StatusBadRequest, "latitude is required")
	case req.Longitude == nil:
		return fail(http.StatusBadRequest, "longitude is required")
	case req.Timestamp == "":
		return fail(http.StatusBadRequest, "timestamp is required")
	}
	if err := geo.ValidateLatLng(*req.Latitude, *req.Longitude); err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}
	at, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		return fail(http.StatusBadRequest, "timestamp must be ISO 8601, e.g. 2025-01-15T10:35:00Z")
	}

	result, err := a.store.UpdateLocation(r.Context(), req.VehicleID, *req.Latitude, *req.Longitude, at)
	if errors.Is(err, store.ErrVehicleNotFound) {
		return fail(http.StatusNotFound, "vehicle "+req.VehicleID+" not found")
	}
	if err != nil {
		return serverError(r, err)
	}

	// The data is committed. Hand the alerts to the hub and move on:
	// Publish only drops them on a buffered channel, so this response is
	// never delayed by slow or disconnected WebSocket clients.
	for _, alert := range result.Alerts {
		a.hub.Publish(alert)
	}

	current := make([]Response, 0, len(result.Current))
	for _, geofence := range result.Current {
		current = append(current, Response{
			"geofence_id":   geofence.ID,
			"geofence_name": geofence.Name,
			"status":        "inside",
		})
	}

	return http.StatusOK, Response{
		"vehicle_id":        req.VehicleID,
		"location_updated":  true,
		"current_geofences": current,
	}
}

// GET /vehicles/location/{vehicle_id}
func (a *API) getVehicleLocation(r *http.Request) (int, Response) {
	vehicleID := r.PathValue("vehicle_id")

	location, err := a.store.GetVehicleLocation(r.Context(), vehicleID)
	if errors.Is(err, store.ErrVehicleNotFound) {
		return fail(http.StatusNotFound, "vehicle "+vehicleID+" not found")
	}
	if err != nil {
		return serverError(r, err)
	}

	current := make([]Response, 0, len(location.Geofences))
	for _, geofence := range location.Geofences {
		current = append(current, Response{
			"geofence_id":   geofence.ID,
			"geofence_name": geofence.Name,
			"category":      geofence.Category,
		})
	}

	// current_location is null until the vehicle sends its first update.
	var currentLocation any
	if location.Location != nil {
		currentLocation = location.Location
	}

	return http.StatusOK, Response{
		"vehicle_id":        location.VehicleID,
		"vehicle_number":    location.VehicleNumber,
		"current_location":  currentLocation,
		"current_geofences": current,
	}
}
