package storage

import (
	"database/sql"
	"github/lib/pq"
)

type DataPair struct {
	NID  int64
	Data []byte
}

type IDPair struct {
	NID int64
	ID  string
}

type stmts struct {
	selectEventIDsStmt       *sql.Stmt
	selectEventNIDsStmt      *sql.Stmt
	selectRoomIDsStmt        *sql.Stmt
	selectRoomNIDsStmt       *sql.Stmt
	selectEventStateNIDsStmt *sql.Stmt
	selectEventJSONsStmt     *sql.Stmt
	selectStateParentsStmt   *sql.Stmt
	selectStateDataStmt      *sql.Stmt
	insertRoomStmt           *sql.Stmt
	insertEventStmt          *sql.Stmt
	insertEventJSONStmt      *sql.Stmt
	insertStateStmt          *sql.Stmt
	updateRoomStmt           *sql.Stmt
	updateEventStmt          *sql.Stmt
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
		"INSERT INTO rooms (room_nid, room_id, state_nid, forward_edges," +
			" backward_edges, is_alive) VALUES ($1, $2, $3, $4, $5)",
	)
	s.selectRoomIDsStmt = p(
		"SELECT room_nid, room_id FROM rooms WHERE room_nid = ANY($1)",
	)
	s.selectRoomNIDsStmt = p(
		"SELECT room_nid, room_id FROM rooms WHERE room_id = ANY($1)",
	)
	s.selectRoomRegionsStmt = p(
		"SELECT active_contiguous_region_nids FROM rooms WHERE room_id = ANY($1)",
	)

	s.insertEventStmt = p(
		"INSERT INTO events (event_nid, room_nid, depth, state_nid," +
			" is_state, is_redacted, event_id, reference_sha256)" +
			" VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
	)
	s.selectEventIDsStmt = p(
		"SELECT event_nid, event_id FROM events WHERE event_nid = ANY($1)",
	)
	s.selectEventNIDsStmt = p(
		"SELECT event_nid, event_id FROM events WHERE event_id = ANY($1)",
	)
	s.selectEventStateNIDsStmt = p(
		"SELECT event_nid, state_nid WHERE event_nid = ANY($1)" +
			" AND state_nid IS NOT NULL",
	)

	s.insertEventJSON = p(
		"INSERT INTO event_json (event_nid, event_json) VALUES ($1, $2)",
	)
	s.selectEventJSONsStmt = p(
		"SELECT event_nid, event_json FROM event_json" +
			" WHERE event_id = ANY($1)",
	)

	s.selectStateDataNIDsStmt = p(
		"SELECT state_nid, state_data_nids FROM state" +
			" WHERE state_nid = ANY($1)",
	)
	s.selectStateDataStmt = p(
		"SELECT state_data_nid, state_data FROM state_data" +
			" WHERE state_data_nid = ANY($1)",
	)
	s.insertStateStmt = p(
		"INSERT INTO state (state_nid, room_nid, state_parent_nids," +
			" state_data) VALUES ($1, $2, $3, $4)",
	)
	s.updateRoomStmt = p(
		"UPDATE rooms SET state_nid=$2, forward_edges=$3, backward_edges=$4," +
			" is_alive=$5 WHERE room_nid=$1",
	)
	s.updateEventStmt = p(
		"UPDATE events SET state_nid=$2, is_redacted=$3 WHERE event_nid=$1",
	)
}

func (s *stmts) selectEventIDs(eventNIDs []int64) ([]IDPair, error) {
	return selectIDS(s.selectEventIDsStmt, eventNIDs)
}

func (s *stmts) selectEventNIDs(eventIDs []string) ([]IDPair, error) {
	return selectNIDS(s.selectEventIDsStmt, eventIDs)
}

func (s *stmts) selectRoomIDs(roomNIDs []int64) ([]IDPair, error) {
	return selectIDS(s.selectRoomIDsStmt, roomNIDs)
}

func (s *stmts) selectRoomNIDs(roomIDs []string) ([]IDPair, error) {
	return selectNIDS(s.selectRoomIDsStmt, roomIDs)
}

type EventStateNIDPair struct {
	EventNID int64
	StateNID int64
}

func (s *stmts) selectEventStateNIDs(eventIDs []int64) ([]EventStateNIDPair, error) {
	rows, err := s.selectEventStateNIDsStmt.Query(pq.Int64Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var i int
	results := make([]EventStateNIDPair, len(nids))
	for rows.Next() {
		if err = rows.Scan(&results[i].EventNID, &results[i].StateID); err != nil {
			return nil, err
		}
		i++
	}
	return results[:i], nil
}

func (s *stmts) selectEventJSONs(eventNIDs []int64) ([]DataPair, error) {
	return selectData(s.selectEventJSONsStmt, eventNIDs)
}

func selectStateParents(q Queryable, stateNIDs []int64) ([]DataPair, error) {
	return selectData(s.selectStateParentsStmt, stateNIDs)
}

func selectStateData(q Queryable, stateNIDs []int64) ([]DataPair, error) {
	return selectData(s.selectDataStmt, stateNIDs)
}

func selectData(stmt *sql.Stmt, nids []int64) ([]DataPair, error) {
	rows, err := stmt.Query(pq.Int64Array(nids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var i int
	results := make([]DataPair, len(nids))
	for rows.Next() {
		if err = rows.Scan(&results[i].NID, &results[i].Data); err != nil {
			return nil, err
		}
		i++
	}
	return results[:i], nil
}

func selectIDs(q Queryable, prefix, suffix string, nids []int64) ([]IDPair, error) {
	rows, err := stmt.Query(pq.Int64Array(nids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var i int
	results := make([]IDPair, len(nids))
	for rows.Next() {
		if err = rows.Scan(&results[i].NID, &results[i].ID); err != nil {
			return nil, err
		}
		i++
	}
	return results[:i], nil
}

func selectNIDs(q Queryable, prefix, suffix string, ids []string) ([]IDPair, error) {
	rows, err := stmt.Query(pq.StringArray(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var i int
	results := make([]IDPair, len(nids))
	for rows.Next() {
		if err = rows.Scan(&results[i].NID, &results[i].ID); err != nil {
			return nil, err
		}
		i++
	}
	return results[:i], nil
}
