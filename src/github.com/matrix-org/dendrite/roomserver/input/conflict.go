package input

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/matrix-org/dendrite/roomserver/types"
	"github.com/matrix-org/gomatrixserverlib/eventauth"
	"sort"
)

type ConflictResolutionDatabase interface {
	EventStateKeyNIDs(eventStateKeys []string) ([]types.IDPair, error)
	EventJSONs(eventNIDs []int64) ([]types.EventJSON, error)
}

func resolveConflicts(db ConflictResolutionDatabase, state, conflicts []types.StateEntry) ([]types.StateEntry, error) {
	// 1) Load all the conflicting events from the database.
	conflictEventNIDs := make([]int64, len(conflicts))
	for i, conflict := range conflicts {
		conflictEventNIDs[i] = conflict.EventNID
	}
	sort.Sort(int64Sorter(conflictEventNIDs))
	conflictJSONs, err := db.EventJSONs(conflictEventNIDs)
	if err != nil {
		return nil, err
	}

	// 2) Parse the conflicted events.
	conflictEvents := make([]eventauth.Event, len(conflictJSONs))
	for i, eventJSON := range conflictJSONs {
		if err := json.Unmarshal(eventJSON.EventJSON, &conflictEvents[i]); err != nil {
			return nil, err
		}
	}

	// 3) Find the unconflicted state
	var unconflicted []types.StateEntry
	conflictedMap := newStateEntryMap(conflicts)
	for _, stateEntry := range state {
		if _, ok := conflictedMap.lookup(stateEntry.StateKey); ok {
			continue
		}
		unconflicted = append(unconflicted, stateEntry)
	}
	unconflictedMap := newStateEntryMap(unconflicted)

	// 4) Work out which bits of state are needed for auth.
	stateNeeded := eventauth.StateNeededForAuth(conflictEvents)

	// 5) Load the numeric IDs for the event state keys so that we can load the
	// relevant events from the database.
	var eventStateKeys []string
	eventStateKeys = append(eventStateKeys, stateNeeded.Member...)
	eventStateKeys = append(eventStateKeys, stateNeeded.ThirdPartyInvite...)
	eventStateKeyNIDs, err := db.EventStateKeyNIDs(eventStateKeys)
	if err != nil {
		return nil, err
	}
	eventStateKeyMap := newIDMap(eventStateKeyNIDs)

	// 6) Load the auth events.
	authStateKeys := listStateKeys(stateNeeded, eventStateKeyMap)
	authEventNIDs := listAuthEventNIDs(authStateKeys, unconflictedMap)
	sort.Sort(int64Sorter(authEventNIDs))
	authEventJSONs, err := db.EventJSONs(authEventNIDs)
	if err != nil {
		return nil, err
	}
	auth, err := newAuthEvents(stateNeeded, authEventJSONs, eventStateKeyMap, unconflictedMap)
	if err != nil {
		return nil, err
	}

	// 7) resolve the conflicts
	resolved := resolveConflictBlocks(auth, conflicts, conflictJSONs, conflictEvents)

	resolved = append(resolved, unconflicted...)

	sort.Sort(stateEntrySorter(resolved))

	return resolved, nil
}

func resolveConflictBlocks(
	auth authEvents,
	conflicted []types.StateEntry,
	eventJSONs []types.EventJSON,
	events []eventauth.Event,
) []types.StateEntry {
	var resolvingType int64
	var resolvingState []resolvedStateKey
	var result []types.StateEntry
	for len(conflicted) != 0 {
		blockKey := conflicted[0].StateKey
		if resolvingType != blockKey.EventTypeNID {
			auth.updateAuthEventsWithKeys(resolvingType, resolvingState)
			resolvingType = 0
			resolvingState = nil
		}
		var block []types.StateEntry
		i := 0
		for i < len(conflicted) && block[i].StateKey == blockKey {
			block = append(block, conflicted[i])
			i++
		}
		entry, event, stateKey := resolveBlock(auth, block, eventJSONs, events)
		result = append(result, entry)
		if blockKey.EventTypeNID == mRoomMemberNID || blockKey.EventTypeNID == mRoomThirdPartyInviteNID {
			resolvingState = append(resolvingState, resolvedStateKey{stateKey, event})
		}
		if blockKey.EventStateKeyNID == emptyStateKeyNID {
			auth.updateAuthEvents(blockKey.EventTypeNID, event)
		}
		conflicted = conflicted[i:]
	}
	return result
}

func (a *authEvents) updateAuthEvents(eventType int64, event *eventauth.Event) {
	if eventType == mRoomCreateNID {
		a.create = event
	}

	if eventType == mRoomPowerLevelsNID {
		a.powerLevels = event
	}

	if eventType == mRoomJoinRulesNID {
		a.joinRules = event
	}
}

