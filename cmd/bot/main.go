package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nickzhog/programmable-assistant/internal/config"
	"github.com/nickzhog/programmable-assistant/internal/fsm"
	"github.com/nickzhog/programmable-assistant/internal/handler"
	"github.com/nickzhog/programmable-assistant/internal/runner"
	"github.com/nickzhog/programmable-assistant/internal/store"
	tele "gopkg.in/telebot.v3"
)

func main() {
	configPath := flag.String("config", "build/config.toml", "Path to config file")
	dbPath := flag.String("db", "bot.db", "Path to bbolt database file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		slog.Error("failed to open store", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	telebot, err := tele.NewBot(tele.Settings{
		Token:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		slog.Error("failed to create bot", "error", err)
		os.Exit(1)
	}

	r := runner.New()
	fsmMgr := fsm.NewManager(db)

	h := handler.New(telebot, db, r, fsmMgr, cfg)
	h.Register()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.Info("bot started")
	go telebot.Start()

	<-ctx.Done()
	slog.Info("shutting down...")

	telebot.Stop()
	slog.Info("bot stopped")
}
