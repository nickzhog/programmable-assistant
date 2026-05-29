package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/nickzhog/programmable-assistant/internal/session"
	tele "gopkg.in/telebot.v3"
)

func (h *Handler) newSession(userID int64, name, workDir string) *session.Session {
	sess := session.New(userID, name, workDir)
	if sess.ActivePlanAlias == "" {
		sess.ActivePlanAlias = h.config.OpenCode.Defaults.Plan
	}
	if sess.ActiveBuildAlias == "" {
		sess.ActiveBuildAlias = h.config.OpenCode.Defaults.Build
	}
	return sess
}

func (h *Handler) HandleNewSession(c tele.Context) error {
	userID := c.Sender().ID
	name := strings.TrimSpace(c.Message().Payload)
	if name == "" {
		return c.Send("Usage: /new_session <name>")
	}

	navPath, _ := h.store.GetNavPath(context.Background(), userID)

	sess := h.newSession(userID, name, navPath)
	if err := h.store.SaveSession(context.Background(), sess); err != nil {
		return c.Send(fmt.Sprintf("Error creating session: %v", err))
	}

	h.store.SetActiveSessionID(context.Background(), userID, sess.ID)

	return c.Send(fmt.Sprintf("\u2705 Session created: %s (dir: %s)", sess.Name, sess.WorkDir))
}

func (h *Handler) HandleSessions(c tele.Context) error {
	userID := c.Sender().ID
	sessions, err := h.store.ListSessions(context.Background(), userID)
	if err != nil {
		return c.Send(fmt.Sprintf("Error: %v", err))
	}

	if len(sessions) == 0 {
		return c.Send("No sessions. Create one with /new_session <name>")
	}

	activeID, _ := h.store.GetActiveSessionID(context.Background(), userID)

	var sb strings.Builder
	sb.WriteString("\U0001F4CB Sessions:\n\n")

	var rows [][]tele.InlineButton
	for _, sess := range sessions {
		marker := "  "
		if sess.ID == activeID {
			marker = "\u2705 "
		}
		statusIcon := "\u23F8"
		if sess.Status == session.StatusRunning {
			statusIcon = "\u25B6"
		}
		sb.WriteString(fmt.Sprintf("%s%s %s [%s]\n", marker, statusIcon, sess.Name, sess.WorkDir))

		row := []tele.InlineButton{{
			Text: fmt.Sprintf("Switch: %s", sess.Name),
			Data: fmt.Sprintf("sess:switch:%s", sess.ID),
		}}
		if sess.Status != session.StatusRunning {
			row = append(row, tele.InlineButton{
				Text: fmt.Sprintf("Delete: %s", sess.Name),
				Data: fmt.Sprintf("sess:delete:%s", sess.ID),
			})
		}
		rows = append(rows, row)
	}

	return c.Send(sb.String(), &tele.ReplyMarkup{InlineKeyboard: rows})
}

func (h *Handler) handleSessionSwitch(c tele.Context, data string) error {
	userID := c.Sender().ID
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 3 {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid data"})
	}
	sessionID := parts[2]

	sess, err := h.store.GetSession(context.Background(), sessionID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Session not found"})
	}

	h.store.SetActiveSessionID(context.Background(), userID, sess.ID)
	h.store.SetNavPath(context.Background(), userID, sess.WorkDir)

	return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Switched to: %s", sess.Name)})
}

func (h *Handler) handleSessionDelete(c tele.Context, data string) error {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 3 {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid data"})
	}
	sessionID := parts[2]

	sess, err := h.store.GetSession(context.Background(), sessionID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Session not found"})
	}
	if sess.Status == session.StatusRunning {
		return c.Respond(&tele.CallbackResponse{Text: "Cannot delete running session"})
	}

	h.store.DeleteSession(context.Background(), sessionID)

	activeID, _ := h.store.GetActiveSessionID(context.Background(), c.Sender().ID)
	if activeID == sessionID {
		h.store.SetActiveSessionID(context.Background(), c.Sender().ID, "")
	}

	return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Deleted: %s", sess.Name)})
}
