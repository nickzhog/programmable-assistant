package store

import (
	"context"

	"github.com/nickzhog/programmable-assistant/internal/fsm"
	"github.com/nickzhog/programmable-assistant/internal/session"
)

type Store interface {
	Close() error

	SaveSession(ctx context.Context, s *session.Session) error
	GetSession(ctx context.Context, id string) (*session.Session, error)
	ListSessions(ctx context.Context, userID int64) ([]*session.Session, error)
	DeleteSession(ctx context.Context, id string) error

	GetNavPath(ctx context.Context, userID int64) (string, error)
	SetNavPath(ctx context.Context, userID int64, path string) error

	GetFSMState(ctx context.Context, userID int64) (*fsm.State, error)
	SetFSMState(ctx context.Context, userID int64, state *fsm.State) error
	ClearFSMState(ctx context.Context, userID int64) error

	GetActiveSessionID(ctx context.Context, userID int64) (string, error)
	SetActiveSessionID(ctx context.Context, userID int64, sessionID string) error

	SetCallbackRef(ctx context.Context, key string, value string) error
	GetCallbackRef(ctx context.Context, key string) (string, error)
}
