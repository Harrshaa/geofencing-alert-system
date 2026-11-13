// Package models holds the data shapes shared by the store and the API.
// The json tags match the field names in the assessment brief exactly.
package models

import "time"

type Geofence struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"` // [latitude, longitude] pairs
	Category    string      `json:"category"`
	CreatedAt   time.Time   `json:"created_at"`
}

type Vehicle struct {
	ID            string    `json:"id"`
	VehicleNumber string    `json:"vehicle_number"`
	DriverName    string    `json:"driver_name"`
	VehicleType   string    `json:"vehicle_type"`
	Phone         string    `json:"phone"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// GeofenceRef is the short form of a geofence used in location results.
type GeofenceRef struct {
	ID       string
	Name     string
	Category string
}

// Location is a vehicle position at a point in time.
type Location struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Timestamp time.Time `json:"timestamp"`
}

// AlertRule is a configured rule: "alert on <event_type> for <geofence>",
// for one vehicle or (when VehicleID is nil) for all vehicles.
type AlertRule struct {
	AlertID       string    `json:"alert_id"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name"`
	VehicleID     *string   `json:"vehicle_id"`
	VehicleNumber *string   `json:"vehicle_number"`
	EventType     string    `json:"event_type"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// Violation is one recorded entry or exit event.
type Violation struct {
	ID            string    `json:"id"`
	VehicleID     string    `json:"vehicle_id"`
	VehicleNumber string    `json:"vehicle_number"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name"`
	EventType     string    `json:"event_type"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Timestamp     time.Time `json:"timestamp"`
}

// AlertMessage is the real-time message pushed over the WebSocket.
type AlertMessage struct {
	EventID   string        `json:"event_id"`
	EventType string        `json:"event_type"`
	Timestamp time.Time     `json:"timestamp"`
	Vehicle   AlertVehicle  `json:"vehicle"`
	Geofence  AlertGeofence `json:"geofence"`
	Location  AlertLocation `json:"location"`
}

type AlertVehicle struct {
	VehicleID     string `json:"vehicle_id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
}

type AlertGeofence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

type AlertLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
