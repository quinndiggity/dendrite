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
	selectNextStateDataNIDStmt *sql.Stmt
	insertStateDataStmt        *sql.Stmt
	insertStateStmt            *sql.Stmt
	updateEventStateStmt       *sql.Stmt
	selectStateDataNIDsStmt    *sql.Stmt
	selectStateDataEntriesStmt *sql.Stmt
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
			" FROM events WHERE event_id = ANY($1) AND state_nid != 0",
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
			" RETURNING event_nid, state_nid",
	)
	s.insertEventJSONStmt = p(
		"INSERT INTO event_json (event_nid, event_json) VALUES ($1, $2)" +
			" ON CONFLICT DO NOTHING",
	)
	s.selectNextStateDataNIDStmt = p(
		"SELECT nextval('state_data_nid_seq')",
	)
	s.insertStateDataStmt = p(
		"INSERT INTO state_data (state_data_nid, event_type_nid, event_state_key_nid, event_nid)" +
			" VALUES ($1, $2, $3, $4)",
	)
	s.insertStateStmt = p(
		"INSERT INTO state (room_nid, state_data_nids)" +
			" VALUES ($1, $2)" +
			" RETURNING state_nid",
	)
	s.updateEventStateStmt = p(
		"UPDATE events SET state_nid = $2 WHERE event_nid = $1",
	)
	s.selectStateDataNIDsStmt = p(
		"SELECT state_nid, state_data_nids FROM state" +
			" WHERE state_nid = ANY($1) ORDER BY state_nid",
	)
	s.selectStateDataEntriesStmt = p(
		"SELECT state_data_nid, event_type_nid, event_state_key_nid, event_nid" +
			" FROM state_data WHERE state_data_nid = ANY($1)" +
			" ORDER BY state_data_nid, event_type_nid, event_state_key_nid",
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
			&result.BeforeStateNID,
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
	i := 0
	for ; rows.Next(); i++ {
		result := &results[i]
		if err = rows.Scan(
			&result.EventNID,
			&result.EventTypeNID,
			&result.EventStateKeyNID,
		); err != nil {
			return
		}
	}
	results = results[:i]
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

func (s *stmts) insertEvent(eventID string, roomNID, depth, eventTypeNID, eventStateKeyNID int64) (eventNID int64, stateNID int64, err error) {
	err = s.insertEventStmt.QueryRow(eventID, roomNID, depth, eventTypeNID, eventStateKeyNID).Scan(&eventNID, &stateNID)
	return
}

func (s *stmts) insertEventJSON(eventNID int64, eventJSON []byte) error {
	_, err := s.insertEventJSONStmt.Exec(eventNID, eventJSON)
	return err
}

func (s *stmts) selectNextStateDataNID() (stateDataNID int64, err error) {
	err = s.selectNextStateDataNIDStmt.QueryRow().Scan(&stateDataNID)
	return
}

func (s *stmts) insertStateData(stateDataNID int64, entries []types.StateEntry) error {
	for _, entry := range entries {
		_, err := s.insertStateDataStmt.Exec(
			stateDataNID,
			entry.EventTypeNID,
			entry.EventStateKeyNID,
			entry.EventNID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *stmts) insertState(roomNID int64, stateDataNIDs []int64) (stateNID int64, err error) {
	err = s.insertStateStmt.QueryRow(roomNID, pq.Int64Array(stateDataNIDs)).Scan(&stateNID)
	return
}

func (s *stmts) updateEventState(eventNID, stateNID int64) error {
	_, err := s.updateEventStateStmt.Exec(eventNID, stateNID)
	return err
}

func (s *stmts) selectStateDataNIDs(stateNIDs []int64) ([]types.StateDataNIDList, error) {
	rows, err := s.selectStateDataNIDsStmt.Query(pq.Int64Array(stateNIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]types.StateDataNIDList, 0, len(stateNIDs))
	for rows.Next() {
		var result types.StateDataNIDList
		var stateDataNids pq.Int64Array
		if err := rows.Scan(&result.StateNID, &stateDataNids); err != nil {
			return nil, err
		}
		result.StateDataNIDs = stateDataNids
		results = append(results, result)
	}
	return results, nil
}

func (s *stmts) selectStateDataEntries(stateDataNIDs []int64) ([]types.StateEntryList, error) {
	rows, err := s.selectStateDataEntriesStmt.Query(pq.Int64Array(stateDataNIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]types.StateEntryList, len(stateDataNIDs))
	var dummy types.StateEntryList
	current := &dummy
	i := 0
	for rows.Next() {
		var stateDataNID int64
		var entry types.StateEntry
		if err := rows.Scan(
			&stateDataNID,
			&entry.EventTypeNID, &entry.EventStateKeyNID, &entry.EventNID,
		); err != nil {
			return nil, err
		}
		if stateDataNID != current.StateDataNID {
			current = &results[i]
			current.StateDataNID = stateDataNID
			i++
		}
		current.StateEntries = append(current.StateEntries, entry)
	}
	return results[:i], nil
}
