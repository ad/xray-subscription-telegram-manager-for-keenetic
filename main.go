package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"xray-telegram-manager/config"
	"xray-telegram-manager/logger"
	"xray-telegram-manager/service"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

func main() {
	fmt.Printf("Xray Telegram Manager v%s (built %s with %s)\n", Version, BuildTime, GoVersion)

	configPath := "/opt/etc/xray-manager/config.json"

	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logLevel := logger.ParseLogLevel(cfg.LogLevel)

	// Create logs directory if it doesn't exist
	logDir := "/opt/etc/xray-manager/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
	}

	// Try to create file logger, fallback to stdout
	logFile := "/opt/etc/xray-manager/logs/app.log"
	log, err := logger.NewFileLogger(logLevel, logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create file logger, using stdout: %v\n", err)
		log = logger.NewLogger(logLevel, os.Stdout)
	}

	svc, err := service.NewService(cfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create service: %v\n", err)
		os.Exit(1)
	}

	if err := svc.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start service: %v\n", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	log.Info("Service started. Press Ctrl+C to stop or send SIGHUP to reload")

	for {
		sig := <-sigChan

		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Info("Received shutdown signal (%v), shutting down gracefully...", sig)
			if err := svc.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
				os.Exit(1)
			}
			log.Info("Service shutdown completed")
			return

		case syscall.SIGHUP:
			log.Info("Received reload signal (SIGHUP), reloading configuration...")
			if err := svc.Reload(); err != nil {
				log.Error("Failed to reload configuration: %v", err)
			}
		}
	}
}
