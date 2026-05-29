package handler

import (
	"fmt"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/nickzhog/programmable-assistant/internal/navigator"
	tele "gopkg.in/telebot.v3"
)

func (h *Handler) HandleFiles(c tele.Context) error {
	userID := c.Sender().ID

	path, _ := h.store.GetNavPath(context.Background(), userID)
	if path == "" {
		path = os.Getenv("HOME")
		if path == "" {
			path = "/"
		}
		h.store.SetNavPath(context.Background(), userID, path)
	}

	return h.renderNavPage(c, userID, path, 0)
}

func (h *Handler) HandleCd(c tele.Context) error {
	userID := c.Sender().ID
	target := strings.TrimSpace(c.Message().Payload)
	if target == "" {
		return c.Send("Usage: /cd <path>")
	}

	absPath, err := filepath.Abs(target)
	if err != nil {
		return c.Send(fmt.Sprintf("Invalid path: %v", err))
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.Send(fmt.Sprintf("Path not found: %s", absPath))
		}
		return c.Send(fmt.Sprintf("Error: %v", err))
	}
	if !info.IsDir() {
		return c.Send(fmt.Sprintf("Not a directory: %s", absPath))
	}

	h.store.SetNavPath(context.Background(), userID, absPath)
	return h.renderNavPage(c, userID, absPath, 0)
}

func (h *Handler) HandleCallback(c tele.Context) error {
	data := c.Callback().Data
	userID := c.Sender().ID

	if fsmState, _ := h.fsm.GetState(context.Background(), userID); fsmState != nil {
		if strings.HasPrefix(data, "fsm:provider:") {
			return h.handleProviderCallback(c, data)
		}
	}

	switch {
	case strings.HasPrefix(data, "nav:dir:"):
		return h.handleNavDir(c, data, userID)
	case strings.HasPrefix(data, "nav:up:"):
		return h.handleNavUp(c, data, userID)
	case strings.HasPrefix(data, "nav:page:"):
		return h.handleNavPage(c, data, userID)
	case strings.HasPrefix(data, "nav:file:"):
		return h.handleNavFile(c, data, userID)
	case strings.HasPrefix(data, "nav:download:"):
		return h.handleFileDownload(c, data)
	case strings.HasPrefix(data, "sess:switch:"):
		return h.handleSessionSwitch(c, data)
	case strings.HasPrefix(data, "sess:delete:"):
		return h.handleSessionDelete(c, data)
	case strings.HasPrefix(data, "abort:"):
		return h.handleAbort(c, data)
	case strings.HasPrefix(data, "build_from_plan:"):
		return h.handleBuildFromPlan(c, data)
	}

	return c.Respond(&tele.CallbackResponse{Text: "Unknown action"})
}

func (h *Handler) renderNavPage(c tele.Context, userID int64, path string, page int) error {
	pg, err := navigator.ListDir(path, page, navigator.DefaultPageSize)
	if err != nil {
		return c.Send(fmt.Sprintf("Error: %v", err))
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("\U0001F4C2 %s", pg.Path))

	var rows [][]tele.InlineButton

	for _, entry := range pg.Entries {
		entryPath := filepath.Join(pg.Path, entry.Name)

		var prefix string
		var cbData string
		if entry.IsDir {
			prefix = "\U0001F4C2"
			cbData = fmt.Sprintf("nav:dir:%s", h.storePath(entryPath))
		} else {
			prefix = "\U0001F4C4"
			cbData = fmt.Sprintf("nav:file:%s", h.storePath(entryPath))
		}

		rows = append(rows, []tele.InlineButton{{
			Text: fmt.Sprintf("%s %s", prefix, entry.Name),
			Data: cbData,
		}})
	}

	navigation := h.buildNavRow(pg)
	if len(navigation) > 0 {
		rows = append(rows, navigation)
	}

	_, err = h.bot.Send(c.Recipient(), strings.Join(lines, "\n"), &tele.ReplyMarkup{
		InlineKeyboard: rows,
	})
	return err
}

