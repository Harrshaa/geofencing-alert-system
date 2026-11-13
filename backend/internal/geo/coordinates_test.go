package geo

import (
	"reflect"
	"strings"
	"testing"
)

// A valid square in San Francisco, as [latitude, longitude] pairs.
var square = [][]float64{
	{37.7749, -122.4194},
	{37.7849, -122.4194},
	{37.7849, -122.4094},
	{37.7749, -122.4094},
	{37.7749, -122.4194},
}

func TestValidatePolygon(t *testing.T) {
	tests := []struct {
		name    string
		coords  [][]float64
		wantErr string // substring of the expected error; "" means valid
	}{
		{"valid square", square, ""},
		{"valid triangle", [][]float64{{0, 0}, {0, 1}, {1, 0}, {0, 0}}, ""},
		{"empty", nil, "at least 4 points"},
		{"too few points", [][]float64{{0, 0}, {0, 1}, {0, 0}}, "at least 4 points"},
		{"not closed", [][]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}}, "closed polygon"},
		{"point with one number", [][]float64{{0, 0}, {0}, {1, 1}, {0, 0}}, "[latitude, longitude] pair"},
		{"point with three numbers", [][]float64{{0, 0}, {0, 1, 2}, {1, 1}, {0, 0}}, "[latitude, longitude] pair"},
		{"latitude too high", [][]float64{{91, 0}, {0, 1}, {1, 1}, {91, 0}}, "latitude must be between"},
		{"latitude too low", [][]float64{{-90.5, 0}, {0, 1}, {1, 1}, {-90.5, 0}}, "latitude must be between"},
		{"longitude too high", [][]float64{{0, 181}, {0, 1}, {1, 1}, {0, 181}}, "longitude must be between"},
		{"lat/lng swapped", [][]float64{{-122.4, 37.7}, {-122.4, 37.8}, {-122.3, 37.8}, {-122.4, 37.7}}, "latitude must be between"},
		{"only two unique points", [][]float64{{0, 0}, {1, 1}, {0, 0}, {0, 0}}, "3 unique points"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolygon(tt.coords)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidateLatLng(t *testing.T) {
	valid := [][2]float64{{0, 0}, {90, 180}, {-90, -180}, {37.7749, -122.4194}}
	for _, p := range valid {
		if err := ValidateLatLng(p[0], p[1]); err != nil {
			t.Errorf("ValidateLatLng(%v, %v) should be valid, got %v", p[0], p[1], err)
		}
	}
	invalid := [][2]float64{{90.0001, 0}, {-91, 0}, {0, 180.0001}, {0, -181}}
	for _, p := range invalid {
		if err := ValidateLatLng(p[0], p[1]); err == nil {
			t.Errorf("ValidateLatLng(%v, %v) should be invalid", p[0], p[1])
		}
	}
}

func TestIsValidCategory(t *testing.T) {
	for _, category := range []string{"delivery_zone", "restricted_zone", "toll_zone", "customer_area"} {
		if !IsValidCategory(category) {
			t.Errorf("%q should be a valid category", category)
		}
	}
	for _, category := range []string{"", "Delivery_Zone", "parking"} {
		if IsValidCategory(category) {
			t.Errorf("%q should not be a valid category", category)
		}
	}
}

// The API uses [latitude, longitude]; PostGIS wants "longitude latitude".
func TestToWKTSwapsToLongitudeFirst(t *testing.T) {
	got := ToWKT([][]float64{{37.5, -122.25}, {38, -122.25}, {38, -122}, {37.5, -122.25}})
	want := "POLYGON((-122.25 37.5, -122.25 38, -122 38, -122.25 37.5))"
	if got != want {
		t.Fatalf("ToWKT:\n got  %s\n want %s", got, want)
	}
}

func TestFromGeoJSONSwapsBackToLatitudeFirst(t *testing.T) {
	raw := []byte(`{"type":"Polygon","coordinates":[[[-122.25,37.5],[-122.25,38],[-122,38],[-122.25,37.5]]]}`)
	got, err := FromGeoJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]float64{{37.5, -122.25}, {38, -122.25}, {38, -122}, {37.5, -122.25}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FromGeoJSON:\n got  %v\n want %v", got, want)
	}
}

func TestFromGeoJSONRejectsBadInput(t *testing.T) {
	for _, raw := range []string{`not json`, `{"type":"Polygon","coordinates":[]}`} {
		if _, err := FromGeoJSON([]byte(raw)); err == nil {
			t.Errorf("FromGeoJSON(%q) should fail", raw)
		}
	}
}
