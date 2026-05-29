package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/nickzhog/programmable-assistant/internal/runner"
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
	parts := strings.SplitN(data, ":", 4)
	if len(parts) < 4 {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid data"})
	}
	planSessionID := parts[2]

	planSess, err := h.store.GetSession(context.Background(), planSessionID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Plan session not found"})
	}

	msgText := c.Message().Text
	if msgText == "" {
		msgText = c.Callback().Message.Text
	}
	if msgText == "" {
		return c.Respond(&tele.CallbackResponse{Text: "No plan text found"})
	}

	pt, ok := h.config.OpenCode.Aliases[planSess.ActivePlanAlias]
	providerName := ""
	modelName := ""
	if ok {
		providerName = pt.Provider
		modelName = pt.Model
	}
	prefixLine := fmt.Sprintf("[Mode: Plan | Provider: %s | Model: %s]\n", providerName, modelName)

	planText := strings.TrimPrefix(msgText, prefixLine)
	planText = strings.TrimSuffix(planText, fmt.Sprintf("\n\n\u2705 %s complete.", "plan"))

	userID := c.Sender().ID

	sess := h.newSession(userID, fmt.Sprintf("build-%s", planSess.Name), planSess.WorkDir)
	if planSess.ActiveBuildAlias != "" {
		sess.ActiveBuildAlias = planSess.ActiveBuildAlias
	}
	if err := h.store.SaveSession(context.Background(), sess); err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Failed to create build session"})
	}
	h.store.SetActiveSessionID(context.Background(), userID, sess.ID)

	c.Respond(&tele.CallbackResponse{Text: "Build session created, running..."})

	aliasKey := sess.ActiveBuildAlias
	if aliasKey == "" {
		aliasKey = h.config.OpenCode.Defaults.Build
	}
	if aliasKey == "" {
		h.bot.Send(c.Recipient(), "No build alias configured. Use /provider.")
		return nil
	}

	alias, ok := h.config.OpenCode.Aliases[aliasKey]
	if !ok {
		h.bot.Send(c.Recipient(), fmt.Sprintf("Alias %s not found.", aliasKey))
		return nil
	}

	sess.Status = session.StatusRunning
	h.store.SaveSession(context.Background(), sess)

	opts := runner.RunOptions{
		WorkDir:  sess.WorkDir,
		Alias:    alias,
		AliasKey: aliasKey,
		Mode:     "build",
		Prompt:   planText,
	}

	prefix := fmt.Sprintf("[Mode: Build | Provider: %s | Model: %s", alias.Provider, alias.Model)
	if alias.Thinking != "" {
		prefix += fmt.Sprintf(" | Thinking: %s", alias.Thinking)
	}
	prefix += "]\n"

	msg, msgErr := h.bot.Send(c.Recipient(), prefix+"Starting...", &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{{Text: "\U0001F6D1 Abort", Data: fmt.Sprintf("abort:%s", sess.ID)}},
		},
	})
	if msgErr != nil {
		sess.Status = session.StatusIdle
		h.store.SaveSession(context.Background(), sess)
		return nil
	}

	runCtx := context.Background()

	ch, runErr := h.runner.Run(runCtx, sess.ID, opts)
	if runErr != nil {
		sess.Status = session.StatusIdle
		h.store.SaveSession(context.Background(), sess)
		h.bot.Edit(msg, prefix+fmt.Sprintf("Error: %v", runErr))
		return nil
	}

	go h.streamOutput(runCtx, ch, msg, sess, prefix, "build")

	return nil
}
