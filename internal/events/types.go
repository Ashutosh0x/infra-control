package events

import (
	"github.com/ashutosh0x/infra-control/pkg/types"
)

// Event is an alias for types.Event, re-exported so internal packages can use
// the shared type without importing pkg/types directly.
type Event = types.Event

// EventHandler is an alias for types.EventHandler.
type EventHandler = types.EventHandler

// Internal specific event types can be defined here
// e.g. types that should not be exposed in the public pkg/types API
