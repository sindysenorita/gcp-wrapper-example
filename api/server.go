package api

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"net/http"
	"time"
)

type Config struct {
	Timeout          time.Duration
	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration
	Addr             string
}

type Server struct {
	cfg     Config
	logger  zerolog.Logger
	handler http.Handler
}

func NewServer(cfg Config, logger zerolog.Logger) *Server {
	h := chi.NewMux()
	s := &Server{
		cfg:    cfg,
		logger: logger,
	}
	h.Mount("/api", handlers(s))
	s.handler = h
	return s
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.cfg.Addr,
		ReadTimeout:  s.cfg.HTTPReadTimeout,
		WriteTimeout: s.cfg.HTTPWriteTimeout,
		Handler:      s.handler,
	}
	startErrChan := make(chan error, 1)
	shutdownErrChan := make(chan error, 1)
	go func(startChan, stopChan chan error) {
		select {
		case <-startChan:
		case <-ctx.Done():
			ctxShutdown, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			err := srv.Shutdown(ctxShutdown)
			if err != nil {
				shutdownErrChan <- fmt.Errorf("failed to shutdown server: %w", err)
			} else {
				shutdownErrChan <- nil
			}
		}
	}(startErrChan, shutdownErrChan)

	s.logger.Info().Msgf("serving http on: %s", s.cfg.Addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		s.logger.Error().Err(fmt.Errorf("failed to start ListenAndserve: %w", err))
		startErrChan <- err
		return err
	} else {
		startErrChan <- nil
		return <-shutdownErrChan
	}
}

func handlers(s *Server) http.Handler {
	h := chi.NewMux()
	h.Get("/healthcheck", HealthCheck(s.logger))
	return h
}

func HealthCheck(
	logger zerolog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug().Msg("debug")
		logger.Info().Msg("info")
		logger.Warn().Msg("warn")
		logger.Error().Err(fmt.Errorf("error")).Msg("error")
		w.WriteHeader(http.StatusOK)
	}
}
