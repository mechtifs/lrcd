package main

import (
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"lrcd/models"

	"github.com/godbus/dbus/v5"
)

func main() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Fatal("failed to connect to session bus:", err)
	}

	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal("failed to get user config directory:", err)
	}
	configDir := filepath.Join(userConfigDir, "lrcd")
	err = os.MkdirAll(configDir, 0o755)
	if err != nil {
		log.Fatal("failed to create config directory:", err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	config, err := ParseConfig(configPath)
	if err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}
	cacheDir := ""
	if config.UseCache {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			log.Fatal("failed to get user cache directory:", err)
		}
		cacheDir = filepath.Join(userCacheDir, "lrcd")
		err = os.MkdirAll(cacheDir, 0o755)
		if err != nil {
			log.Fatal("failed to create cache directory:", err)
		}
	}

	slog.SetLogLoggerLevel(config.LogLevel)

	propsCh := make(chan models.MPRISProperties, 8)
	controller := NewController(&ControllerOptions{
		providers:    config.Providers,
		publishers:   config.Publishers,
		fetchMode:    config.FetchMode,
		fetchTimeout: config.FetchTimeout,
		showTitle:    config.ShowTitle,
		filters:      config.Filters,
		urlBlacklist: config.URLBlacklist,
		propsCh:      propsCh,
		cacheDir:     cacheDir,
	})
	mpris := NewMPRIS(propsCh, conn)
	go mpris.Serve()
	go controller.Serve()

	sCh := make(chan os.Signal, 1)
	signal.Notify(sCh, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-sCh, "received, shutting down...")
	conn.Close()
	mpris.Exit()
	controller.Exit()
}