func (a *authEvents) updateAuthEventsWithKeys(eventType int64, keys []resolvedStateKey) {
	if eventType == 0 {
		return
	}

	if eventType == mRoomMemberNID {
		for _, key := range keys {
			a.member[key.stateKey] = key.event
		}
	}

	if eventType == mRoomThirdPartyInviteNID {
		for _, key := range keys {
			a.thirdPartyInvite[key.stateKey] = key.event
		}
	}
}

func (a *authEvents) badlyNamedFunction(eventTypeNID int64, eventStateKeyNID int64, eventStateKey string, event *eventauth.Event) {
	if eventTypeNID == mRoomCreateNID && eventStateKeyNID == emptyStateKeyNID {
		a.create = event
	}
	if eventTypeNID == mRoomPowerLevelsNID && eventStateKeyNID == emptyStateKeyNID {
		a.powerLevels = event
	}
	if eventTypeNID == mRoomJoinRulesNID && eventStateKeyNID == emptyStateKeyNID {
		a.powerLevels = event
	}
	if eventTypeNID == mRoomThirdPartyInviteNID {
		a.thirdPartyInvite[eventStateKey] = event
	}
	if eventTypeNID == mRoomMemberNID {
		a.member[eventStateKey] = event
	}
}

type resolvedStateKey struct {
	stateKey string
	event    *eventauth.Event
}

func resolveBlock(
	auth authEvents,
	conflicted []types.StateEntry,
	eventJSONs []types.EventJSON,
	events []eventauth.Event,
) (entry types.StateEntry, event *eventauth.Event, stateKey string) {
	var things []badlyNamed
	for _, c := range conflicted {
		var parsedEvent struct {
			Depth    int64  `json:"depth"`
			EventID  string `json:"event_id"`
			StateKey string `json:"state_key"`
		}
		index, ok := lookupEventNID(eventJSONs, c.EventNID)
		if !ok {
			panic(fmt.Errorf("Corrupt DB: Missing numeric event ID %d", c.EventNID))
		}
		if err := json.Unmarshal(eventJSONs[index].EventJSON, &parsedEvent); err != nil {
			panic(fmt.Errorf("Corrupt DB: Unable to parse event with numeric ID %d", c.EventNID))
		}

		var thing badlyNamed
		thing.depth = parsedEvent.Depth
		thing.sha1 = sha1.Sum([]byte(parsedEvent.EventID))
		thing.eventNID = c.EventNID
		thing.event = &events[index]
		things = append(things, thing)

		stateKey = parsedEvent.StateKey
		entry.StateKey = c.StateKey
	}

	sort.Sort(badlyNamedSorter(things))

	var i int
	if entry.EventTypeNID > maxAuthEventNID {
		for i = len(things) - 1; i > 0; i-- {
			if eventauth.Allowed(*things[i].event, &auth) == nil {
				break
			}
		}
	} else {
		var i int
		for i = 1; i < len(things); i++ {
			auth.badlyNamedFunction(entry.EventTypeNID, entry.EventStateKeyNID, stateKey, things[i-1].event)
			if eventauth.Allowed(*things[i].event, &auth) != nil {
				break
			}
		}
		i -= 1
		auth.badlyNamedFunction(entry.EventTypeNID, entry.EventStateKeyNID, stateKey, nil)
	}
	entry.EventNID = things[i].eventNID
	event = things[i].event
	return
}

type badlyNamed struct {
	depth    int64
	sha1     [sha1.Size]byte
	eventNID int64
	event    *eventauth.Event
}

type badlyNamedSorter []badlyNamed

func (s badlyNamedSorter) Len() int {
	return len(s)
}

func (s badlyNamedSorter) Less(i, j int) bool {
	if s[i].depth == s[j].depth {
		return bytes.Compare(s[i].sha1[:], s[j].sha1[:]) == 1
	}
	return s[i].depth < s[j].depth
}

