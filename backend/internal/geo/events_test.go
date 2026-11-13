package geo

import (
	"reflect"
	"testing"
)

func TestDiffGeofences(t *testing.T) {
	tests := []struct {
		name        string
		previous    []string
		current     []string
		wantEntered []string
		wantExited  []string
	}{
		{"first update, outside everything", nil, nil, nil, nil},
		{"first update, starts inside one", nil, []string{"geo_1"}, []string{"geo_1"}, nil},
		{"stays outside", nil, nil, nil, nil},
		{"stays inside: no event", []string{"geo_1"}, []string{"geo_1"}, nil, nil},
		{"enters", nil, []string{"geo_1"}, []string{"geo_1"}, nil},
		{"exits", []string{"geo_1"}, nil, nil, []string{"geo_1"}},
		{"moves from one geofence to another", []string{"geo_1"}, []string{"geo_2"}, []string{"geo_2"}, []string{"geo_1"}},
		{"overlapping: enters a second, stays in the first", []string{"geo_1"}, []string{"geo_1", "geo_2"}, []string{"geo_2"}, nil},
		{"overlapping: leaves one of two", []string{"geo_1", "geo_2"}, []string{"geo_2"}, nil, []string{"geo_1"}},
		{"order does not matter", []string{"geo_2", "geo_1"}, []string{"geo_1", "geo_2"}, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entered, exited := DiffGeofences(tt.previous, tt.current)
			if !reflect.DeepEqual(entered, tt.wantEntered) {
				t.Errorf("entered: got %v, want %v", entered, tt.wantEntered)
			}
			if !reflect.DeepEqual(exited, tt.wantExited) {
				t.Errorf("exited: got %v, want %v", exited, tt.wantExited)
			}
		})
	}
}

func TestRuleMatchesEvent(t *testing.T) {
	tests := []struct {
		rule, event string
		want        bool
	}{
		{"entry", "entry", true},
		{"entry", "exit", false},
		{"exit", "exit", true},
		{"exit", "entry", false},
		{"both", "entry", true},
		{"both", "exit", true},
	}
	for _, tt := range tests {
		if got := RuleMatchesEvent(tt.rule, tt.event); got != tt.want {
			t.Errorf("RuleMatchesEvent(%q, %q) = %v, want %v", tt.rule, tt.event, got, tt.want)
		}
	}
}
