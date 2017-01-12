package types

// Internal types for the roomserver.
// These should not become part of the API.

type StateAtEvent struct {
	// The state before the event.
	BeforeStateNID int64
	// The state entry for the event itself.
	StateEntry
}

func (a StateAtEvent) LessThan(b StateAtEvent) bool {
	if a.BeforeStateNID != b.BeforeStateNID {
		return a.BeforeStateNID < b.BeforeStateNID
	}
	return a.StateEntry.LessThan(b.StateEntry)
}

type StateEntry struct {
	StateKey
	EventNID int64
}

func (a StateEntry) LessThan(b StateEntry) bool {
	if a.StateKey != b.StateKey {
		return a.StateKey.LessThan(b.StateKey)
	}
	return a.EventNID < b.EventNID
}

type StateKey struct {
	EventTypeNID     int64
	EventStateKeyNID int64
}

func (a StateKey) LessThan(b StateKey) bool {
	if a.EventTypeNID != b.EventTypeNID {
		return a.EventTypeNID < b.EventTypeNID
	}
	return a.EventStateKeyNID < b.EventStateKeyNID
}

type StateDataNIDList struct {
	StateNID      int64
	StateDataNIDs []int64
}

type StateEntryList struct {
	StateDataNID int64
	StateEntries []StateEntry
}

type IDPair struct {
	ID  string
	NID int64
}

type EventJSON struct {
	EventNID  int64
	EventJSON []byte
}
