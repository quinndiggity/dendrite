package storage

import (
	"database/sql"
	"github.com/lib/pq"
	"github.com/matrix-org/dendrite/roomserver/types"
)

type stmts struct {
	insertRoomNIDStmt          *sql.Stmt
	selectRoomNIDStmt          *sql.Stmt
	selectStateAtEventIDsStmt  *sql.Stmt
	selectStateEventsStmt      *sql.Stmt
	insertEventTypeNIDStmt     *sql.Stmt
	selectEventTypeNIDStmt     *sql.Stmt
	insertEventStateKeyNIDStmt *sql.Stmt
	selectEventStateKeyNIDStmt *sql.Stmt
	insertEventStmt            *sql.Stmt
	insertEventJSONStmt        *sql.Stmt
}

func (s *stmts) prepare(db *sql.DB) (err error) {
	p := func(query string) (result *sql.Stmt) {
		if err != nil {
			return
		}
		result, err = db.Prepare(query)
		return
	}

	s.insertRoomNIDStmt = p(
		"INSERT INTO rooms (room_id) VALUES ($1)" +
			" ON CONFLICT ON CONSTRAINT room_id_unique" +
			" DO UPDATE SET room_id = $1" +
			" RETURNING (rooms.room_nid)",
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
	s.insertEventTypeNIDStmt = p(
		"INSERT INTO event_types (event_type) VALUES ($1)" +
			" ON CONFLICT ON CONSTRAINT event_type_unique" +
			" DO UPDATE SET event_type = $1" +
			" RETURNING (event_type_nid)",
	)
	s.selectEventTypeNIDStmt = p(
		"SELECT event_type_nid FROM event_types WHERE event_type = $1",
	)
	s.insertEventStateKeyNIDStmt = p(
		"INSERT INTO event_state_keys (event_state_key) VALUES ($1)" +
			" ON CONFLICT ON CONSTRAINT event_state_key_unique" +
			" DO UPDATE SET event_state_key = $1" +
			" RETURNING (event_state_key_nid)",
	)
	s.selectEventStateKeyNIDStmt = p(
		"SELECT event_state_key_nid FROM event_state_keys" +
			" WHERE event_state_key = $1",
	)
	s.insertEventStmt = p(
		"INSERT INTO events (event_id, room_nid, depth, event_type_nid, event_state_key_nid)" +
			" VALUES ($1, $2, $3, $4, $5)" +
			" ON CONFLICT ON CONSTRAINT event_id_unique" +
			" DO UPDATE SET event_id = $1" +
			" RETURNING (event_nid)",
	)
	s.insertEventJSONStmt = p(
		"INSERT INTO event_json (event_nid, event_json) VALUES ($1, $2)" +
			" ON CONFLICT DO NOTHING",
	)
	return
}

func queryOneInt64(stmt *sql.Stmt, argument string) (result int64, err error) {
	err = stmt.QueryRow(argument).Scan(&result)
	if err == sql.ErrNoRows {
		result = 0
		err = nil
	}
	return
}

func (s *stmts) insertRoomNID(roomID string) (roomNID int64, err error) {
	return queryOneInt64(s.insertRoomNIDStmt, roomID)
}

func (s *stmts) selectRoomNID(roomID string) (roomNID int64, err error) {
	return queryOneInt64(s.selectRoomNIDStmt, roomID)
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
	rows, err := s.selectStateEventsStmt.Query(pq.StringArray(eventIDs))
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

func (s *stmts) insertEventTypeNID(eventType string) (eventTypeNID int64, err error) {
	return queryOneInt64(s.insertEventTypeNIDStmt, eventType)
}

func (s *stmts) selectEventTypeNID(eventType string) (eventTypeNID int64, err error) {
	return queryOneInt64(s.selectEventTypeNIDStmt, eventType)
}

func (s *stmts) insertEventStateKeyNID(eventStateKey string) (eventStateKeyNID int64, err error) {
	return queryOneInt64(s.insertEventStateKeyNIDStmt, eventStateKey)
}

func (s *stmts) selectEventStateKeyNID(eventStateKey string) (eventStateKeyNID int64, err error) {
	return queryOneInt64(s.selectEventStateKeyNIDStmt, eventStateKey)
}

func (s *stmts) insertEvent(eventID string, roomNID, depth, eventTypeNID, eventStateKeyNID int64) (eventNID int64, err error) {
	err = s.insertEventStmt.QueryRow(eventID, roomNID, depth, eventTypeNID, eventStateKeyNID).Scan(&eventNID)
	return
}

func (s *stmts) insertEventJSON(eventNID int64, eventJSON []byte) error {
	_, err := s.insertEventJSONStmt.Exec(eventNID, eventJSON)
	return err
}
