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

	// Add a new event to the database.
	// Returns the numeric ID assigned to the event ID, type and state_key.
	// If the state_key is nil then the event is not a state_event and the
	// eventStateKeyNID will be 0.
	AddEvent(eventJSON []byte, eventID string, roomNID, depth int64, eventType string, eventStateKey *string) (types.StateAtEvent, error)

	// Lookup the numeric active region ID for a given numeric room ID.
	// Returns 0 if we don't have an active region for that room
	ActiveRegionNID(roomNID int64) (int64, error)

	// Add a new Region to the database.
	CreateNewActiveRegion(roomNID, stateNID int64, forward, backward []int64) (int64, error)

	// Lookup the numeric state data IDs for the each numeric state ID
	// The returned slice is sorted by numeric state ID.
	StateDataNIDs(stateNIDs []int64) ([]types.StateDataNIDList, error)

	// Lookup the state data for each numeric state data ID
	// The returned slice is sorted by numeric state data ID.
	StateEntries(stateDataNIDs []int64) ([]types.StateEntryList, error)

	// Add the state to the database.
	AddState(roomNID int64, stateDataNIDs []int64, state []types.StateEntry) (stateNID int64, err error)

	// Set the state for the event.
	SetEventState(eventNID, stateNID int64) error
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
	stateAtEvent, err := h.storeEvent(roomNID, event)
	if err != nil {
		return err
	}

	// 4) If the events are outliers then we've done enough.
	if input.Kind == api.KindOutlier {
		return nil
	}

	// 5) Store the state for before the event. If the state wasn't given in
	//    input then we will need to calculate it from the prev_events.
	if stateAtEvent.BeforeStateNID == 0 {
		stateNID, err := h.handleState(roomNID, event)
		if err != nil {
			return err
		}
		err = h.db.SetEventState(stateAtEvent.EventNID, stateNID)
		if err != nil {
			return err
		}
	} else {
		// Happy days, we have already stored state for the event.
		// TOCONSIDER: Does the state at an event ever need to change?
	}

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
		// If we don't deduplicate the prev_events then the length check below will fail.
		prevEventIDs = prevEventIDs[:unique(sort.StringSlice(prevEventIDs))]

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

func (h *InputEventHandler) storeEvent(roomNID int64, event event) (types.StateAtEvent, error) {
	return h.db.AddEvent(
		event.raw, event.EventID, roomNID, event.Depth,
		event.Type, event.StateKey,
	)
}

const (
	// The maximum number of state data blocks to compose when encoding the
	// state at an event. If this is too small then it becomes harder to
	// benefit from delta encoding. If this is too large then more state data
	// blocks have to be fetched when loading.
	maxStateDataNIDs = 64
)

