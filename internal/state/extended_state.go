package state

// ExtendedState represents an extension to the default state object.
type ExtendedState interface {
	// State is the underlying State interface.
	// The type that implements ExtendedState will inherit the implementation of InternalState.
	State

	// SetState sets the underlying State of the ExtendedState. This only needs to be called internally by microcluster.
	SetState(State)

	// GetState returns the underlying State of the ExtendedState. This only needs to be called internally by microcluster.
	GetState() State
}
