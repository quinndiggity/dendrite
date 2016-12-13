package input

import (
	"encoding/json"
	"fmt"
	"github.com/matrix-org/dendrite/roomserver/api"
	"github.com/matrix-org/dendrite/roomserver/types"
	"sort"
)

type InputEventHandlerDatabase interface {
	// Add a new room to the database.
	// The room starts off without an active region.
	CreateNewRoom(roomID string) (roomNID int64, err error)

	// Lookup the numeric room ID for a given string room ID.
	// Returns 0 if we don't have a numeric ID for that room.
	RoomNID(roomID string) (int64, error)

	// Lookup the state at each event.
	StateAtEvents(eventIDs []string) ([]types.StateAtEvent, error)

	// Lookup the numeric event IDs for the given string state event IDs.
	// If some of the events are missing then the returned list
	// will be smaller than the requested list.
	StateEventNIDs(eventIDs []string) ([]types.StateEntry, error)

	// Lookup the numeric active region ID for a given numeric room ID.
	// Returns 0 if we don't have an active region for that room
	ActiveRegionNID(roomNID int64) (int64, error)

	// Add a new Region to the database.
	CreateNewActiveRegion(roomNID, stateNID int64, forward, backward []int64) (int64, error)
}

type InputEventHandler struct {
	db InputEventHandlerDatabase
}

func (h *InputEventHandler) Setup(db InputEventHandlerDatabase) {
	h.db = db
}

func (h *InputEventHandler) Handle(input *api.InputEvent) error {
	// 1) Check that the event is valid JSON and check that we have all the
	//    necessary state to process the event:
	//     a) If the input specifies the state before the event then check that
	//        all the referenced state has been persisted.
	//     b) If the input is of kind Outlier check that either the state
	//        before the event is specified in the input or we have the state
	//        for all of the prev_events.
	roomID, event, err := h.prepareState(input)
	if err != nil {
		return err
	}

	// 2) Check whether the room exists. If the room doesn't exist then create
	//    the room if it's appropriate to do so.
	roomNID, err := h.prepareRoom(input.Kind, roomID)
	if err != nil {
		return err
	}

	// 3) Insert the event and assign it a NID.
	err = h.insertEvent(roomNID, event)

	// 4) If the events are outliers then we've done enough.
	if input.Kind == api.KindOutlier {
		return nil
	}

	// 5) Store the state for before the event. If the state wasn't given in
	//    input then we will need to calculate it from the prev_events.

	// 6) Get the active region for the room and update it with the event.
	//    If the input is of kind Join then we may need to create a new region.
	//    If the input is of kind Backfill then we add the event to old end of
	//    the region, otherwise we add the event to the new end of the region.

	// 4) Get the active region if necessary.
	// Outlier events don't need an active region.
	return nil
}

func (h *InputEventHandler) prepareState(input *api.InputEvent) (
	roomID string, event event, err error,
) {
	// Parse the event JSON.
	event.raw = input.Event
	if err = json.Unmarshal(event.raw, &event); err != nil {
		return
	}

	roomID = event.RoomID

	if input.Kind == api.KindOutlier {
		// We don't need to check for state for outlier events.
		return
	}

	if input.State != nil {
		event.stateBefore, err = h.db.StateEventNIDs(input.State)
		if err != nil {
			return
		}

		if len(event.stateBefore) != len(input.State) {
			err = fmt.Errorf("Missing necessary state event for %q", event.EventID)
			return
		}
	} else {
		prevEventIDs := make([]string, len(event.PrevEvents))
		for i, prevEvent := range event.PrevEvents {
			prevEventIDs[i] = prevEvent.EventID
		}
		sort.Strings(prevEventIDs)
		// Remove duplicates prev_events. Do we need to do this?
		// Should we allow duplicate prev_event entries in the same event?
		prevEventIDs = unique(prevEventIDs)

		// Look up the states for the prevEvents.
		event.stateAtPrevEvents, err = h.db.StateAtEvents(prevEventIDs)
		if err != nil {
			return
		}
		if len(event.stateAtPrevEvents) != len(prevEventIDs) {
			err = fmt.Errorf("Missing necessary state at prev_event for %q", event.EventID)
			return
		}
	}
	return
}

func (h *InputEventHandler) prepareRoom(kind int, roomID string) (roomNID int64, err error) {
	// First check if there's an ID without holding the lock.
	roomNID, err = h.db.RoomNID(roomID)
	if err != nil || roomNID != 0 {
		return
	}
	// The room doesn't exists. Check if we should create it.
	if kind != api.KindOutlier {
		err = fmt.Errorf("The first events added to a room must be outliers: %q", roomID)
		return
	}
	// Create a new room.
	roomNID, err = h.db.CreateNewRoom(roomID)
	return
}

func (h *InputEventHandler) insertEvent(roomNID int64, event event) error {
	// TODO: insert the event.
	return nil
}

// unique removes duplicate elements from a sorted slice.
// Modifes the slice in-place O(n)
func unique(a []string) []string {
	if len(a) == 0 {
		return nil
	}
	lastValue := a[0]
	var j int
	for _, value := range a {
		if value != lastValue {
			a[j] = lastValue
			lastValue = value
			j++
		}
	}
	a[j] = lastValue
	j++
	return a[:j]
}

func (h *InputEventHandler) prepareRegion(kind int, roomNID int64) (regionNID int64, err error) {
	// Check if the room has a region without holding a lock.
	regionNID, err = h.db.ActiveRegionNID(roomNID)
	if err != nil || regionNID != 0 {
		return
	}
	// The room doesn't have an active region. Check if we should make one.
	if kind != api.KindJoin {
		err = fmt.Errorf("A room can only be actived by a Join: %d", roomNID)
		return
	}
	return
}

type eventReference struct {
	// The event ID referred to.
	EventID string
}

func (er *eventReference) UnmarshalJSON(data []byte) error {
	// TODO: implement this.
	var parts []json.RawMessage
	if err := json.Unmarshal(data, &parts); err != nil {
		return err
	}
	if len(parts) != 2 {
		return fmt.Errorf("input: More than two elements in prev_events")
	}
	return json.Unmarshal(parts[0], &er.EventID)
}

type event struct {
	// Copy of the raw JSON.
	raw []byte `json:"-"`
	// The state event numeric IDs at the event or nil if none were provided.
	stateBefore []types.StateEntry `json:"-"`
	// The state entry information for this event.
	eventStateEntry types.StateEntry `json:"-"`
	// The state for each of the prev events if needed.
	stateAtPrevEvents []types.StateAtEvent `json:"-"`
	// The event_id. We need this so that we can check if we already have this
	// event in the room.
	EventID string `json:"event_id"`
	// The room_id. Needed so we know which room to update.
	RoomID string `json:"room_id"`
	// The prev_events for the event. Needed for tracking forward and backward
	// edges for the room.
	PrevEvents []eventReference `json:"prev_events"`
	// The type of the event. Needed for state conflict resolution.
	Type string `json:"type"`
	// The depth of the event. Needed for working out the corrected depth.
	Depth int `json:"depth"`
	// The state_key if present. Needed for state conflict resolution and to
	// know if the event is a state event.
	StateKey *string `json:"state_key"`
	// The content. Needed for processing m.room.member events and for state
	// conflict resolution.
	Content json.RawMessage `json:"content"`
}
