package handler

import (
	"fmt"
	"context"
	"path/filepath"

	tele "gopkg.in/telebot.v3"
)

func (h *Handler) HandleUpload(c tele.Context) error {
	userID := c.Sender().ID
	navPath, _ := h.store.GetNavPath(context.Background(), userID)
	if navPath == "" {
		return c.Send("No current directory set. Use /files or /cd first.")
	}

	doc := c.Message().Document
	if doc == nil {
		return c.Send("No document found in message.")
	}

	savePath := filepath.Join(navPath, doc.FileName)
	err := h.bot.Download(&doc.File, savePath)
	if err != nil {
		return c.Send(fmt.Sprintf("Upload failed: %v", err))
	}

	return c.Send(fmt.Sprintf("\u2705 Saved to: %s", savePath))
}

func (h *Handler) HandlePhoto(c tele.Context) error {
	userID := c.Sender().ID
	navPath, _ := h.store.GetNavPath(context.Background(), userID)
	if navPath == "" {
		return c.Send("No current directory set. Use /files or /cd first.")
	}

	photo := c.Message().Photo
	if photo == nil {
		return c.Send("No photo found in message.")
	}

	fileName := fmt.Sprintf("photo_%s.jpg", photo.FileID)
	savePath := filepath.Join(navPath, fileName)
	err := h.bot.Download(&photo.File, savePath)
	if err != nil {
		return c.Send(fmt.Sprintf("Upload failed: %v", err))
	}

	return c.Send(fmt.Sprintf("\u2705 Photo saved to: %s", savePath))
}
