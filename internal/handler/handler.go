package handler

import (
	"github.com/nickzhog/programmable-assistant/internal/config"
	"github.com/nickzhog/programmable-assistant/internal/fsm"
	"github.com/nickzhog/programmable-assistant/internal/runner"
	"github.com/nickzhog/programmable-assistant/internal/store"
	tele "gopkg.in/telebot.v3"
)

type Handler struct {
	bot    *tele.Bot
	store  store.Store
	runner *runner.Runner
	fsm    *fsm.Manager
	config *config.AppConfig
}

func New(bot *tele.Bot, store store.Store, runner *runner.Runner, fsmMgr *fsm.Manager, cfg *config.AppConfig) *Handler {
	return &Handler{
		bot:    bot,
		store:  store,
		runner: runner,
		fsm:    fsmMgr,
		config: cfg,
	}
}

func (h *Handler) AuthMiddleware() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Sender() == nil {
				return nil
			}
			if !h.config.IsAllowed(c.Sender().ID) {
				return nil
			}
			return next(c)
		}
	}
}

func (h *Handler) Register() {
	h.bot.Use(h.AuthMiddleware())

	h.bot.Handle("/start", h.HandleStart)
	h.bot.Handle("/files", h.HandleFiles)
	h.bot.Handle("/cd", h.HandleCd)
	h.bot.Handle("/sessions", h.HandleSessions)
	h.bot.Handle("/new_session", h.HandleNewSession)
	h.bot.Handle("/provider", h.HandleProvider)
	h.bot.Handle("/plan", h.HandlePlan)
	h.bot.Handle("/build", h.HandleBuild)
	h.bot.Handle(tele.OnDocument, h.HandleUpload)
	h.bot.Handle(tele.OnPhoto, h.HandlePhoto)

	h.bot.Handle(tele.OnCallback, h.HandleCallback)
}
