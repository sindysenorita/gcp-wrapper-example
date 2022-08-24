package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/pelletier/go-toml"
	"github.com/rs/zerolog"
	"github.com/sindysenorita/gcp-wrapper"
	"github.com/sindysenorita/gcp-wrapper-example/api"

	"log"

	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	rootCtx, cancel := context.WithCancel(context.Background())
	go func() {
		<-signalChan
		cancel()
	}()
	defer func() {
		cancel()
		os.Exit(0)
	}()

	var cfgPath string
	flag.StringVar(&cfgPath, "c", "config.toml", "path to config file")

	cfg, err := loadConfigFromFile(cfgPath)
	if err != nil {
		log.Fatalf("error load from config: %v", err)
	}
	apiCfg := api.Config{
		HTTPReadTimeout:  10 * time.Second,
		HTTPWriteTimeout: 10 * time.Second,
		Timeout:          10 * time.Second,
		Addr:             "0.0.0.0:8080",
	}

	var zlog zerolog.Logger
	logCfg := cfg.Logger
	switch logCfg.Output {
	case "gcp_logging":
		zerologGCP, err := gcp_wrapper.NewZerolog(
			rootCtx,
			cfg.Service.Name,
			cfg.Logger.Level,
			gcp_wrapper.GCPConfig{
				ProjectID:          cfg.GCP.ProjectID,
				ServiceAccountPath: cfg.GCP.ServiceAccountPath,
			},
		)
		if err != nil {
			log.Fatal(fmt.Errorf("failed to create zerolog gcp writer: %w", err))
		}
		defer zerologGCP.Flush()
		zlog = zerologGCP.Logger
	case "stdout":
		zlog, err = newStdoutZerolog(cfg.Service.Name, cfg.Logger)
		if err != nil {
			log.Fatal(fmt.Errorf("failed to create zerolog console: %w", err))
		}
	default:
		log.Fatal("invalid log output value")
	}

	s := api.NewServer(apiCfg, zlog)
	err = s.Run(rootCtx)
	if err != http.ErrServerClosed {
		fmt.Printf("error server: %v\n", err)
	}
}

type Config struct {
	Service ServiceConfig `toml:"service"`
	Logger  LoggerConfig  `toml:"logger"`
	GCP     GCP           `toml:"gcp"`
}

type ServiceConfig struct {
	Name string `toml:"name"`
}

type LoggerConfig struct {
	Level  string `toml:"level"`
	Output string `toml:"output"`
}

type GCP struct {
	ProjectID          string `toml:"project_id"`
	ServiceAccountPath string `toml:"service_account_path"`
}

func loadConfigFromFile(filePath string) (Config, error) {
	cfg := Config{}
	file, err := os.Open(filePath)
	if err != nil {
		return cfg, fmt.Errorf("error open config file: %w", err)
	}
	err = toml.NewDecoder(file).Decode(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("error parsing toml: %w", err)
	}
	return cfg, nil
}

func newStdoutZerolog(serviceName string, cfg LoggerConfig) (zerolog.Logger, error) {
	writer := zerolog.ConsoleWriter{Out: os.Stdout}
	logLvl, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return zerolog.Nop(), fmt.Errorf("invalid zerolog log level: %w", err)
	}
	zlog := zerolog.New(writer).
		Level(logLvl).
		With().
		Str("service", serviceName).
		Timestamp().
		Logger()
	return zlog, nil
}
