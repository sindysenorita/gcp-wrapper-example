package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/pelletier/go-toml"
	"github.com/rs/zerolog"
	"github.com/sindysenorita/gcp-wrapper-example/api"
	"github.com/sindysenorita/gcplogger"
	"io"

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
	logLvl, err := zerolog.ParseLevel(logCfg.Level)
	if err != nil {
		log.Fatalf("invalid zerolog log level: %v", err)
	}
	gcpConfig := gcplogger.GCPConfig{
		ProjectID:          cfg.GCP.ProjectID,
		ServiceAccountPath: cfg.GCP.ServiceAccountPath,
	}
	// setup zerolog
	switch logCfg.Output {
	case "gcp_logging":
		gcpWriter, err := gcplogger.NewZerolog(rootCtx, cfg.Service.Name, gcpConfig)
		if err != nil {
			log.Fatal(fmt.Errorf("failed to create zerolog gcp writer: %w", err))
		}
		zlog = setupZerolog(cfg.Service.Name, logLvl, gcpWriter)
		if err != nil {
			log.Fatal(fmt.Errorf("failed to create zerolog gcp writer: %w", err))
		}
	case "stdout":
		zlog = setupZerolog(cfg.Service.Name, logLvl, zerolog.ConsoleWriter{Out: os.Stdout})
	default:
		log.Fatal("invalid log output value")
	}

	// TODO: try example if stdlib log is customized into a leveled logging: https://www.honeybadger.io/blog/golang-logging/
	// can we convert log flags into structured logging?

	// setup stdlib log
	switch logCfg.Output {
	case "gcp_logging":
		gcpWriter, err := gcplogger.NewStdLog(rootCtx, cfg.Service.Name, gcpConfig)
		if err != nil {
			log.Fatal(fmt.Errorf("failed to create stdlog gcp writer: %w", err))
		}
		log.SetOutput(gcpWriter)
	case "stdout":
		log.SetOutput(os.Stdout)
	default:
		log.Fatal("invalid log output value")
	}

	s := api.NewServer(apiCfg, zlog)
	err = s.Run(rootCtx)
	if err != nil && err != http.ErrServerClosed {
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

func setupZerolog(serviceName string, level zerolog.Level, writer io.Writer) zerolog.Logger {
	return zerolog.New(writer).
		Level(level).
		With().
		Str("service", serviceName).
		Timestamp().
		Logger()
}
