/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Projector is a extension that executes every time when some event is triggered and data need to be updated.
type IProjector interface {
	IExtension

	// Returns is synchronous projector.
	Sync() bool

	// Enumerate events to trigger the projector.
	//
	// Events enumerated in alphabetical QNames order.
	Events(func(IProjectorEvent))

	// Returns events to trigger as map.
	EventsMap() map[QName][]ProjectorEventKind

	// Returns projector states.
	States() IStorages

	// Returns projector intents.
	Intents() IStorages

	// Returns is projector is able to handle `sys.Error` events.
	// False by default.
	WantErrors() bool
}

// Describe event to trigger the projector.
type IProjectorEvent interface {
	IComment

	// Returns type to trigger projector.
	//
	// This can be a record or command.
	On() IType

	// Returns set (sorted slice) of event kind to trigger.
	Kind() []ProjectorEventKind
}

// Events enumeration to trigger the projector
type ProjectorEventKind uint8

//go:generate stringer -type=ProjectorEventKind -output=stringer_projectoreventkind.go

const (
	ProjectorEventKind_Insert ProjectorEventKind = iota + 1
	ProjectorEventKind_Update
	ProjectorEventKind_Activate
	ProjectorEventKind_Deactivate
	ProjectorEventKind_Execute
	ProjectorEventKind_ExecuteWithParam

	ProjectorEventKind_Count
)

// ProjectorEventKind_AnyChanges describes events for record any change.
var ProjectorEventKind_AnyChanges = []ProjectorEventKind{
	ProjectorEventKind_Insert,
	ProjectorEventKind_Update,
	ProjectorEventKind_Activate,
	ProjectorEventKind_Deactivate,
}

type IProjectorBuilder interface {
	IProjector
	IExtensionBuilder

	// Sets is synchronous projector.
	SetSync(bool) IProjectorBuilder

	// Adds event to trigger the projector.
	//
	// QName can be some record type or command.
	//
	// If event kind is missed then default is:
	//   - ProjectorEventKind_Any for GDoc/GRecords, CDoc/CRecords and WDoc/WRecords
	//	 - ProjectorEventKind_Execute for Commands
	//	 - ProjectorEventKind_ExecuteWith for Objects and ODocs
	//
	// # Panics:
	//	- if QName is empty (NullQName)
	//	- if QName type is not a record and not a command
	//	- if event kind is not applicable for QName type.
	AddEvent(on QName, event ...ProjectorEventKind) IProjectorBuilder

	// Sets event comment.
	//
	// # Panics:
	//	- if event for QName is not added.
	SetEventComment(on QName, comment ...string) IProjectorBuilder

	// Returns projector states builder.
	StatesBuilder() IStoragesBuilder

	// Returns projector intents builder.
	IntentsBuilder() IStoragesBuilder

	// Sets is projector is able to handle `sys.Error` events.
	SetWantErrors() IProjectorBuilder
}