func (s badlyNamedSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

const (
	mRoomCreateNID           = 1
	mRoomPowerLevelsNID      = 2
	mRoomJoinRulesNID        = 3
	mRoomThirdPartyInviteNID = 4
	mRoomMemberNID           = 5
	maxAuthEventNID          = 5
	emptyStateKeyNID         = 1
)

type authEvents struct {
	create           *eventauth.Event
	powerLevels      *eventauth.Event
	joinRules        *eventauth.Event
	member           map[string]*eventauth.Event
	thirdPartyInvite map[string]*eventauth.Event
}

func newAuthEvents(
	stateNeeded eventauth.StateNeeded,
	eventJSONs []types.EventJSON,
	stateNIDMap idMap,
	state stateEntryMap,
) (authEvents, error) {
	var result authEvents
	result.member = make(map[string]*eventauth.Event)
	result.thirdPartyInvite = make(map[string]*eventauth.Event)

	events := make([]eventauth.Event, len(eventJSONs))
	for i, eventJSON := range eventJSONs {
		if err := json.Unmarshal(eventJSON.EventJSON, &events[i]); err != nil {
			return result, err
		}
	}

	eventWithEmptyStateKey := func(typeNID int64) *eventauth.Event {
		eventNID, ok := state.lookup(types.StateKey{typeNID, emptyStateKeyNID})
		if !ok {
			return nil
		}
		index, ok := lookupEventNID(eventJSONs, eventNID)
		if !ok {
			return nil
		}
		return &events[index]
	}

	eventWithStateKey := func(typeNID int64, stateKey string) *eventauth.Event {
		stateKeyNID, ok := stateNIDMap.lookup(stateKey)
		if !ok {
			return nil
		}
		eventNID, ok := state.lookup(types.StateKey{typeNID, stateKeyNID})
		if !ok {
			return nil
		}
		index, ok := lookupEventNID(eventJSONs, eventNID)
		if !ok {
			return nil
		}
		return &events[index]
	}

	if stateNeeded.Create {
		result.create = eventWithEmptyStateKey(mRoomCreateNID)
	}
	if stateNeeded.PowerLevels {
		result.powerLevels = eventWithEmptyStateKey(mRoomPowerLevelsNID)
	}
	if stateNeeded.JoinRules {
		result.joinRules = eventWithEmptyStateKey(mRoomJoinRulesNID)
	}
	for _, key := range stateNeeded.Member {
		result.member[key] = eventWithStateKey(mRoomMemberNID, key)
	}
	for _, key := range stateNeeded.Member {
		result.thirdPartyInvite[key] = eventWithStateKey(mRoomThirdPartyInviteNID, key)
	}
	return result, nil
}

func (a *authEvents) Create() (*eventauth.Event, error) {
	return a.create, nil
}

func (a *authEvents) PowerLevels() (*eventauth.Event, error) {
	return a.powerLevels, nil
}

func (a *authEvents) JoinRules() (*eventauth.Event, error) {
	return a.joinRules, nil
}

func (a *authEvents) Member(stateKey string) (*eventauth.Event, error) {
	return a.member[stateKey], nil
}

func (a *authEvents) ThirdPartyInvite(stateKey string) (*eventauth.Event, error) {
	return a.thirdPartyInvite[stateKey], nil
}

func listStateKeys(stateNeeded eventauth.StateNeeded, stateNIDMap idMap) []types.StateKey {
	var stateKeys []types.StateKey
	if stateNeeded.Create {
		stateKeys = append(stateKeys, types.StateKey{mRoomCreateNID, emptyStateKeyNID})
	}
	if stateNeeded.PowerLevels {
		stateKeys = append(stateKeys, types.StateKey{mRoomPowerLevelsNID, emptyStateKeyNID})
	}
	if stateNeeded.JoinRules {
		stateKeys = append(stateKeys, types.StateKey{mRoomJoinRulesNID, emptyStateKeyNID})
	}
	for _, member := range stateNeeded.Member {
		stateKeyNID, ok := stateNIDMap.lookup(member)
		if ok {
			stateKeys = append(stateKeys, types.StateKey{mRoomMemberNID, stateKeyNID})
		}
	}
	for _, token := range stateNeeded.ThirdPartyInvite {
		stateKeyNID, ok := stateNIDMap.lookup(token)
		if ok {
			stateKeys = append(stateKeys, types.StateKey{mRoomThirdPartyInviteNID, stateKeyNID})
		}
	}
	return stateKeys
}

func listAuthEventNIDs(stateKeys []types.StateKey, state stateEntryMap) []int64 {
	var eventNIDs []int64
	for _, stateKey := range stateKeys {
		eventNID, inState := state.lookup(stateKey)
		if inState {
			eventNIDs = append(eventNIDs, eventNID)
		}
	}
	return eventNIDs
}

type idMap map[string]int64

func newIDMap(ids []types.IDPair) idMap {
	result := make(map[string]int64)
	for _, pair := range ids {
		result[pair.ID] = pair.NID
	}
	return idMap(result)
}

func (m idMap) lookup(id string) (nid int64, ok bool) {
	nid, ok = map[string]int64(m)[id]
	return
}

type stateEntryMap []types.StateEntry

func newStateEntryMap(stateEntries []types.StateEntry) stateEntryMap {
	return stateEntryMap(stateEntries)
}

func (m stateEntryMap) lookup(stateKey types.StateKey) (eventNID int64, ok bool) {
	list := []types.StateEntry(m)
	i := sort.Search(len(list), func(i int) bool {
		return !list[i].StateKey.LessThan(stateKey)
	})
	if list[i].StateKey == stateKey {
		ok = true
		eventNID = list[i].EventNID
	}
	return
}

func lookupEventNID(eventJSONs []types.EventJSON, eventNID int64) (index int, ok bool) {
	i := sort.Search(len(eventJSONs), func(i int) bool {
		return eventJSONs[i].EventNID >= eventNID
	})
	if eventJSONs[i].EventNID == eventNID {
		ok = true
		index = i
	}
	return
}
