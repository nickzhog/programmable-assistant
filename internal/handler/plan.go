package handler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nickzhog/programmable-assistant/internal/runner"
	"github.com/nickzhog/programmable-assistant/internal/session"
	tele "gopkg.in/telebot.v3"
)

const (
	streamBatchInterval = 2 * time.Second
	maxMessageLength    = 4096
)

func (h *Handler) HandlePlan(c tele.Context) error {
	prompt := c.Message().Payload
	if prompt == "" {
		return c.Send("Usage: /plan <prompt>")
	}
	return h.runMode(c, "plan", prompt)
}

func (h *Handler) HandleBuild(c tele.Context) error {
	prompt := c.Message().Payload
	if prompt == "" {
		return c.Send("Usage: /build <prompt>")
	}
	return h.runMode(c, "build", prompt)
}

func (h *Handler) runMode(c tele.Context, mode string, prompt string) error {
	userID := c.Sender().ID

	activeID, err := h.store.GetActiveSessionID(context.Background(), userID)
	if err != nil || activeID == "" {
		return c.Send("No active session. Create one with /new_session <name>")
	}

	sess, err := h.store.GetSession(context.Background(), activeID)
	if err != nil {
		return c.Send("Session not found.")
	}

	if sess.Status == session.StatusRunning {
		return c.Send("\u26A0 Session is already running.")
	}

	var aliasKey string
	if mode == "plan" {
		aliasKey = sess.ActivePlanAlias
	} else {
		aliasKey = sess.ActiveBuildAlias
	}

	if aliasKey == "" {
		return c.Send(fmt.Sprintf("No %s alias configured. Use /provider.", mode))
	}

	alias, ok := h.config.OpenCode.Aliases[aliasKey]
	if !ok {
		return c.Send(fmt.Sprintf("Alias %s not found in config.", aliasKey))
	}

	sess.Status = session.StatusRunning
	if err := h.store.SaveSession(context.Background(), sess); err != nil {
		return c.Send(fmt.Sprintf("Error: %v", err))
	}

	opts := runner.RunOptions{
		WorkDir:  sess.WorkDir,
		Alias:    alias,
		AliasKey: aliasKey,
		Mode:     mode,
		Prompt:   prompt,
	}

	modeLabel := "Plan"
	if mode == "build" {
		modeLabel = "Build"
	}

	prefix := fmt.Sprintf("[Mode: %s | Provider: %s | Model: %s]\n",
		modeLabel, alias.Provider, alias.Model)

	msg, err := h.bot.Send(c.Recipient(), prefix+"Starting...", &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{{Text: "\U0001F6D1 Abort", Data: fmt.Sprintf("abort:%s", sess.ID)}},
		},
	})
	if err != nil {
		sess.Status = session.StatusIdle
		h.store.SaveSession(context.Background(), sess)
		return c.Send(fmt.Sprintf("Error sending message: %v", err))
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := h.runner.Run(runCtx, sess.ID, opts)
	if err != nil {
		sess.Status = session.StatusIdle
		h.store.SaveSession(context.Background(), sess)
		h.bot.Edit(msg, prefix+fmt.Sprintf("Error: %v", err))
		return nil
	}

	go h.streamOutput(runCtx, cancel, ch, msg, sess, prefix, mode)

	return nil
}

func (h *Handler) streamOutput(ctx context.Context, cancel context.CancelFunc, ch <-chan runner.OutputChunk,
	msg *tele.Message, sess *session.Session, prefix string, mode string) {

	var mu sync.Mutex
	var buffer strings.Builder
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case chunk, ok := <-ch:
				if !ok {
					return
				}
				mu.Lock()
				if chunk.Err != nil {
					buffer.WriteString(fmt.Sprintf("\n[Error: %v]", chunk.Err))
				} else if chunk.Text != "" {
					buffer.WriteString(chunk.Text + "\n")
				}

				content := prefix + buffer.String()
				if len(content) > maxMessageLength {
					content = truncateContent(content, maxMessageLength)
				}

				h.bot.Edit(msg, content, &tele.ReplyMarkup{
					InlineKeyboard: [][]tele.InlineButton{
						{{Text: "\U0001F6D1 Abort", Data: fmt.Sprintf("abort:%s", sess.ID)}},
					},
				})
				mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	<-done

	mu.Lock()
	defer mu.Unlock()

	sess.Status = session.StatusIdle
	h.store.SaveSession(context.Background(), sess)

	finalContent := prefix + buffer.String()
	if len(finalContent) > maxMessageLength-200 {
		finalContent = truncateContent(finalContent, maxMessageLength-200)
	}

	finalMessage := finalContent + fmt.Sprintf("\n\n\u2705 %s complete.", mode)

	var buttons [][]tele.InlineButton
	if mode == "plan" {
		buttons = append(buttons, []tele.InlineButton{{
			Text: "\u26A1 Create Build session from plan",
			Data: fmt.Sprintf("build_from_plan:%s:%d", sess.ID, msg.ID),
		}})
	}

	h.bot.Edit(msg, finalMessage, &tele.ReplyMarkup{InlineKeyboard: buttons})
}

func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cutoff := maxLen - 30
	if cutoff < 0 {
		cutoff = 0
	}
	return s[:cutoff] + "\n\n[...truncated]"
}
