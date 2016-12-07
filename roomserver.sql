-- A room holds information about the rooms this server has data for.
CREATE TABLE rooms (
    -- Local numeric ID for the room.
    room_nid bigint NOT NULL PRIMARY KEY,
    -- Textual ID for the room.
    room_id text NOT NULL,
    -- The current state of the room or NULL if the server is no longer joined
    -- to the room.
    state_nid bigint DEFAULT NULL,
    -- The current active region or NULL if the server is no longer joined to
    -- the room.
    active_region_nid bigint DEFAULT NULL,
    -- A room may only appear in this table once.
    UNIQUE (room_id)
);

-- A region is a block of events in a room. that grows forward as new events
-- are received by the server and grows backwards as the server pulls in new
-- events over backfill.
-- There can only be one active region for a room at any given time. A region
-- is created whenever the server joins a room. That region becomes the active
-- region until the server leaves the room.
-- When the server leaves the room the region is retained so that is remains
-- possible to iterate through the events from that time.
-- When the room is rejoined a new region will be created.
-- If the new region is backfilled far enough to collide with the old region
-- then it will use the old region to backfill the new region.
-- The events in the old region will now be in both regions, but will have
-- different positions in each.
CREATE TABLE regions (
    -- Local numeric ID for the region
    region_nid NOT NULL PRIMARY KEY,
    -- The room_nid this region is for.
    room_nid bigint NOT NULL,
    -- List of new events that are not referenced by any event in this room.
    forward_edge_nids bigint[] NOT NULL,
    -- List of event_ids referenced by an event in this contiguous region that
    -- are not referenced by an event in the region.
    backward_edge_ids string[] NOT NULL
);

-- The events table holds metadata for each event, the actual JSON is stored
-- separately to keep the size of the rows small.
CREATE TABLE events (
    -- Local numeric ID for the event.
    bigint NOT NULL PRIMARY KEY,
    -- Local numeric ID for the room the event is in.
    room_nid bigint NOT NULL,
    -- The depth of the event in the room taken from the "depth" key of the
    -- event JSON, corrected to be bigger than the "depth" of the events that
    -- preceeded it.
    -- It is not always possible to correct the depth since we do not always
    -- have copies of the ancestors.
    corrected_depth bigint NOT NULL,
    -- Local numeric ID for the state at the event.
    -- This is NULL if we don't know the state at the event.
    -- If the state is not NULL this this event is part of the contiguous
    -- part of the event graph
    -- Since many different events will have the same state we separate the
    -- state into a separate table.
    state_nid bigint DEFAULT NULL,
    -- Whether the event is a state event
    is_state boolean NOT NULL,
    -- Whether the event has been redacted.
    is_redacted boolean NOT NULL,
    -- The textual event id.
    event_id text NOT NULL,
    -- The sha256 reference hash for the event.
    reference_sha256 bytea NOT NULL,
    -- An event may only appear in this table once.
    UNIQUE (event_id)
);

-- The event_positions table records the positions of events within a region.
-- An event can be part of more than one region.
-- For example a server could leave a room, then rejoin the same room later,
-- then backfill until it reached the events it had already received for the
-- room.
-- A region can be used as a consitent view into a mostly contiguous section
-- of the room.
CREATE TABLE event_positions (
    -- The numeric ID for the contiguous_region the ordering is for.
    contiguous_region_nid bigint NOT NULL,
    -- The position of the event in the contiguous_region. Postitive IDs are
    -- used for new events, negative IDs are used for backfilled events.
    event_position bigint NOT NULL,
    -- The numeric ID of the event.
    event_nid bigint NOT NULL,
    -- Each event should have a different position in a region.
    UNIQUE (contiguous_region_nid, event_position)
);

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
