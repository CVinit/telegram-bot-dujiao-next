package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/v/telegram-bot-dujiao-next/internal/api"
	"github.com/v/telegram-bot-dujiao-next/internal/bot"
	"github.com/v/telegram-bot-dujiao-next/internal/config"
	"github.com/v/telegram-bot-dujiao-next/internal/handler"
	"github.com/v/telegram-bot-dujiao-next/internal/state"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败：%v", err)
	}

	if cfg.Telegram.BotToken == "" {
		log.Fatal("未配置 Telegram Bot Token")
	}
	if cfg.Dujiao.AdminUsername == "" || cfg.Dujiao.AdminPassword == "" {
		log.Fatal("未配置 dujiao-next 管理员账号密码")
	}

	apiClient := api.NewClient(cfg.Dujiao)
	if err := apiClient.EnsureToken(context.Background()); err != nil {
		log.Fatalf("登录 dujiao-next 失败：%v", err)
	}

	stateMgr := state.NewManager(5 * time.Minute)

	b, err := bot.New(cfg, apiClient, stateMgr)
	if err != nil {
		log.Fatalf("创建 Bot 失败：%v", err)
	}

	// Stock alert checker
	checker := handler.NewStockAlertChecker(apiClient, cfg, b)
	ctx, cancel := context.WithCancel(context.Background())
	go checker.Run(ctx)

	// Graceful shutdown
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		log.Println("Shutting down...")
		cancel()
		b.Stop()
	}()

	b.Start(ctx)
}
