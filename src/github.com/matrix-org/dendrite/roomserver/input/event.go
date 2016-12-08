package input

import (
	"encoding/json"
	"github.com/matrix-org/dendrite/roomserver/api"
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

	// Assign numeric IDs for each of the events.
	// If the events are new this will asigne a new ID.
	// If the events are old this will return the existing ID.
	// The the smallest new event ID is returned to assit the caller in determining
	// the difference.
	AssignEventNIDs(eventIDs []string) (nids []int64, smallestNewNID int64, err error)

	//

	// The next availble numeric region ID. They start at 1.
	NextRegionNID() int64
	// Add a new Region to the database.
	InsertNewActiveRegion(roomNID, stateNID, regionNID int64, forward, backward []int64) error

	// Lookup the numeric event IDs for the given string event IDs.
	// If some of the events are missing then the returned list
	// will be smaller than the requested list.
	EventNIDs(eventIDs []string) ([]int64, error)
}

type InputEventHandler struct {
	db InputEventHandlerDatabase
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
	Raw []byte `json:"-"`
	// The event_id. We need this so that we can check if we already have this
	// event in the room.
	EventID string
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
	roomNID, err := prepareRoom(input.Kind, roomID)
	if err != nil {
		return err
	}

	// 4) Work out the state at each event. If the events are outliers then this
	// does very little.

	// 4) Get the active region if necessary.
	// Outlier events don't need an active region.
}

func (h *InputEventHandler) prepareEvents(input *api.InputEvents) (
	roomID string, events []event, states [][]int64, err error,
) {
	events = make([]event, len(input.Events))
	states = make([][]int64, len(input.Events))
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
			states[i], err = h.db.EventNIDs(stateIDs)
			if err != nil {
				return
			}
			if len(states[i]) != len(stateIDs) {
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
	roomNID, err := h.db.RoomNID(roomID)
	if err != nil || roomNID != 0 {
		return
	}
	// The room doesn't exist so create it.
	roomNID := h.db.NextRoomNID()
	err = h.db.InsertNewRoom(roomNID, roomID)
	return
}

func (h *InputEventHandler) prepareRegion(kind int, roomNID int64) (regionNID int64, err error) {
	// Outlier events don't need an active region.
	if input.Kind == api.KindOutlier {
		return
	}
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
