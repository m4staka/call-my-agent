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
	"call-my-agent/pkg/store"
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
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	cfgPath := fs.String("config", "config.json", "Path to config file")
	jsonOut := fs.Bool("json", false, "Print JSON output")
	limit := fs.Int("limit", 0, "Limit number of sessions in status output")
	_ = fs.Parse(args)

	_, srv := buildServiceWithOptions(*cfgPath, false)
	opts := service.StatusOptions{JSON: *jsonOut, Limit: *limit}
	if err := srv.Status(os.Stdout, opts); err != nil {
		fmt.Fprintf(os.Stderr, "status failed: %v\n", err)
		os.Exit(1)
	}
}

func buildService(args []string) (config.Config, *service.Service) {
	fs := flag.NewFlagSet("cma", flag.ExitOnError)
	cfgPath := fs.String("config", "config.json", "Path to config file")
	_ = fs.Parse(args)

	return buildServiceFromPath(*cfgPath)
}

func buildServiceFromPath(cfgPath string) (config.Config, *service.Service) {
	return buildServiceWithOptions(cfgPath, true)
}

func buildServiceWithOptions(cfgPath string, requireToken bool) (config.Config, *service.Service) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	var provider service.MessageProvider
	if requireToken {
		token := os.Getenv(tokenEnv)
		if token == "" {
			fmt.Fprintf(os.Stderr, "%s is required\n", tokenEnv)
			os.Exit(1)
		}
		provider = telegram.NewProvider(token, cfg.PollInterval())
	}

	ai := &codex.ExecClient{
		CommandTemplate: cfg.Inbound.Reply.Command,
		WorkingDir:      cfg.Inbound.Reply.Cwd,
	}
	srv, err := service.New(cfg, provider, ai, nil, store.NewFileSessionStore(cfg.SessionStorePath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "init service: %v\n", err)
		os.Exit(1)
	}
	return cfg, srv
}

func usage() {
	fmt.Println("Usage: cma <start|heartbeat|status> -config <path>")
}
