CREATE TABLE rooms (
    -- Local numeric ID for the room.
    room_nid bigint NOT NULL,
    -- Textual ID for the room.
    room_id  text NOT NULL,
    -- The current state of the room.
    state_nid bigint NOT NULL,
    -- List of new events that are not referenced by any event in the room.
    -- Stored as a CBOR list of ints.
    forward_edges bytea NOT NULL,
    -- List of events not in the room that are referenced by non outlier events
    -- Stored as a CBOR list of ints.
    backward_edges byta NOT NULL,
    -- Is the room alive? A room is alive if this server is joined to the room.
    is_alive boolean NOT NULL,
    -- A room may only appear in this table once.
    UNIQUE (room_id)
);

CREATE TABLE events (
    -- Local numeric ID for the event. Postitive IDs are used for new events,
    -- negative IDs are used for backfilled events and outliers.
    event_nid           bigint NOT NULL PRIMARY KEY,
    -- Local numeric ID for the room the event is in.
    room_nid            bigint NOT NULL,
    -- The depth of the event in the room taken from the "depth" key of the
    -- event JSON. This is NULL if the event is an outlier.
    depth               bigint NULL,
    -- Local numeric ID for the state at the event.
    -- This is NULL if we don't know the state at the event.
    -- Since many different events will have the same state we separate the
    -- state into a separate table.
    state_nid           bigint DEFAULT NULL,
    -- The textual event id.
    event_id            text NOT NULL,
    -- The sha256 reference hash for the event.
    reference_sha256    bytea NOT NULL,
    -- Whether the event has been redacted.
    is_redacted boolean NOT NULL,
    -- An event may only appear in this table once.
    UNIQUE (event_id)
);

-- Create an index by depth on the contiguous portion of the event list.
CREATE INDEX event_depth ON events (room_nid, depth, event_nid) WHERE state_nid IS NOT NULL;

-- Stores the JSON for each event. This kept separate from the main events
-- table to keep the rows in the main events table small.
CREATE TABLE event_json (
    event_nid  bigint NOT NULL PRIMARY KEY,
    event_json text NOT NULL
);

-- The state of a room before an event. The state is stored as a CBOR map of
-- (type -> state_key -> event_nid) in ``state_data`` and a list of other
-- state entries to union together with the ``state_data``.
CREATE TABLE state (
    state_nid bigint NOT NULL PRIMARY KEY,
    -- CBOR list of state_nids
    state_delta_nids bytea NOT NULL,
    -- CBOR map of (type -> state_key -> event_nid)
    state_data bytea NOT NULL
);
