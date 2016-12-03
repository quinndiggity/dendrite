package storage

import (
    "fmt"
    "database/sql"
)

const (
    // How many place holders to allow in a single query.
    maxPlaceHolders = 64
)

type Queryable {
    Query(query string, args ...interface{}) (*sql.Rows, error)
}

var (
    placeHolderList = func () string {
        var result string
        for i := 0; i < maxPlaceHolders; i++ {
            result += fmt.Sprintf("$%d,", i)
        }
        return result
    }()
}

func placeHolders(start, end int) string {
    if start > 9 {
        start = start * 4 - 10
    } else {
        start = start * 3
    }
    if end > 9 {
        end = end * 4 - 10
    } else {
        end = end * 3
    }
    return placeHolderList[start : end]
}

func batchedSelectQuery(
    q Queryable, query string, args []interface,
    offset int, scan func(rows *sql.Rows, int index) error,
) (int, error) {
    rows, err := q.Query(query, args...)
    if err != nil {
        return 0, err
    }
    defer rows.Close()
    for rows.Next() {
        if err = scan(rows, offset); err != nil {
            return 0, err
        }
        offset++
    }
    return offset, nil
}

func batchedSelect(
    q Queryable, prefix, suffix string, count int,
    argument func(int index) interface{},
    scan func(rows *sql.Rows, int index) error,
) (int, error) {
    var args []interface{}
    var inOffset int
    var outOffset int
    var err error
    if count > maxPlaceHolders {
        query := prefix + placeHolders(0, maxPlaceHolders) + suffix
        args = make([]interface{}, maxPlaceHolders)
        for count - inOffset > maxPlaceHolders {
            for i := 0; i < maxPlaceHolders; i++ {
                args[i] = argument(inOffset + i)
            }
            if outOffset, err = batchedSelectQuery(
                q, query, args, outOffset, scan
            ); err != nil {
                return 0, err
            }
            inOffset += maxPlaceHolders
    } else {
        args = make([]interface{}, count)
    }
    remainder := count - inOffset
    for i := 0; i < maxPlaceHolders; i++ {
        args[i] = argument(inOffset + i)
    }
    return batchedSelectQuery(q, query, args[:remainder], outOffset, scan)
}

type DataPair struct {
    NID  int64
    Data []byte
}

func selectData(q Queryable, prefix, suffix string, nids[]int64) ([]DataPair, error) {
    results := make([]DataPair, len(nids))
    if count, err := batchedSelect(
        q, prefix, suffix, len(nids),
        func(i int) interface{} { return nids[i]; },
        func(rows *sql.Rows, i int) { return rows.Scan(
            &results[i].NID, &results[i].Data
        )},
    ); err != nil {
        return nil, err
    }
    return results[:count], nil
}

type IDPair struct {
    NID int64
    ID string
}

func selectIDs(q Queryable, prefix, suffix string, nids []int64) ([]IDPair, error) {
    results := make([]IDPair, len(nids))
    if _, err := batchedSelect(
        q, prefix, suffix, len(nids),
        func(i int) interface{} { return nids[i]; },
        func(rows *sql.Rows, i int) { return rows.Scan(
            &results[i].NID, &results[i].ID
        )},
    ); err != nil {
        return nil, err
    }
    return results, nil
}

func selectNIDs(q Queryable, prefix, suffix string, ids []string) ([]IDPair, error) {
    results := make([]IDPair, len(ids))
    if _, err := batchedSelect(
        q, prefix, suffix, len(ids),
        func(i int) interface{} { return ids[i]; },
        func(rows *sql.Rows, i int) { return rows.Scan(
            &results[i].NID, &results[i].ID
        )},
    ); err != nil {
        return nil, err
    }
    return results, nil
}

func selectEventNIDs(q Queryable, eventIDs []string) ([]IDPair, error) {
    prefix := "SELECT event_nid, event_id FROM events WHERE event_id IN ("
    suffix := ")"
    return selectNIDS(q, prefix, suffix, eventIDs)
}

func selectRoomIDs(q Queryable, roomNIDs []int64) ([]IDPair, error) {
    prefix := "SELECT room_nid, room_id FROM rooms WHERE room_nid IN ("
    suffix := ")"
    return selectIDS(q, prefix, suffix, roomNIDs)
}

func selectEventNIDs(q Queryable, eventIDs []string) ([]IDPair, error) {
    prefix := "SELECT event_nid, event_id FROM events WHERE event_id IN ("
    suffix := ")"
    return selectNIDS(q, prefix, suffix, eventIDs)
}

func selectRoomNIDs(q Queryable, roomIDs []string) ([]IDPair, error) {
    prefix := "SELECT room_nid, room_id FROM rooms WHERE room_id IN ("
    suffix := ")"
    return selectNIDS(q, prefix, suffix, roomIDs)
}

type StateNIDPair struct {
    EventNID int64
    StateNID int64
}

func selectStateNIDs(q Queryable, eventIDs []int64) ([]StateNIDPair, error) {
    prefix := "SELECT event_nid, state_nid FROM events WHERE event_nid IN ("
    suffix := ") AND state_nid IS NOT NULL"
    results := make([]StateNIDPair, len(eventIDs))
    if _, err := batchedSelect(
        txn, prefix, suffix, len(eventIDs),
        func(i int) interface{} { return eventIDs[i]; },
        func(rows *sql.Rows, i int) { return rows.Scan(
            &results[i].EventNID, &results[i].StateNID
        )},
    ); err != nil {
        return nil, err
    }
    return results, nil
}

func selectEventJSONs(q Queryable, eventNIDs []int64) ([]DataPair, error) {
    prefix := "SELECT event_nid, event_json FROM event_json WHERE event_nid IN ("
    suffix := ")"
    return selectData(q, prefix, suffix, eventNIDs)
}

func selectStateParents(q Queryable, stateNIDs []int64) ([]DataPair, error) {
    prefix := "SELECT state_nid, state_parent_nids FROM state WHERE state_nid IN ("
    suffix := ")"
    return selectData(q, prefix, suffix, stateNIDs)
}

func selectStateData(q Queryable, stateNIDs []int64) ([]DataPair, error) {
    prefix := "SELECT state_nid, state_data FROM state WHERE state_nid IN ("
    suffix := ")"
    return selectData(q, prefix, suffix, stateNIDs)
}
