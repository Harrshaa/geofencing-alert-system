package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"geofence-alerts/internal/store"
)

type createVehicleRequest struct {
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
}

// phonePattern accepts an optional leading +, then 7 to 15 digits, allowing
// spaces and dashes between them. Deliberately loose: formats vary by country.
var phonePattern = regexp.MustCompile(`^\+?[0-9][0-9 \-]{5,18}[0-9]$`)

// POST /vehicles
func (a *API) createVehicle(r *http.Request) (int, Response) {
	var req createVehicleRequest
	if err := decodeJSON(r, &req); err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}

	req.VehicleNumber = strings.TrimSpace(req.VehicleNumber)
	req.DriverName = strings.TrimSpace(req.DriverName)
	req.VehicleType = strings.TrimSpace(req.VehicleType)
	req.Phone = strings.TrimSpace(req.Phone)

	switch {
	case req.VehicleNumber == "":
		return fail(http.StatusBadRequest, "vehicle_number is required")
	case req.DriverName == "":
		return fail(http.StatusBadRequest, "driver_name is required")
	case req.VehicleType == "":
		return fail(http.StatusBadRequest, "vehicle_type is required")
	case req.Phone == "":
		return fail(http.StatusBadRequest, "phone is required")
	case !phonePattern.MatchString(req.Phone):
		return fail(http.StatusBadRequest, "phone must be a valid phone number, e.g. +1234567890")
	}

	id, status, err := a.store.CreateVehicle(r.Context(), req.VehicleNumber, req.DriverName, req.VehicleType, req.Phone)
	if errors.Is(err, store.ErrDuplicate) {
		return fail(http.StatusConflict, "a vehicle with vehicle_number "+req.VehicleNumber+" already exists")
	}
	if err != nil {
		return serverError(r, err)
	}

	return http.StatusCreated, Response{
		"id":             id,
		"vehicle_number": req.VehicleNumber,
		"status":         status,
	}
}

// GET /vehicles
func (a *API) listVehicles(r *http.Request) (int, Response) {
	page, err := parsePage(r)
	if err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}

	vehicles, err := a.store.ListVehicles(r.Context(), page)
	if err != nil {
		return serverError(r, err)
	}
	return http.StatusOK, Response{"vehicles": vehicles}
}
