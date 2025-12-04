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

type command struct {
	name        string
	description string
	run         func(args []string) error
}

var commands = []command{
	{name: "start", description: "Start the Telegram bot and process incoming messages", run: runStart},
	{name: "heartbeat", description: "Trigger a single heartbeat pass", run: runHeartbeat},
	{name: "status", description: "Print session status information", run: runStatus},
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("no command specified")
	}

	if isHelpArg(args[0]) {
		usage()
		return nil
	}

	cmd, ok := lookupCommand(args[0])
	if !ok {
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}

	return cmd.run(args[1:])
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func lookupCommand(name string) (command, bool) {
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd, true
		}
	}
	return command{}, false
}

func runStart(args []string) error {
	srv, err := buildService(args)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

func runHeartbeat(args []string) error {
	srv, err := buildService(args)
	if err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}
	if err := srv.RunHeartbeat(context.Background()); err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	cfgPath := fs.String("config", "config.json", "Path to config file")
	jsonOut := fs.Bool("json", false, "Print JSON output")
	limit := fs.Int("limit", 0, "Limit number of sessions in status output")
	_ = fs.Parse(args)

	_, srv, err := buildServiceWithOptions(*cfgPath, false)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	opts := service.StatusOptions{JSON: *jsonOut, Limit: *limit}
	if err := srv.Status(os.Stdout, opts); err != nil {
		return fmt.Errorf("status: %w", err)
	}
	return nil
}

func buildService(args []string) (*service.Service, error) {
	fs := flag.NewFlagSet("cma", flag.ExitOnError)
	cfgPath := fs.String("config", "config.json", "Path to config file")
	_ = fs.Parse(args)

	_, srv, err := buildServiceFromPath(*cfgPath)
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func buildServiceFromPath(cfgPath string) (config.Config, *service.Service, error) {
	return buildServiceWithOptions(cfgPath, true)
}

func buildServiceWithOptions(cfgPath string, requireToken bool) (config.Config, *service.Service, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("load config: %w", err)
	}

	var provider service.MessageProvider
	if requireToken {
		token := os.Getenv(tokenEnv)
		if token == "" {
			return config.Config{}, nil, fmt.Errorf("%s is required", tokenEnv)
		}
		provider = telegram.NewProvider(token, cfg.PollInterval())
	}

	ai := &codex.ExecClient{
		CommandTemplate: cfg.Inbound.Reply.Command,
		WorkingDir:      cfg.Inbound.Reply.Cwd,
	}
	srv, err := service.New(cfg, provider, ai, nil, store.NewFileSessionStore(cfg.SessionStorePath))
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("init service: %w", err)
	}
	return cfg, srv, nil
}

func usage() {
	fmt.Println("Usage: cma <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	for _, cmd := range commands {
		fmt.Printf("  %-10s %s\n", cmd.name, cmd.description)
	}
	fmt.Println()
	fmt.Println("Global options:")
	fmt.Println("  -config string   Path to config file (default \"config.json\")")
}
