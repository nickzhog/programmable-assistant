package handler

import (
	"fmt"
	"context"
	"sort"
	"strings"

	"github.com/nickzhog/programmable-assistant/internal/fsm"
	tele "gopkg.in/telebot.v3"
)

func (h *Handler) HandleProvider(c tele.Context) error {
	userID := c.Sender().ID

	activeID, err := h.store.GetActiveSessionID(context.Background(), userID)
	if err != nil || activeID == "" {
		return c.Send("No active session. Create one with /new_session <name>")
	}

	if err := h.fsm.Start(context.Background(), userID, fsm.ProviderNS); err != nil {
		return c.Send(fmt.Sprintf("Error: %v", err))
	}

	return h.renderProviderModeStep(c, userID)
}

func (h *Handler) renderProviderModeStep(c tele.Context, userID int64) error {
	activeID, _ := h.store.GetActiveSessionID(context.Background(), userID)
	sess, _ := h.store.GetSession(context.Background(), activeID)

	var sb strings.Builder
	sb.WriteString("\U0001F916 Select mode:\n\n")
	if sess != nil {
		sb.WriteString(fmt.Sprintf("Plan:  %s\n", sess.ActivePlanAlias))
		sb.WriteString(fmt.Sprintf("Build: %s\n", sess.ActiveBuildAlias))
	}

	return c.Send(sb.String(), &tele.ReplyMarkup{InlineKeyboard: [][]tele.InlineButton{
		{{Text: "\U0001F9E0 Plan alias", Data: "fsm:provider:mode:plan"}},
		{{Text: "\U0001F528 Build alias", Data: "fsm:provider:mode:build"}},
		{{Text: "\u274C Cancel", Data: "fsm:provider:cancel"}},
	}})
}

func (h *Handler) handleProviderCallback(c tele.Context, data string) error {
	userID := c.Sender().ID

	switch {
	case data == "fsm:provider:cancel":
		h.fsm.Finish(context.Background(), userID)
		return c.Respond(&tele.CallbackResponse{Text: "Cancelled"})
	case strings.HasPrefix(data, "fsm:provider:mode:"):
		return h.handleProviderMode(c, data, userID)
	case strings.HasPrefix(data, "fsm:provider:alias:"):
		return h.handleProviderAlias(c, data, userID)
	}

	return c.Respond(&tele.CallbackResponse{Text: "Unknown action"})
}

func (h *Handler) handleProviderMode(c tele.Context, data string, userID int64) error {
	mode := strings.TrimPrefix(data, "fsm:provider:mode:")
	if mode != "plan" && mode != "build" {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid mode"})
	}

	if err := h.fsm.Transition(context.Background(), userID, fsm.StepSelectAlias, map[string]string{
		"mode": mode,
	}); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Error: %v", err)})
	}

	return h.renderProviderAliasStep(c, userID, mode)
}

func (h *Handler) renderProviderAliasStep(c tele.Context, userID int64, mode string) error {
	aliases := h.config.AliasNames()
	sort.Strings(aliases)

	var rows [][]tele.InlineButton
	for _, name := range aliases {
		a := h.config.OpenCode.Aliases[name]
		rows = append(rows, []tele.InlineButton{{
			Text: fmt.Sprintf("%s (%s/%s)", name, a.Provider, a.Model),
			Data: fmt.Sprintf("fsm:provider:alias:%s:%s", mode, name),
		}})
	}
	rows = append(rows, []tele.InlineButton{{
		Text: "\u274C Cancel",
		Data: "fsm:provider:cancel",
	}})

	modeIcon := "\U0001F9E0"
	modeLabel := "Plan"
	if mode == "build" {
		modeIcon = "\U0001F528"
		modeLabel = "Build"
	}

	return c.Send(
		fmt.Sprintf("%s Select %s alias:", modeIcon, modeLabel),
		&tele.ReplyMarkup{InlineKeyboard: rows},
	)
}

func (h *Handler) handleProviderAlias(c tele.Context, data string, userID int64) error {
	parts := strings.SplitN(data, ":", 5)
	if len(parts) < 5 {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid data"})
	}
	mode := parts[3]
	aliasName := parts[4]

	if _, ok := h.config.OpenCode.Aliases[aliasName]; !ok {
		return c.Respond(&tele.CallbackResponse{Text: "Unknown alias"})
	}

	activeID, err := h.store.GetActiveSessionID(context.Background(), userID)
	if err != nil || activeID == "" {
		h.fsm.Finish(context.Background(), userID)
		return c.Respond(&tele.CallbackResponse{Text: "No active session"})
	}

	sess, err := h.store.GetSession(context.Background(), activeID)
	if err != nil {
		h.fsm.Finish(context.Background(), userID)
		return c.Respond(&tele.CallbackResponse{Text: "Session not found"})
	}

	if mode == "plan" {
		sess.ActivePlanAlias = aliasName
	} else {
		sess.ActiveBuildAlias = aliasName
	}

	if err := h.store.SaveSession(context.Background(), sess); err != nil {
		h.fsm.Finish(context.Background(), userID)
		return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Error: %v", err)})
	}

	h.fsm.Finish(context.Background(), userID)

	alias := h.config.OpenCode.Aliases[aliasName]
	modeLabel := "Plan"
	if mode == "build" {
		modeLabel = "Build"
	}

	return c.Send(fmt.Sprintf(
		"\u2705 %s alias set to: %s (%s/%s)",
		modeLabel, aliasName, alias.Provider, alias.Model,
	))
}
