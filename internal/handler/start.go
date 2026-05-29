package handler

import (
	"fmt"
	"context"
	"strings"

	tele "gopkg.in/telebot.v3"
)

func (h *Handler) HandleStart(c tele.Context) error {
	userID := c.Sender().ID
	navPath, _ := h.store.GetNavPath(context.Background(), userID)
	if navPath == "" {
		navPath = "/"
	}

	activeID, _ := h.store.GetActiveSessionID(context.Background(), userID)
	var activeStr string
	if activeID != "" {
		sess, err := h.store.GetSession(context.Background(), activeID)
		if err == nil && sess != nil {
			activeStr = fmt.Sprintf("\U0001F4CB Active session: %s (%s)", sess.Name, sess.WorkDir)
		} else {
			activeStr = "No active session"
		}
	} else {
		activeStr = "No active session"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\U0001F4C2 Navigator: %s\n", navPath))
	sb.WriteString(activeStr)

	if activeID != "" {
		sess, err := h.store.GetSession(context.Background(), activeID)
		if err == nil && sess != nil {
			sb.WriteString(fmt.Sprintf("\n\n\U0001F9E0 Plan alias: %s", sess.ActivePlanAlias))
			if sess.ActivePlanAlias != "" {
				if alias, ok := h.config.OpenCode.Aliases[sess.ActivePlanAlias]; ok {
					sb.WriteString(fmt.Sprintf(" (%s/%s", alias.Provider, alias.Model))
					if alias.Thinking != "" {
						sb.WriteString(fmt.Sprintf(", thinking: %s", alias.Thinking))
					}
					sb.WriteString(")")
				}
			}
			sb.WriteString(fmt.Sprintf("\n\U0001F528 Build alias: %s", sess.ActiveBuildAlias))
			if sess.ActiveBuildAlias != "" {
				if alias, ok := h.config.OpenCode.Aliases[sess.ActiveBuildAlias]; ok {
					sb.WriteString(fmt.Sprintf(" (%s/%s", alias.Provider, alias.Model))
					if alias.Thinking != "" {
						sb.WriteString(fmt.Sprintf(", thinking: %s", alias.Thinking))
					}
					sb.WriteString(")")
				}
			}
		}
	}

	sb.WriteString("\n\n\u2B07 Select action:")

	return c.Send(sb.String(), &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{
				{Text: "\U0001F4C2 Files", Data: "cmd:files"},
				{Text: "\U0001F4CB Sessions", Data: "cmd:sessions"},
			},
			{
				{Text: "\U0001F916 Provider", Data: "cmd:provider"},
				{Text: "\U0001F4DD New Session", Data: "cmd:new_session"},
			},
		},
	})
}
