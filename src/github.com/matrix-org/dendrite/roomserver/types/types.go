package types

// Internal types for the roomserver.
// These should not become part of the API.

type StateAtEvent struct {
	// The state before the event.
	BeforeStateID int64
	// The state entry for the event itself.
	StateEntry
}

type StateEntry struct {
	EventTypeNID     int64
	EventStateKeyNID int64
	EventNID         int64
}
