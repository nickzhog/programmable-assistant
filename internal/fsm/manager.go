package fsm

import (
	"context"
	"fmt"
)

const (
	ProviderNS      = "provider"
	StepSelectMode  = "select_mode"
	StepSelectAlias = "select_alias"
	StepComplete    = "complete"
)

type Store interface {
	GetFSMState(ctx context.Context, userID int64) (*State, error)
	SetFSMState(ctx context.Context, userID int64, state *State) error
	ClearFSMState(ctx context.Context, userID int64) error
}

type Manager struct {
	store Store
}

func NewManager(store Store) *Manager {
	return &Manager{store: store}
}

func (m *Manager) Start(ctx context.Context, userID int64, ns string) error {
	existing, err := m.store.GetFSMState(ctx, userID)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("another FSM flow is already active")
	}
	return m.store.SetFSMState(ctx, userID, &State{
		Step:      StepSelectMode,
		Data:      make(map[string]string),
		HandlerNS: ns,
	})
}

func (m *Manager) Transition(ctx context.Context, userID int64, step string, data map[string]string) error {
	state, err := m.store.GetFSMState(ctx, userID)
	if err != nil || state == nil {
		return fmt.Errorf("no active FSM state")
	}
	state.Step = step
	for k, v := range data {
		state.Data[k] = v
	}
	return m.store.SetFSMState(ctx, userID, state)
}

func (m *Manager) GetState(ctx context.Context, userID int64) (*State, error) {
	return m.store.GetFSMState(ctx, userID)
}

func (m *Manager) Finish(ctx context.Context, userID int64) error {
	return m.store.ClearFSMState(ctx, userID)
}
