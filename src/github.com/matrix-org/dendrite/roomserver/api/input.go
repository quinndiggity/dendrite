package roomserver

type EventID string
type EventJSON []byte

const (
	// Outlier events fall outside the contiguous event graph.
	// We do not have the state for these events.
	// These events are state events used to authenticate other events.
	// They can become part of the contiguous event graph via backfill.
	KindOutlier = 0
	// New Events extend the contiguous graph going forwards.
	// They always have state.
	KindNew = 1
	// Backfilled events extend the contiguous graph going backwards.
	// They always have state.
	KindBackfill = 2
)

type InputEvents struct {
	// Whether these events are new, backfilled or outliers.
	Kind int
	// The event JSON for each event to add.
	Events []EventJSON
	// Optional list of state events forming the intial state for the
	// backward extremities.
	// These state events must have already been persisted.
	State map[EventID][]EventID
}

type InputPurgeHistory struct {
	// The room_id to remove history from.
	RoomID string
	// The depth to purge history up to.
	Depth int64
}

type InputRedact struct {
	// List of events to redact.
	EventIDs []EventID
}

type QueryEventExistsRequest struct {
}

type QueryEventJSONRequest struct {
	EventIDs []EventID
}

type QueryEventsResponse struct {
	// Bitmap of
	Exists []byte
	Events []EventJSON
	Auth   [][]EventID
	State  [][]EventID
}

type QueryStateRequest struct {
}

type QueryStateResponse struct {
	State map[EventID][]EventID
}
