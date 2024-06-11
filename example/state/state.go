package state

import (
	"github.com/canonical/microcluster/state"
)

// MyState is an extension to the default microcluster State.
type MyState struct {
	state.State

	AdditionalField string
}

// SetState sets the underlying State of the ExtendedState. This only needs to be called internally by microcluster.
func (s *MyState) SetState(internalState state.State) {
	s.State = s
}

// GetState returns the underlying State of the ExtendedState. This only needs to be called internally by microcluster.
func (s *MyState) GetState() state.State {
	return s.State
}
