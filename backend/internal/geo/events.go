package geo

// Event types for a vehicle crossing a geofence boundary.
const (
	EventEntry = "entry"
	EventExit  = "exit"
	EventBoth  = "both" // only used by alert rules
)

// DiffGeofences is the entry/exit detection.
//
// previous = IDs of the geofences the vehicle was inside before this update.
// current  = IDs of the geofences that contain the new position.
//
// A geofence in current but not in previous is an ENTRY.
// A geofence in previous but not in current is an EXIT.
// A geofence in both means the vehicle stayed inside: no event.
//
// On a vehicle's first ever update, previous is empty, so every geofence it
// starts inside counts as an entry.
func DiffGeofences(previous, current []string) (entered, exited []string) {
	wasInside := make(map[string]bool, len(previous))
	for _, id := range previous {
		wasInside[id] = true
	}
	isInside := make(map[string]bool, len(current))
	for _, id := range current {
		isInside[id] = true
	}

	for _, id := range current {
		if !wasInside[id] {
			entered = append(entered, id)
		}
	}
	for _, id := range previous {
		if !isInside[id] {
			exited = append(exited, id)
		}
	}
	return entered, exited
}

// RuleMatchesEvent reports whether an alert rule's event type covers an event.
// A rule for "both" matches entry and exit.
func RuleMatchesEvent(ruleEventType, eventType string) bool {
	return ruleEventType == EventBoth || ruleEventType == eventType
}
