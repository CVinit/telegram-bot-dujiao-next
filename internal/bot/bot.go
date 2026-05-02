package bot

import (
	"context"
	"fmt"
	"log"

	tele "gopkg.in/telebot.v3"

	"github.com/v/telegram-bot-dujiao-next/internal/api"
	"github.com/v/telegram-bot-dujiao-next/internal/config"
	"github.com/v/telegram-bot-dujiao-next/internal/handler"
	"github.com/v/telegram-bot-dujiao-next/internal/state"
)

type Bot struct {
	tele   *tele.Bot
	cfg    *config.Config
	api    *api.Client
	state  *state.Manager
	handler *handler.Handler
	cancel context.CancelFunc
}

func New(cfg *config.Config, apiClient *api.Client, stateMgr *state.Manager) (*Bot, error) {
	pref := tele.Settings{
		Token:  cfg.Telegram.BotToken,
		Poller: &tele.LongPoller{Timeout: 10},
	}

	t, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	h := handler.New(apiClient, stateMgr, cfg)

	b := &Bot{
		tele:    t,
		cfg:     cfg,
		api:     apiClient,
		state:   stateMgr,
		handler: h,
	}

	b.tele.Use(b.whitelistMiddleware)
	b.registerHandlers()

	return b, nil
}

func (b *Bot) whitelistMiddleware(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if !b.cfg.IsAllowedUser(c.Sender().ID) {
			return c.Reply("无权使用此 Bot")
		}
		return next(c)
	}
}

func (b *Bot) registerHandlers() {
	b.tele.Handle("/start", b.handler.OnStart)
	b.tele.Handle("/sales", b.handler.OnSales)
	b.tele.Handle("/orders", b.handler.OnOrders)
	b.tele.Handle("/cards", b.handler.OnCards)
	b.tele.Handle("/fulfill", b.handler.OnFulfill)
	b.tele.Handle("/stock", b.handler.OnStock)
	b.tele.Handle("/cancel", b.handler.OnCancel)

	b.tele.Handle(tele.OnCallback, b.handler.OnCallback)
	b.tele.Handle(tele.OnText, b.handler.OnText)
	b.tele.Handle(tele.OnDocument, b.handler.OnDocument)
}

func (b *Bot) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	b.cancel = cancel

	b.api.StartRefreshLoop(ctx, b.cfg.Dujiao.JWTRefreshInterval)

	log.Println("Bot started")
	b.tele.Start()
}

func (b *Bot) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	b.state.Stop()
	b.tele.Stop()
}

func (b *Bot) Send(chatID int64, text string) error {
	_, err := b.tele.Send(tele.ChatID(chatID), text)
	return err
}
