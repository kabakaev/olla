package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/thushan/olla/internal/adapter/registry/profile"
	"github.com/thushan/olla/internal/app/management"
	"github.com/thushan/olla/internal/config"
	"github.com/thushan/olla/internal/gen/olla/management/v1/managementv1connect"
	"github.com/thushan/olla/internal/logger"
)

type ManagementService struct {
	config     *config.ManagementConfig
	logger     logger.StyledLogger
	server     *http.Server
	factory    profile.ProfileFactory
	service    *management.Service
}

func NewManagementService(cfg *config.ManagementConfig, logger logger.StyledLogger, factory profile.ProfileFactory) *ManagementService {
	return &ManagementService{
		config:  cfg,
		logger:  logger,
		factory: factory,
	}
}

func (s *ManagementService) Name() string {
	return "management"
}

func (s *ManagementService) Start(ctx context.Context) error {
	s.logger.Info("Initialising Management service", "port", s.config.Port)

	s.service = management.NewService(s.logger, s.factory, s.config.Token)
	
	mux := http.NewServeMux()
	path, handler := managementv1connect.NewManagementServiceHandler(s.service)
	mux.Handle(path, handler)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		s.logger.Info("Management server listening", "address", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("Management server error", "error", err)
		}
	}()

	return nil
}

func (s *ManagementService) Stop(ctx context.Context) error {
	s.logger.Info(" Stopping Management server...")
	if s.server != nil {
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Error("Management server shutdown error", "error", err)
			return err
		}
	}
	s.logger.ResetLine()
	s.logger.InfoWithStatus("Stopping Management server", "OK")
	return nil
}

func (s *ManagementService) Dependencies() []string {
	return []string{}
}
