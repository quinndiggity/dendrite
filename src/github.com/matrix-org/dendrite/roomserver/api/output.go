package api

// An OutputNewEvent is output with an event suitable for notifying clients.
// The new room state and room visibility is also included so that consumers
// can decide which clients to notify about the event.
// The added state and visibility are given as lists of event IDs because
// consumers will typically already have the event for those IDs cached.
// The added state is a list because more than one event could be added to the
// state by a single event as a result of the event refering to new ancestors.
type OutputNewEvent struct {
	// The new room event JSON.
	EventJSON EventJSON
	// The room state that this event adds.
	// This may include the event itself if the event is a state event.
	// This may omit the event itself even though the event is a state
	// event due to conflict resolution.
	AddState []EventID
	// State that this event removes.
	// TODO: Am I overengineering this given that there isn't any way to
	// remove state in the matrix protocol?
	RemoveState []EventID
	// Whether to remove all existing state before updating the state
	// This happens when the server rejoins a room.
	ResetState bool
	// The State IDs needed to determine who this event is visible to.
	VisibilityState []EventID
}

// An OutputNewInvite is output with an invite suitable for notifying clients.
// This is kept in a separate stream to the usual events since they can happen
// without context.
// Consumers should watch for both OutputNewEvent and OutputNewInvite for new
// invites.
type OutputNewInvite struct {
	// The JSON for the invite.
	EventJSON EventJSON
}
