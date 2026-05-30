package handler

import (
	"context"
	"strings"

	"github.com/nickzhog/programmable-assistant/internal/session"
	tele "gopkg.in/telebot.v3"
)

func (h *Handler) handleAbort(c tele.Context, data string) error {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) < 2 {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid data"})
	}
	sessionID := parts[1]

	h.runner.Abort(sessionID)

	return c.Respond(&tele.CallbackResponse{Text: "Aborted"})
}

func (h *Handler) handleBuildFromPlan(c tele.Context, data string) error {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) < 2 {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid data"})
	}
	planSessionID := parts[1]

	planSess, err := h.store.GetSession(context.Background(), planSessionID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Plan session not found"})
	}

	if planSess.OpenCodeSessionID == "" {
		return c.Respond(&tele.CallbackResponse{Text: "No opencode session to fork from. Re-run /plan first."})
	}

	if planSess.Status == session.StatusRunning {
		return c.Respond(&tele.CallbackResponse{Text: "Session is still running"})
	}

	userID := c.Sender().ID

	activeID, _ := h.store.GetActiveSessionID(context.Background(), userID)
	if activeID != planSessionID {
		h.store.SetActiveSessionID(context.Background(), userID, planSessionID)
	}

	c.Respond(&tele.CallbackResponse{Text: "Running build from plan..."})

	return h.runMode(c, "build", "Implement the plan above.", true)
}
