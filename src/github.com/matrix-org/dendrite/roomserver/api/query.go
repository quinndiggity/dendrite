package api

// A QueryEventExistsRequest queries whether the events exist in the database
type QueryEventExistsRequest struct {
	// List of event IDs to check if we already have.
	EventIDs []EventID
}

// A QueryEventExistsResponse returns whether we have copies of the events
type QueryEventExistsResponse struct {
	// Bitmap of whether the event exists in the database or not.
	Exists []byte
}

// A QueryEventJSONRequest requests a list of event JSON objects.
type QueryEventJSONRequest struct {
	// List of event IDs to return JSON for.
	EventIDs []EventID
}

// A QueryEventJSONResponse returns a list of event JSON objects
type QueryEventJSONResponse struct {
	// The JSON for each requested event.
	EventJSONs []EventJSON
}

// A QueryRoomStateRequest requests the current forward edges and current state
// for a room. This can be used to send an event into the room.
type QueryRoomStateRequest struct {
	// The ID of the room to return.
	RoomID string
	// Filter the state of the room to particular types and state keys.
	// If the list is empty then return all state types and keys.
	State []struct {
		// The type of event to return.
		EventType EventType
		// The state keys to return for the listed type.
		// If the list is empty then return all state types.
		StateKeys []StateKey
	}
}

// A QueryRoomStateResponse returns the current forward edges and current state
// for a room.
type QueryRoomStateResponse struct {
	// Whether the server is joined to the room.
	IsAlive bool
	// The event IDs and reference SHA-256 hash for the forward edge events.
	ForwardEdges []struct {
		EventID         EventID
		ReferenceSha256 []byte
	}
	// The event IDs for the current state.
	State []EventID
	// The IDs for each partition of the output stream.
	PartitionIDs []int32
	// The offsets for each partition of the output stream.
	Offsets []int64
}

// A QueryStateAfterRequest requests the state after the listed events.
// This is needed if the number of forward edges for the new event exceeds the
// maximum that can be fitted into a single event and the sender is forced
// to use a subset.
// This can also be used to authenticate events returned from other sources.
type QueryStateAfterRequest struct {
	EventIDs []EventID
	// Filter the state to return by event type and state key.
	State map[EventType][]StateKey
}

// A QueryStateAfterResponse returns the state as a list of event IDs.
type QueryStateResponse struct {
	// The state as a list of eventIDs
	State []EventID
}

// A QueryBackwardEdgesRequest requests the backwards edges of the contiguous
// event graph for a room.
type QueryBackwardEdgesRequest struct {
	// The ID of the room.
	RoomID RoomID
}

// A QueryBackwardEdgesRequest returns the events IDs that form the backwards
// edges of the contiguous event graph for a room.
type QueryBackwardEdgesResponse struct {
	EventIDs []EventID
}
