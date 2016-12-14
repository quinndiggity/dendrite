-- A room holds information about the rooms this server has data for.
CREATE SEQUENCE room_nid_seq;
CREATE TABLE rooms (
    -- Local numeric ID for the room.
    room_nid bigint PRIMARY KEY DEFAULT nextval('room_nid_seq'),
    -- Textual ID for the room.
    room_id text NOT NULL CONSTRAINT room_id_unique UNIQUE,
    -- The current state of the room or 0 if the server is no longer joined
    -- to the room.
    state_nid bigint NOT NULL DEFAULT 0,
    -- The current active region or 0 if the server is no longer joined to
    -- the room.
    active_region_nid bigint NOT NULL DEFAULT 0
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
CREATE SEQUENCE region_nid_seq;
CREATE TABLE regions (
    -- Local numeric ID for the region
    region_nid bigint PRIMARY KEY DEFAULT nextval('region_nid_seq'),
    -- The room_nid this region is for.
    room_nid bigint NOT NULL,
    -- List of new events that are not referenced by any event in this region.
    forward_edge_nids bigint[] NOT NULL,
    -- List of event_ids referenced by an event in this region that are not in
    -- the region.
    backward_edge_ids text[] NOT NULL
);

-- The events table holds metadata for each event, the actual JSON is stored
-- separately to keep the size of the rows small.
-- TODO: Work out how redactions will work.
CREATE SEQUENCE event_nid_seq;
CREATE TABLE events (
    -- Local numeric ID for the event.
    event_nid bigint PRIMARY KEY DEFAULT nextval('event_nid_seq'),
    -- Local numeric ID for the room the event is in.
    room_nid bigint NOT NULL,
    -- The depth of the event in the room taken from the "depth" key of the
    -- event JSON, corrected to be bigger than the "depth" of the events that
    -- preceeded it.
    -- It is not always possible to correct the depth since we do not always
    -- have copies of the ancestors.
    -- Needed for assigning depth when sending new events.
    corrected_depth bigint NOT NULL DEFAULT 0,
    -- The "depth" key of the event in the room taken directly from the "depth"
    -- key of the event.
    -- Needed for state resolution.
    depth bigint NOT NULL,
    -- Local numeric ID for the state at the event.
    -- This is 0 if we don't know the state at the event.
    -- If the state is not 0 this this event is part of the contiguous
    -- part of the event graph
    -- Since many different events will have the same state we separate the
    -- state into a separate table.
    state_nid bigint NOT NULL DEFAULT 0,
    -- Local numeric ID for the type of the event.
    event_type_nid bigint NOT NULL,
    -- The state_key for the event or 0 if the event is not a state event.
    event_state_key_nid bigint NOT NULL,
    -- The textual event id.
    -- Used to lookup the numeric ID when processing requests.
    -- Needed for state resolution.
    -- An event may only appear in this table once.
    event_id text NOT NULL CONSTRAINT event_id_unique UNIQUE,
    -- The sha256 reference hash for the event.
    -- Needed for setting reference hashes when sending new events.
    reference_sha256 bytea NOT NULL DEFAULT ''
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
    region_nid bigint NOT NULL,
    -- The position of the event in the contiguous_region. Postitive IDs are
    -- used for new events, negative IDs are used for backfilled events.
    event_position bigint NOT NULL,
    -- The numeric ID of the event.
    event_nid bigint NOT NULL,
    -- Each event should have a different position in a region.
    UNIQUE (region_nid, event_position)
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

-- Numeric versions of the event "type"s. Event types tend to be taken from a
-- small common pool. Assigning each a numeric ID should reduce the amount of
-- data that needs to be stored and fetched from the database.
-- It also means that many operations can work with int64 arrays rather than
-- string arrays which may help reduce GC pressure.
-- Well known event types are pre-assigned numeric IDs:
--   1 -> m.room.create
--   2 -> m.room.power_levels
--   3 -> m.room.join_rules
--   4 -> m.room.member
-- Picking well-known numeric IDs for the events types that require special
-- attention during state conflict resolution means that we write that code
-- using numeric constants.
-- It also means that the numeric IDs for common event types should be
-- consistent between different instances which might make ad-hoc debugging
-- easier.
CREATE SEQUENCE event_type_nid_seq;
CREATE TABLE event_types (
    -- Local numeric ID for the event type.
    event_type_nid bigint PRIMARY KEY DEFAULT nextval('event_type_nid_seq'),
    -- The string event_type.
    event_type text NOT NULL CONSTRAINT event_type_unique UNIQUE
);
INSERT INTO event_types (event_type) VALUES ('m.room.create');
INSERT INTO event_types (event_type) VALUES ('m.room.power_levels');
INSERT INTO event_types (event_type) VALUES ('m.room.join_rules');
INSERT INTO event_types (event_type) VALUES ('m.room.member');

-- Numeric versions of the event "state_key"s. State keys tend to be reused so
-- assigning each string a numeric ID should reduce the amount of data that
-- needs to be stored and fetched from the database.
-- It also means that many operations can work with int64 arrays rather than
-- string arrays which may help reduce GC pressure.
-- Well known state keys are pre-assigned numeric IDs:
--   1 -> "" (the empty string)
CREATE SEQUENCE event_state_key_nid_seq;
CREATE TABLE event_state_keys (
    -- Local numeric ID for the state key.
    event_state_key_nid bigint PRIMARY KEY DEFAULT nextval('event_state_key_nid_seq'),
    event_state_key text NOT NULL CONSTRAINT event_state_key_unique UNIQUE
);
INSERT INTO event_state_keys (event_state_key) VALUES ('');

-- The state of a room before an event.
-- Stored as a list of state_data entries stored in a separate table.
-- The state could be stored in a single state_data entry but matrix rooms
-- tend to accumulate small changes over time so it's more efficient to encode
-- the state as deltas.
-- If the list of deltas becomes too long it may become more efficient to
-- the full state under single state_data_nid.
CREATE TABLE state (
    -- Local numeric ID for the state.
    state_nid bigint NOT NULL PRIMARY KEY,
    -- Local numeric ID of the room this state is for.
    -- Unused in normal operation, but potentially useful for background work.
    room_nid bigint NOT NULL,
    -- list of state_data_nids, stored sorted by state_data_nid.
    state_data_nids bigint[] NOT NULL
);

-- The state data map.
-- Designed to give enough information to run the state resolution algorithm
-- without hitting the database in the common case.
-- TODO: Is it worth replacing the unique btree index with a covering index so
-- that postgres could lookup the state using an index-only scan?
-- The type and state_key are included in the index to make it easier to
-- lookup a specific (type, state_key) pair for an event. It also makes it easy
-- to read the state for a given state_data_nid ordered by (type, state_key)
-- which in turn makes it easier to merge state data blocks.
CREATE TABLE state_data (
    -- Local numeric ID for this state data.
    state_data_nid bigint NOT NULL,
    event_type_nid bigint NOT NULL,
    event_state_key_nid bigint NOT NULL,
    event_nid bigint NOT NULL,
    UNIQUE (state_data_nid, event_type_nid, event_state_key_nid)
);
