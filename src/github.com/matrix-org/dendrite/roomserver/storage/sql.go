package storage

import (
	"database/sql"
	"github.com/lib/pq"
	"github.com/matrix-org/dendrite/roomserver/types"
)

type stmts struct {
	insertRoomStmt            *sql.Stmt
	selectMaxRoomNIDStmt      *sql.Stmt
	selectRoomNIDStmt         *sql.Stmt
	selectStateAtEventIDsStmt *sql.Stmt
	selectStateEventsStmt     *sql.Stmt
}

func (s *stmts) prepare(db *sql.DB) (err error) {
	p := func(query string) (result *sql.Stmt) {
		if err != nil {
			return
		}
		result, err = db.Prepare(query)
		return
	}

	s.insertRoomStmt = p(
		"INSERT INTO rooms (room_nid, room_id) VALUES ($1, $2)",
	)
	s.selectMaxRoomNIDStmt = p(
		"SELECT room_nid FROM rooms ORDER BY room_nid DESC LIMIT 1",
	)
	s.selectRoomNIDStmt = p(
		"SELECT room_nid FROM rooms WHERE room_id = $1",
	)
	s.selectStateAtEventIDsStmt = p(
		"SELECT event_nid, event_type_nid, event_state_key_nid, state_nid" +
			" FROM events WHERE event_id = ANY($1)",
	)
	s.selectStateEventsStmt = p(
		"SELECT event_nid, event_type_nid, event_state_key_nid" +
			" FROM events WHERE event_id = ANY($1)",
	)
	return
}

func (s *stmts) insertRoom(roomNID int64, roomID string) error {
	_, err := s.insertRoomStmt.Exec(roomNID, roomID)
	return err
}

func (s *stmts) selectMaxRoomNID() (roomNID int64, err error) {
	err = s.selectMaxRoomNIDStmt.QueryRow().Scan(&roomNID)
	if err == sql.ErrNoRows {
		roomNID = 0
		err = nil
	}
	return
}

func (s *stmts) selectRoomNID(roomID string) (roomNID int64, err error) {
	err = s.selectRoomNIDStmt.QueryRow(roomID).Scan(&roomNID)
	if err == sql.ErrNoRows {
		roomNID = 0
		err = nil
	}
	return
}

func (s *stmts) selectStateAtEventIDs(eventIDs []string) (results []types.StateAtEvent, err error) {
	results = make([]types.StateAtEvent, len(eventIDs))
	rows, err := s.selectStateAtEventIDsStmt.Query(pq.StringArray(eventIDs))
	if err != nil {
		return
	}
	defer rows.Close()
	for i := 0; rows.Next(); i++ {
		result := &results[i]
		if err = rows.Scan(
			&result.EventNID,
			&result.EventTypeNID,
			&result.EventStateKeyNID,
			&result.BeforeStateID,
		); err != nil {
			return
		}
	}
	return
}

func (s *stmts) selectStateEvents(eventIDs []string) (results []types.StateEntry, err error) {
	results = make([]types.StateEntry, len(eventIDs))
	rows, err := s.selectStateAtEventIDsStmt.Query(pq.StringArray(eventIDs))
	if err != nil {
		return
	}
	defer rows.Close()
	for i := 0; rows.Next(); i++ {
		result := &results[i]
		if err = rows.Scan(
			&result.EventNID,
			&result.EventTypeNID,
			&result.EventStateKeyNID,
		); err != nil {
			return
		}
	}
	return
}
