package events

import (
	"github.com/ashutosh0x/infra-control/pkg/types"
)

// Re-export event types from pkg/types for convenience within internal packages
type Event = types.Event
type EventHandler = types.EventHandler

// Internal specific event types can be defined here
// e.g. types that should not be exposed in the public pkg/types API
