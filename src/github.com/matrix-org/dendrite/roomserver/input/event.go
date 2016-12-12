package input

import (
	"encoding/json"
	"fmt"
	"github.com/matrix-org/dendrite/roomserver/api"
	"sort"
)

type InputEventHandlerDatabase interface {
	CreateRoomLock(roomID string) (unlock func())
	ActivateRoomLock(roomNID int64) (unlock func())
	RegionLock(regionNID int64) (unlock func())
	// The next available numeric room ID. They start at 1.
	NextRoomNID() int64
	// Add a new room to the database.
	InsertNewRoom(roomNID int64, roomID string) error
	// Lookup the numeric room ID for a given string room ID.
	// Returns 0 if we don't have a numeric ID for that room.
	RoomNID(roomID string) (int64, error)

	// Lookup the state at each
	GetEventStates(eventNIDs []string) (results []struct {
		EventNID int64
		StateNID int64
		IsState  bool
	}, err error)

	// Assign numeric IDs for each of the events.
	// If the events are new this will asigne a new ID.
	// If the events are old this will return the existing ID.
	// The the smallest new event ID is returned to assit the caller in determining
	// the difference.
	AssignEventNIDs(eventIDs []string) (nids []int64, smallestNewNID int64, err error)

	// The next availble numeric region ID. They start at 1.
	NextRegionNID() int64
	// Add a new Region to the database.
	InsertNewActiveRegion(roomNID, stateNID, regionNID int64, forward, backward []int64) error

	// Lookup the numeric event IDs for the given string event IDs.
	// If some of the events are missing then the returned list
	// will be smaller than the requested list.
	EventNIDs(eventIDs []string) ([]struct {
		EventID  string
		EventNID int64
	}, error)
}

type InputEventHandler struct {
	db InputEventHandlerDatabase
}

func (h *InputEventHandler) Handle(input *api.InputEvents) error {
	// 0) Check that we have some events
	if len(input.Events) == 0 {
		return fmt.Errorf("Asked to add 0 events")
	}

	// 1) Parse the events and check they all have the same roomID.
	// 2) Check that we have all the state events and find their numeric IDs
	roomID, events, states, err := h.prepareEvents(input)
	if err != nil {
		return err
	}

	// 3) Check whether the room exists. If the room doesn't exist then create
	// the room if it's appropriate to do so.
	roomNID, err := h.prepareRoom(input.Kind, roomID)
	if err != nil {
		return err
	}

	// 4) Insert the events and assign them NIDs.
	err = h.insertEvents(roomNID, events)

	// 5) If the events are outliers then we've done enough.
	if input.Kind == api.KindOutlier {
		return nil
	}

	// 4) Check if we have the necessary state if the event's aren't outliers

	// 4) Work out the state at each event.

	// 4) Get the active region if necessary.
	// Outlier events don't need an active region.
}

func (h *InputEventHandler) prepareEvents(input *api.InputEvents) (
	roomID string, events []event, err error,
) {
	events = make([]event, len(input.Events))
	for i, eventJSON := range input.Events {
		// Parse the event JSON.
		event := &events[i]
		event.Raw = eventJSON
		if err = json.Unmarshal(event.Raw, event); err != nil {
			return
		}

		// Check that the string roomID is consistent.
		if i == 0 {
			roomID = event.RoomID
		} else if event.RoomID != roomID {
			err = fmt.Errorf("All events must have the same room ID: %q != %q", roomID, event.RoomID)
			return
		}

		// Check that we have all the referenced state events
		if stateIDs, exists := input.State[event.EventID]; exists {
			event.StateEventNIDs, err = h.db.EventNIDs(stateIDs)
			if err != nil {
				return
			}
			if len(event.StateEventNIDs) != len(stateIDs) {
				err = fmt.Errorf("Missing necessary state event ID for %q", event.EventID)
				return
			}
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
	// That out a lock to make sure that we don't race with another request
	// that attempts to create the room.
	unlock := h.db.CreateRoomLock(roomID)
	defer unlock()
	// Check that there still isn't an ID now that we hold the lock.
	roomNID, err = h.db.RoomNID(roomID)
	if err != nil || roomNID != 0 {
		return
	}
	// The room doesn't exist so create it.
	roomNID = h.db.NextRoomNID()
	err = h.db.InsertNewRoom(roomNID, roomID)
	return
}

func (h *InputEventHandler) checkStates(events []event) {
	// Work out which string event IDs we need state for.
	eventIDs := make([]string, len(events))
	prevEventIDs := make([]string, 0, 2*len(events))
	for i, event := range events {
		eventsIDs[i] = event.EventID
		if event.StateEventNIDs == nil {
			// We weren't supplied with the state at this event.
			// So we'll need to calculate it using the state for each of the
			// "prev_events" of this event.
			for _, prevEvent := range event.PrevEvents {
				prevEventIDs = append(prevEventIDs, prevEvent.EventID)
			}
		}
	}
	sort.Strings(eventIDs)
	sort.Strings(prevEventIDs)
	eventIDs = unique(eventIDs)
	// We only need to look up the numeric IDs for the events that aren't in
	// this block, since we learned their numeric IDs when we inserted them.
	prevEventIDs = difference(unique(requiredPrevEventIDs), eventIDs)

	// Fetch both the required state and the state for the eventIDs we are
	// adding in case we already have some state for those events.
	states, err := h.db.GetEventStates(append(requiredStateEventIDS, eventIDs))

	// Check that we have states for all the event IDs we need state for.
	for _, eventID := range requiredStateEventIDs {
		// The states are sorted by eventID so we can use a binary search here.
		i = sort.Search(len(states), func(i int) {
			return states[i].EventID >= eventId
		})
		if i == len(state) || states[i].EventID != eventID {
			return fmt.Errorf("Missing for previous event ID: %q", eventID)
		}
	}
}

type stateAtEvent struct {
	StateNID      int64
	PrevEventNIDs []int64
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

// difference returns all the elements that are in the first sorted slice
// that aren't in the second sorted slice.
func difference(a, b []string) []string {
	result := make([]string, 0, len(a))
	for {
		if len(a) == 0 {
			return result
		}
		if len(b) == 0 {
			return append(result, a...)
		}
		valueA := a[0]
		valueB := b[0]
		if valueA < valueB {
			result = append(result, valueA)
			a = a[1:]
		} else {
			b = b[1:]
			if valueA == valueB {
				a = a[1:]
			}
		}
	}
}

func (h *InputEventHandler) prepareRegion(kind int, roomNID int64) (regionNID int64, err error) {
	// Check if the room has a region without holding a lock.
	regionNID, err = h.db.ActiveRegion(roomNID)
	if err != nil || regionNID != 0 {
		return
	}
	// The room doesn't have an active region. Check if we should make one.
	if input.Kind != api.KindJoin {
		err = fmt.Errorf("A room can only be actived by a Join: %d", roomNID)
	}
	return
}

type eventReference struct {
	// The event ID referred to.
	EventID string
}

func (er *eventReference) UnmarshallJSON([]byte) error {
	// TODO: implement this.
}

type event struct {
	// Copy of the raw JSON.
	raw []byte `json:"-"`
	// The state event numeric IDs at the event or nil if none were provided.
	stateEventNIDs []int64 `json:"-"`
	eventNID       int64
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
