package api

import (
	"errors"
	"net/http"
	"strings"

	"geofence-alerts/internal/geo"
	"geofence-alerts/internal/store"
)

type configureAlertRequest struct {
	GeofenceID string  `json:"geofence_id"`
	VehicleID  *string `json:"vehicle_id"` // optional: missing, null or "" means all vehicles
	EventType  string  `json:"event_type"`
}

// POST /alerts/configure
func (a *API) configureAlert(r *http.Request) (int, Response) {
	var req configureAlertRequest
	if err := decodeJSON(r, &req); err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}

	req.GeofenceID = strings.TrimSpace(req.GeofenceID)
	if req.VehicleID != nil {
		trimmed := strings.TrimSpace(*req.VehicleID)
		if trimmed == "" {
			req.VehicleID = nil
		} else {
			req.VehicleID = &trimmed
		}
	}

	if req.GeofenceID == "" {
		return fail(http.StatusBadRequest, "geofence_id is required")
	}
	if req.EventType == "" {
		return fail(http.StatusBadRequest, "event_type is required")
	}
	if req.EventType != geo.EventEntry && req.EventType != geo.EventExit && req.EventType != geo.EventBoth {
		return fail(http.StatusBadRequest, "event_type must be one of: entry, exit, both")
	}

	id, status, err := a.store.CreateAlertRule(r.Context(), req.GeofenceID, req.VehicleID, req.EventType)
	switch {
	case errors.Is(err, store.ErrGeofenceNotFound):
		return fail(http.StatusNotFound, "geofence "+req.GeofenceID+" not found")
	case errors.Is(err, store.ErrVehicleNotFound):
		return fail(http.StatusNotFound, "vehicle "+*req.VehicleID+" not found")
	case errors.Is(err, store.ErrDuplicate):
		return fail(http.StatusConflict, "an identical alert rule already exists")
	case err != nil:
		return serverError(r, err)
	}

	return http.StatusCreated, Response{
		"alert_id":    id,
		"geofence_id": req.GeofenceID,
		"vehicle_id":  req.VehicleID, // null when the rule covers all vehicles
		"event_type":  req.EventType,
		"status":      status,
	}
}

// GET /alerts?geofence_id=geo_1&vehicle_id=veh_1
func (a *API) listAlerts(r *http.Request) (int, Response) {
	page, err := parsePage(r)
	if err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}

	query := r.URL.Query()
	rules, err := a.store.ListAlertRules(r.Context(), query.Get("geofence_id"), query.Get("vehicle_id"), page)
	if err != nil {
		return serverError(r, err)
	}
	return http.StatusOK, Response{"alerts": rules}
}

// DELETE /alerts/{alert_id}
func (a *API) deleteAlert(r *http.Request) (int, Response) {
	alertID := r.PathValue("alert_id")

	err := a.store.DeleteAlertRule(r.Context(), alertID)
	if errors.Is(err, store.ErrNotFound) {
		return fail(http.StatusNotFound, "alert "+alertID+" not found")
	}
	if err != nil {
		return serverError(r, err)
	}
	return http.StatusOK, Response{"alert_id": alertID, "status": "deleted"}
}
