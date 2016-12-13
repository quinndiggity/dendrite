package storage

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/matrix-org/dendrite/roomserver/types"
	"sync"
)

type Database struct {
	stmts
	db              *sql.DB
	createRoomMutex sync.Mutex
	maxRoomNID      int64
}

func (d *Database) Open(dataSourceName string) (err error) {
	if d.db, err = sql.Open("postgres", dataSourceName); err != nil {
		return
	}
	if err = d.prepare(d.db); err != nil {
		return
	}
	if d.maxRoomNID, err = d.selectMaxRoomNID(); err != nil {
		return
	}
	return
}

func (d *Database) CreateNewRoom(roomID string) (roomNID int64, err error) {
	d.createRoomMutex.Lock()
	defer d.createRoomMutex.Unlock()
	// Check that another thread didn't try to create the room while we
	// were waiting to claim the lock.
	roomNID, err = d.selectRoomNID(roomID)
	if err != nil || roomNID != 0 {
		return
	}
	d.maxRoomNID++
	roomNID = d.maxRoomNID
	err = d.insertRoom(roomNID, roomID)
	return
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

func (d *Database) ActiveRegionNID(roomNID int64) (int64, error) {
	panic(fmt.Errorf("Not implemented"))
}

func (d *Database) CreateNewActiveRegion(roomNID, stateNID int64, forward, backward []int64) (int64, error) {
	panic(fmt.Errorf("Not impelemented"))
}
