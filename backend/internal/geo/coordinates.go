// Package geo holds the geofence rules that do not need a database:
// coordinate validation, the [lat, lng] <-> [lng, lat] conversion, and
// entry/exit detection.
package geo

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Categories is the fixed list of geofence categories from the brief.
var Categories = []string{"delivery_zone", "restricted_zone", "toll_zone", "customer_area"}

func IsValidCategory(category string) bool {
	for _, allowed := range Categories {
		if category == allowed {
			return true
		}
	}
	return false
}

// ValidateLatLng checks that a single point is on the globe.
func ValidateLatLng(lat, lng float64) error {
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		return errors.New("latitude must be between -90 and 90")
	}
	if math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		return errors.New("longitude must be between -180 and 180")
	}
	return nil
}

// ValidatePolygon checks the rules for a geofence boundary given as
// [latitude, longitude] pairs:
//   - at least 4 points (3 unique + 1 closing point)
//   - every point is a valid [latitude, longitude] pair
//   - the first and last points are identical (closed polygon)
//
// Shape problems such as crossing edges are checked later by PostGIS (ST_IsValid).
func ValidatePolygon(coords [][]float64) error {
	if len(coords) < 4 {
		return errors.New("coordinates must have at least 4 points (3 unique points plus the closing point)")
	}

	for i, point := range coords {
		if len(point) != 2 {
			return fmt.Errorf("coordinates[%d] must be a [latitude, longitude] pair", i)
		}
		if err := ValidateLatLng(point[0], point[1]); err != nil {
			return fmt.Errorf("coordinates[%d]: %w", i, err)
		}
	}

	first, last := coords[0], coords[len(coords)-1]
	if first[0] != last[0] || first[1] != last[1] {
		return errors.New("first and last coordinates must be identical (closed polygon)")
	}

	// Count distinct points, ignoring the closing point.
	unique := make(map[[2]float64]bool)
	for _, point := range coords[:len(coords)-1] {
		unique[[2]float64{point[0], point[1]}] = true
	}
	if len(unique) < 3 {
		return errors.New("coordinates must contain at least 3 unique points")
	}
	return nil
}

// ToWKT converts API coordinates ([latitude, longitude]) into the WKT text
// PostGIS reads, e.g. "POLYGON((lng lat, lng lat, ...))".
//
// This is the ONLY place where latitude/longitude are swapped on the way in.
// PostGIS expects x = longitude first, y = latitude second.
func ToWKT(coords [][]float64) string {
	points := make([]string, len(coords))
	for i, point := range coords {
		lat, lng := point[0], point[1]
		points[i] = formatFloat(lng) + " " + formatFloat(lat)
	}
	return "POLYGON((" + strings.Join(points, ", ") + "))"
}

// FromGeoJSON converts a PostGIS polygon (as GeoJSON text from ST_AsGeoJSON)
// back into API coordinates ([latitude, longitude]).
//
// This is the ONLY place where latitude/longitude are swapped on the way out.
func FromGeoJSON(raw []byte) ([][]float64, error) {
	var polygon struct {
		Coordinates [][][]float64 `json:"coordinates"` // rings -> points -> [lng, lat]
	}
	if err := json.Unmarshal(raw, &polygon); err != nil {
		return nil, fmt.Errorf("parse polygon: %w", err)
	}
	if len(polygon.Coordinates) == 0 {
		return nil, errors.New("polygon has no rings")
	}

	outerRing := polygon.Coordinates[0]
	coords := make([][]float64, len(outerRing))
	for i, point := range outerRing {
		if len(point) < 2 {
			return nil, errors.New("polygon point is not a pair")
		}
		lng, lat := point[0], point[1]
		coords[i] = []float64{lat, lng}
	}
	return coords, nil
}

// formatFloat prints a float with full precision and no exponent.
func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
