CREATE TABLE rooms (
    -- Local numeric ID for the room.
    room_nid bigint NOT NULL,
    -- Textual ID for the room.
    room_id text NOT NULL,
    -- The current state of the room.
    state_nid bigint NOT NULL,
    -- The current active contiguous regions.
    -- There can be more than one active contiguous region if the room is
    -- forked. A fork happens when a server sends us new events that cannot
    -- be joined to this contiguous region. This should not happen under normal
    -- operation.
    -- New events can reference multiple contiguous regions. The event will be
    -- assigned the lowest active contiguous region of its ancestors.
    contiguous_region_nids bigint[] DEFAULT NULL,
    -- Is the room alive? A room is alive if this server is joined to the room.
    is_alive boolean NOT NULL,
    -- A room may only appear in this table once.
    UNIQUE (room_id)
);

-- A contiguous_regions is a chunk of the event graph for a room where all the
-- events are connected. That is every event either has a "prev_event" in the
-- region or is a "prev_event" of an event in the region.
CREATE TABLE contiguous_regions (
    -- Local numeric ID for the contiguous_region
    contiguous_region_nid NOT NULL,
    -- Back reference to the room_id this contiguous_region is for.
    -- Unused in normal operation, but potentially useful for background work.
    room_nid            bigint NOT NULL,
    -- List of new events that are not referenced by any event in this region.
    forward_edges bigint[] NOT NULL,
    -- List of events in this contigous region that reference an event that is
    -- not in this contiguous region.
    backward_edges bigint[] NOT NULL
);

-- The events table holds the meta-data for each event. The
CREATE TABLE events (
    -- Local numeric ID for the event. Postitive IDs are used for new events,
    -- negative IDs are used for backfilled events and outliers.
    event_nid           bigint NOT NULL PRIMARY KEY,
    -- Local numeric ID for the room the event is in.
    room_nid            bigint NOT NULL,
    -- The contiguous_region this event belongs to.
    -- This can be NULL if the event is an outlier.
    contiguous_region_nid bigint DEFAULT NULL,
    -- The depth of the event in the room taken from the "depth" key of the
    -- event JSON, corrected to be bigger than the "depth" of the events that
    -- preceeded it.
    corrected_depth     bigint NOT NULL,
    -- Local numeric ID for the state at the event.
    -- This is NULL if we don't know the state at the event.
    -- If the state is not NULL this this event is part of the contiguous
    -- part of the event graph
    -- Since many different events will have the same state we separate the
    -- state into a separate table.
    state_nid           bigint DEFAULT NULL,
    -- Whether the event is a state event
    is_state boolean NOT NULL,
    -- Whether the event has been redacted.
    is_redacted boolean NOT NULL,
    -- The textual event id.
    event_id            text NOT NULL,
    -- The sha256 reference hash for the event.
    reference_sha256    bytea NOT NULL,
    -- An event may only appear in this table once.
    UNIQUE (event_id)
);

-- Create an index by depth on each contiguous portion of the event graph.
CREATE INDEX event_depth
    ON events (contiguous_region_nid, corrected_depth, event_nid)
    WHERE contiguous_region_nid IS NOT NULL;

-- Stores the JSON for each event. This kept separate from the main events
-- table to keep the rows in the main events table small.
CREATE TABLE event_json (
    -- Local numeric ID for the event.
    event_nid bigint NOT NULL PRIMARY KEY,
    -- The JSON for the event.
    -- Stored as TEXT because this should be valid UTF-8.
    -- Not stored as a JSONB because we always just pull the entire event.
    -- TODO: Should we be compressing the events with Snappy or DEFLATE?
    event_json text NOT NULL
);

-- The state of a room before an event.
-- Stored as a list of state_data entries stored in a separate table.
CREATE TABLE state (
    -- Local numeric ID for the state.
    state_nid bigint NOT NULL PRIMARY KEY,
    -- Local numeric ID of the room this state is for.
    -- Unused in normal operation, but potentially useful for background work.
    room_nid bigint NOT NULL,
    -- How many times this is referenced in the events table.
    reference_count bigint NOT NULL,
    -- list of state_data_nids
    state_data_nids bigint[] NOT NULL,
);

-- The state data map stored as CBOR map of (type -> state_key -> event_nid).
CREATE TABLE state_data (
    -- Local numeric ID for this state data.
    state_data_nid bigint NOT NULL PRIMARY KEY,
    -- How many times this is referenced in the state table.
    reference_count bigint NOT NULL,
    -- CBOR map of (type -> state_key -> event_nid)
    state_data bytea NOT NULL
);