func (h *Handler) buildNavRow(pg *navigator.Page) []tele.InlineButton {
	var buttons []tele.InlineButton

	parent := filepath.Dir(pg.Path)
	if pg.Path != "/" && parent != pg.Path {
		buttons = append(buttons, tele.InlineButton{
			Text: "\u2B05 Up",
			Data: fmt.Sprintf("nav:up:%s", h.storePath(parent)),
		})
	}

	if pg.PageNum > 0 {
		buttons = append(buttons, tele.InlineButton{
			Text: "\u25C0 Prev",
			Data: fmt.Sprintf("nav:page:%s:%d", h.storePath(pg.Path), pg.PageNum-1),
		})
	}

	if pg.PageNum < pg.TotalPages-1 {
		buttons = append(buttons, tele.InlineButton{
			Text: "Next \u25B6",
			Data: fmt.Sprintf("nav:page:%s:%d", h.storePath(pg.Path), pg.PageNum+1),
		})
	}

	return buttons
}

func (h *Handler) storePath(path string) string {
	ref := fmt.Sprintf("p_%d", len(path)) // short compressed key
	h.store.SetCallbackRef(context.Background(), ref, path)
	return ref
}

func (h *Handler) resolvePath(c tele.Context, data string) (string, error) {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid callback data")
	}
	ref := parts[1]
	path, err := h.store.GetCallbackRef(context.Background(), ref)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("path ref not found")
	}
	return path, nil
}

func (h *Handler) handleNavDir(c tele.Context, data string, userID int64) error {
	path, err := h.resolvePath(c, data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid path"})
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid directory"})
	}

	h.store.SetNavPath(context.Background(), userID, path)
	h.bot.Delete(c.Message())
	return h.renderNavPage(c, userID, path, 0)
}

func (h *Handler) handleNavUp(c tele.Context, data string, userID int64) error {
	path, err := h.resolvePath(c, data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid path"})
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid directory"})
	}

	h.store.SetNavPath(context.Background(), userID, path)
	h.bot.Delete(c.Message())
	return h.renderNavPage(c, userID, path, 0)
}

func (h *Handler) handleNavPage(c tele.Context, data string, userID int64) error {
	parts := strings.SplitN(data, ":", 4)
	if len(parts) < 4 {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid data"})
	}
	ref := parts[2]
	path, err := h.store.GetCallbackRef(context.Background(), ref)
	if err != nil || path == "" {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid path"})
	}

	var page int
	fmt.Sscanf(parts[3], "%d", &page)

	h.bot.Delete(c.Message())
	return h.renderNavPage(c, userID, path, page)
}

func (h *Handler) handleNavFile(c tele.Context, data string, userID int64) error {
	path, err := h.resolvePath(c, data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid path"})
	}

	info, err := os.Stat(path)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Error: %v", err)})
	}
	if info.IsDir() {
		return c.Respond(&tele.CallbackResponse{Text: "Use nav:dir for directories"})
	}

	return h.sendFileOptions(c, path, info)
}

func (h *Handler) sendFileOptions(c tele.Context, path string, info os.FileInfo) error {
	sizeMB := float64(info.Size()) / (1024 * 1024)
	var text strings.Builder
	text.WriteString(fmt.Sprintf("\U0001F4C4 %s\n", filepath.Base(path)))
	text.WriteString(fmt.Sprintf("Size: %.2f MB\n", sizeMB))

	var rows [][]tele.InlineButton

	if info.Size() > 50*1024*1024 {
		text.WriteString("\u26A0 File exceeds 50 MB Telegram limit")
	} else {
		rows = append(rows, []tele.InlineButton{{
			Text: "\U0001F4E5 Download",
			Data: fmt.Sprintf("nav:download:%s", h.storePath(path)),
		}})
	}

	navPath, _ := h.store.GetNavPath(context.Background(), c.Sender().ID)
	rows = append(rows, []tele.InlineButton{{
		Text: "\u21A9 Back",
		Data: fmt.Sprintf("nav:dir:%s", h.storePath(navPath)),
	}})

	return c.Send(text.String(), &tele.ReplyMarkup{InlineKeyboard: rows})
}

func (h *Handler) handleFileDownload(c tele.Context, data string) error {
	path, err := h.resolvePath(c, data)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid path"})
	}

	info, err := os.Stat(path)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Error: %v", err)})
	}

	return c.Reply(&tele.Document{
		File:     tele.FromDisk(path),
		FileName: info.Name(),
	})
}
