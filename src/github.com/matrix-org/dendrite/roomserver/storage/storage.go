package storage

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/matrix-org/dendrite/roomserver/types"
)

type Database struct {
	stmts
	db *sql.DB
}

func (d *Database) Open(dataSourceName string) (err error) {
	if d.db, err = sql.Open("postgres", dataSourceName); err != nil {
		return
	}
	if err = d.prepare(d.db); err != nil {
		return
	}
	return
}

func (d *Database) CreateNewRoom(roomID string) (roomNID int64, err error) {
	return d.insertRoomNID(roomID)
}

func (d *Database) RoomNID(roomID string) (int64, error) {
	return d.selectRoomNID(roomID)
}

func (d *Database) StateAtEvents(eventIDs []string) ([]types.StateAtEvent, error) {
	// TODO: Cache.
	return d.selectStateAtEventIDs(eventIDs)
}

func (d *Database) StateEventNIDs(eventIDs []string) ([]types.StateEntry, error) {
	// TODO: Cache.
	return d.selectStateEvents(eventIDs)
}

func (d *Database) AddEvent(eventJSON []byte, eventID string, roomNID, depth int64, eventType string, eventStateKey *string) (result types.StateAtEvent, err error) {
	if result.EventTypeNID, err = d.assignEventTypeNID(eventType); err != nil {
		return
	}
	if eventStateKey != nil {
		if result.EventStateKeyNID, err = d.assignEventStateKeyNID(*eventStateKey); err != nil {
			return
		}
	}
	if result.EventNID, result.BeforeStateNID, err = d.insertEvent(
		eventID, roomNID, depth, result.EventTypeNID, result.EventStateKeyNID,
	); err != nil {
		return
	}
	err = d.insertEventJSON(result.EventNID, eventJSON)
	return
}

func (d *Database) assignEventTypeNID(eventType string) (eventTypeNID int64, err error) {
	// TODO: Cache.
	eventTypeNID, err = d.selectEventTypeNID(eventType)
	if err != nil || eventTypeNID != 0 {
		return
	}
	return d.insertEventTypeNID(eventType)
}

func (d *Database) assignEventStateKeyNID(eventStateKey string) (eventStateKeyNID int64, err error) {
	// TODO: Cache.
	eventStateKeyNID, err = d.selectEventStateKeyNID(eventStateKey)
	if err != nil || eventStateKeyNID != 0 {
		return
	}
	return d.insertEventStateKeyNID(eventStateKey)
}

func (d *Database) StateDataNIDs(stateNIDs []int64) ([]types.StateDataNIDList, error) {
	panic(fmt.Errorf("Not implemented"))
}

func (d *Database) StateEntries(stateDataNIDs []int64) ([]types.StateEntryList, error) {
	panic(fmt.Errorf("Not implemented"))
}

func (d *Database) ActiveRegionNID(roomNID int64) (int64, error) {
	panic(fmt.Errorf("Not implemented"))
}

func (d *Database) CreateNewActiveRegion(roomNID, stateNID int64, forward, backward []int64) (int64, error) {
	panic(fmt.Errorf("Not impelemented"))
}
