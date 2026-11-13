package api

import (
	"errors"
	"net/http"
	"strings"

	"geofence-alerts/internal/geo"
	"geofence-alerts/internal/store"
)

type createGeofenceRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"` // [latitude, longitude] pairs
	Category    string      `json:"category"`
}

// POST /geofences
func (a *API) createGeofence(r *http.Request) (int, Response) {
	var req createGeofenceRequest
	if err := decodeJSON(r, &req); err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	if req.Name == "" {
		return fail(http.StatusBadRequest, "name is required")
	}
	if req.Category == "" {
		return fail(http.StatusBadRequest, "category is required")
	}
	if !geo.IsValidCategory(req.Category) {
		return fail(http.StatusBadRequest, "category must be one of: "+strings.Join(geo.Categories, ", "))
	}
	if req.Coordinates == nil {
		return fail(http.StatusBadRequest, "coordinates is required")
	}
	if err := geo.ValidatePolygon(req.Coordinates); err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}

	id, status, err := a.store.CreateGeofence(r.Context(), req.Name, req.Description, req.Category, req.Coordinates)
	if errors.Is(err, store.ErrInvalidPolygon) {
		return fail(http.StatusBadRequest, "coordinates do not form a valid polygon: edges must not cross each other and the shape must enclose an area")
	}
	if err != nil {
		return serverError(r, err)
	}

	return http.StatusCreated, Response{
		"id":     id,
		"name":   req.Name,
		"status": status,
	}
}

// GET /geofences?category=delivery_zone
func (a *API) listGeofences(r *http.Request) (int, Response) {
	category := r.URL.Query().Get("category")
	if category != "" && !geo.IsValidCategory(category) {
		return fail(http.StatusBadRequest, "category must be one of: "+strings.Join(geo.Categories, ", "))
	}
	page, err := parsePage(r)
	if err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}

	geofences, err := a.store.ListGeofences(r.Context(), category, page)
	if err != nil {
		return serverError(r, err)
	}
	return http.StatusOK, Response{"geofences": geofences}
}