func (h *InputEventHandler) handleState(roomNID int64, event event) (int64, error) {
	if event.stateBefore != nil {
		// 1) We've been given the state, we just need to store it.
		// TODO: If this state is part of a backfill it may be possible
		// to delta encode it against the more recent state. We'd need
		// to have a copy of the newer state.
		return h.db.AddState(roomNID, nil, event.stateBefore)
	}
	// We need to work out the state using the supplied information.
	prevStates := uniquePrevStates(event)
	if len(prevStates) == 1 {
		prevState := prevStates[0]
		if prevState.EventStateKeyNID == 0 {
			// 2) None of the previous events were state events and they all
			// have the same state, so this event has exactly the same state
			// as the previous events.
			// This should be the common case.
			return prevState.BeforeStateNID, nil
		}
		// 3) The previous event was a state event so we need to store a copy
		// of the previous state updated with that event.
		stateDataNIDLists, err := h.db.StateDataNIDs([]int64{prevState.BeforeStateNID})
		if err != nil {
			return 0, err
		}
		stateDataNIDs := stateDataNIDLists[0].StateDataNIDs
		if len(stateDataNIDs) < maxStateDataNIDs {
			return h.db.AddState(
				roomNID, stateDataNIDs, []types.StateEntry{prevState.StateEntry},
			)
		}
		// If there are too many deltas then we need to calculate the full state.
	}
	if len(prevStates) == 0 {
		// 4) There weren't any prev_events for this event so the state is
		// empty.
		panic(fmt.Errorf("handleState: Not implemented 4"))
		return 0, nil
	}
	// Conflict resolution.
	// First stage: load the state maps for the prev events.
	stateNIDs := make([]int64, len(prevStates))
	for i, state := range prevStates {
		stateNIDs[i] = state.BeforeStateNID
	}
	sort.Sort(int64Sorter(stateNIDs))
	stateNIDs = stateNIDs[:unique(int64Sorter(stateNIDs))]
	stateDataNIDLists, err := h.db.StateDataNIDs(stateNIDs)
	if err != nil {
		return 0, err
	}

	var stateDataNIDs []int64
	for _, list := range stateDataNIDLists {
		stateDataNIDs = append(stateDataNIDs, list.StateDataNIDs...)
	}
	sort.Sort(int64Sorter(stateNIDs))
	stateDataNIDs = stateDataNIDs[:unique(int64Sorter(stateDataNIDs))]
	stateEntryLists, err := h.db.StateEntries(stateDataNIDs)
	if err != nil {
		return 0, err
	}

	var combined []types.StateEntry
	for _, prevState := range prevStates {
		i := sort.Search(len(stateDataNIDLists), func(i int) bool {
			return stateDataNIDLists[i].StateNID >= prevState.BeforeStateNID
		})
		list := stateDataNIDLists[i]
		var fullState []types.StateEntry
		for _, stateDataNID := range list.StateDataNIDs {
			j := sort.Search(len(stateEntryLists), func(j int) bool {
				return stateEntryLists[j].StateDataNID >= stateDataNID
			})
			fullState = append(fullState, stateEntryLists[j].StateEntries...)
		}
		if prevState.EventStateKeyNID != 0 {
			fullState = append(fullState, prevState.StateEntry)
		}

		// Stable sort so that the most recent entry for each state key stays
		// towards the back
		sort.Stable(sortByStateKey(fullState))
		// Unique returns the last entry for each state key.
		fullState = fullState[:unique(sortByStateKey(fullState))]
		// Add the full state for this StateNID.
		combined = append(combined, fullState...)
	}

	// Collect all the entries with the same type and key together.
	sort.Sort(stateEntrySorter(combined))
	// Remove duplicate entires.
	combined = combined[:unique(stateEntrySorter(combined))]

	// Find the conflicts
	conflicts := duplicateStateKeys(combined)
	if len(conflicts) > 0 {
		// 5) There are conflicting state events, for each conflict workout
		// what the appropriate state event is.
		panic(fmt.Errorf("HandleState: Not implemented 5"))
	} else {
		// 6) There weren't any conflicts.
	}

	panic(fmt.Errorf("HandleState: Not implemented 6"))

	return 0, nil
}

func uniquePrevStates(event event) []types.StateAtEvent {
	result := make([]types.StateAtEvent, len(event.stateAtPrevEvents))
	for i, state := range event.stateAtPrevEvents {
		if state.EventStateKeyNID == 0 {
			// If the event is not a state event then we don't care about its
			// event ID or type.
			state.EventNID = 0
			state.EventTypeNID = 0
		}
		result[i] = state
	}
	sort.Sort(stateAtEventSorter(result))
	return result[:unique(stateAtEventSorter(result))]
}

type sortByStateKey []types.StateEntry

func (s sortByStateKey) Len() int           { return len(s) }
func (s sortByStateKey) Less(i, j int) bool { return s[i].StateKey.LessThan(s[j].StateKey) }
func (s sortByStateKey) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type stateEntrySorter []types.StateEntry

func (s stateEntrySorter) Len() int           { return len(s) }
func (s stateEntrySorter) Less(i, j int) bool { return s[i].LessThan(s[j]) }
func (s stateEntrySorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type stateAtEventSorter []types.StateAtEvent

func (s stateAtEventSorter) Len() int           { return len(s) }
func (s stateAtEventSorter) Less(i, j int) bool { return s[i].LessThan(s[j]) }
func (s stateAtEventSorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type int64Sorter []int64

func (s int64Sorter) Len() int           { return len(s) }
func (s int64Sorter) Less(i, j int) bool { return s[i] < s[j] }
func (s int64Sorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Remove duplicate items from a sorted list.
// Takes the same interface as sort.Sort
// Returns the length of the date without duplicates
// Uses the last occurance of a duplicate.
// O(n).
func unique(data sort.Interface) int {
	length := data.Len()
	j := 0
	for i := 1; i < length; i++ {
		if data.Less(i-1, i) {
			data.Swap(i-1, j)
			j++
		}
	}
	data.Swap(length-1, j)
	return j + 1
}

func duplicateStateKeys(a []types.StateEntry) []types.StateEntry {
	var result []types.StateEntry
	j := 0
	for i := 1; i < len(a); i++ {
		if a[j].StateKey != a[i].StateKey {
			result = append(result, a[j:i]...)
			j = i
		}
	}
	if j != len(a)-1 {
		result = append(result, a[j:]...)
	}
	return result
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
	stateAtEvent types.StateAtEvent `json:"-"`
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
	Depth int64 `json:"depth"`
	// The state_key if present. Needed for state conflict resolution and to
	// know if the event is a state event.
	StateKey *string `json:"state_key"`
	// The content. Needed for processing m.room.member events and for state
	// conflict resolution.
	Content json.RawMessage `json:"content"`
}
