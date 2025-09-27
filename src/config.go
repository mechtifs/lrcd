package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"lrcd/providers"
	"lrcd/publishers"

	"go.yaml.in/yaml/v4"
)

type FetchMode int

const (
	FetchModeFallback FetchMode = iota
	FetchModeFastest
)

type rawProvider struct {
	ID string `yaml:"id"`
}

type rawPublisher struct {
	ID      string    `yaml:"id"`
	Offset  int       `yaml:"offset"`
	Options yaml.Node `yaml:"options"`
}

type rawConfig struct {
	LogLevel     string          `yaml:"log_level"`
	FetchMode    string          `yaml:"fetch_mode"`
	FetchTimeout int             `yaml:"fetch_timeout"`
	ShowTitle    bool            `yaml:"show_title"`
	UseCache     bool            `yaml:"use_cache"`
	Filters      []string        `yaml:"filters"`
	URLBlacklist []string        `yaml:"url_blacklist"`
	Providers    []*rawProvider  `yaml:"providers"`
	Publishers   []*rawPublisher `yaml:"publishers"`
}

func CreateProvider(p *rawProvider) (providers.Provider, error) {
	var provider providers.Provider
	switch p.ID {
	case providers.MXMProviderID:
		provider = providers.NewMXMProvider()
	case providers.LRCLIBProviderID:
		provider = providers.NewLRCLIBProvider()
	case providers.KugouProviderID:
		provider = providers.NewKugouProvider()
	case providers.NCMProviderID:
		provider = providers.NewNCMProvider()
	case providers.KuwoProviderID:
		provider = providers.NewKuwoProvider()
	default:
		return nil, fmt.Errorf("unknown provider %q", p.ID)
	}
	return provider, nil
}

func CreatePublisher(p *rawPublisher) (publishers.Publisher, error) {
	var publisher publishers.Publisher
	switch p.ID {
	case publishers.FilePublisherID:
		opt := &publishers.FilePublisherOptions{}
		err := p.Options.Decode(opt)
		if err != nil {
			return nil, err
		}
		publisher = publishers.NewFilePublisher(opt)
	case publishers.HTTPPublisherID:
		opt := &publishers.HTTPPublisherOptions{}
		err := p.Options.Decode(opt)
		if err != nil {
			return nil, err
		}
		publisher = publishers.NewHTTPPublisher(opt)
	case publishers.WebSocketPublisherID:
		opt := &publishers.WebSocketPublisherOptions{}
		err := p.Options.Decode(opt)
		if err != nil {
			return nil, err
		}
		publisher = publishers.NewWebSocketPublisher(opt)
	case publishers.DBusPublisherID:
		opt := &publishers.DBusPublisherOptions{}
		err := p.Options.Decode(opt)
		if err != nil {
			return nil, err
		}
		publisher = publishers.NewDBusPublisher(opt)
	default:
		return nil, fmt.Errorf("unknown publisher %q", p.ID)
	}
	return publisher, nil
}

type Config struct {
	LogLevel     slog.Level
	FetchMode    FetchMode
	FetchTimeout int
	ShowTitle    bool
	UseCache     bool
	Filters      []string
	URLBlacklist []string
	Providers    []*ProviderEntry
	Publishers   []*PublisherEntry
}

func ParseConfig(path string) (*Config, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw rawConfig
	err = yaml.Unmarshal(buf, &raw)
	if err != nil {
		return nil, err
	}

	providers := make([]*ProviderEntry, 0, len(raw.Providers))
	for _, p := range raw.Providers {
		provider, err := CreateProvider(p)
		if err != nil {
			log.Println(err)
			continue
		}
		providers = append(providers, NewProviderEntry(provider))
	}

	publishers := make([]*PublisherEntry, 0, len(raw.Publishers))
	for _, p := range raw.Publishers {
		publisher, err := CreatePublisher(p)
		if err != nil {
			log.Println(err)
			continue
		}
		publishers = append(publishers, NewPublisherEntry(publisher, p.Offset))
	}

	var fetchMode FetchMode
	switch raw.FetchMode {
	case "fallback", "":
		fetchMode = FetchModeFallback
	case "fastest":
		fetchMode = FetchModeFastest
	default:
		return nil, fmt.Errorf("unknown fetch mode %q", raw.FetchMode)
	}

	var logLevel slog.Level
	switch raw.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info", "":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("unknown log level %q", raw.LogLevel)
	}

	config := &Config{
		LogLevel:     logLevel,
		FetchMode:    fetchMode,
		FetchTimeout: raw.FetchTimeout,
		ShowTitle:    raw.ShowTitle,
		UseCache:     raw.UseCache,
		Filters:      raw.Filters,
		URLBlacklist: raw.URLBlacklist,
		Providers:    providers,
		Publishers:   publishers,
	}

	return config, nil
}
