package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"call-my-agent/pkg/codex"
	"call-my-agent/pkg/config"
	"call-my-agent/pkg/service"
	"call-my-agent/pkg/telegram"
)

const tokenEnv = "TELEGRAM_BOT_TOKEN"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "start":
		start(os.Args[2:])
	case "heartbeat":
		heartbeat(os.Args[2:])
	case "status":
		status(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func start(args []string) {
	cfg, srv := buildService(args)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "start failed: %v\n", err)
		os.Exit(1)
	}
	_ = cfg
}

func heartbeat(args []string) {
	_, srv := buildService(args)
	if err := srv.RunHeartbeat(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "heartbeat failed: %v\n", err)
		os.Exit(1)
	}
}

func status(args []string) {
	_, srv := buildService(args)
	srv.Status(os.Stdout)
}

func buildService(args []string) (config.Config, *service.Service) {
	fs := flag.NewFlagSet("cma", flag.ExitOnError)
	cfgPath := fs.String("config", "config.json", "Path to config file")
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	token := os.Getenv(tokenEnv)
	if token == "" {
		fmt.Fprintf(os.Stderr, "%s is required\n", tokenEnv)
		os.Exit(1)
	}

	provider := telegram.NewProvider(token, cfg.PollInterval())
	ai := codex.ExecClient{CommandTemplate: cfg.Inbound.Reply.Command}
	srv := service.New(cfg, provider, ai, nil)
	return cfg, srv
}

func usage() {
	fmt.Println("Usage: cma <start|heartbeat|status> -config <path>")
}
