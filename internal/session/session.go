package session

import "github.com/google/uuid"

type Status string

const (
	StatusIdle    Status = "idle"
	StatusRunning Status = "running"
)

type Session struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	UserID           int64  `json:"user_id"`
	WorkDir          string `json:"work_dir"`
	ActivePlanAlias  string `json:"active_plan_alias"`
	ActiveBuildAlias string `json:"active_build_alias"`
	Status           Status `json:"status"`
}

func New(userID int64, name, workDir string) *Session {
	return &Session{
		ID:      uuid.New().String(),
		Name:    name,
		UserID:  userID,
		WorkDir: workDir,
		Status:  StatusIdle,
	}
}
