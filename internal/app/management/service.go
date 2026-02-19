package management

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"connectrpc.com/connect"
	"github.com/thushan/olla/internal/adapter/registry/profile"
	managementv1 "github.com/thushan/olla/internal/gen/olla/management/v1"
	"github.com/thushan/olla/internal/logger"
	"github.com/thushan/olla/internal/version"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Service implements the ManagementService ConnectRPC handler.
type Service struct {
	logger         logger.StyledLogger
	profileFactory profile.ProfileFactory
	startTime      time.Time
	token          string
}

func NewService(logger logger.StyledLogger, factory profile.ProfileFactory, token string) *Service {
	return &Service{
		logger:         logger,
		profileFactory: factory,
		startTime:      time.Now(),
		token:          token,
	}
}

// ListProfiles implements managementv1connect.ManagementServiceHandler
func (s *Service) ListProfiles(ctx context.Context, req *connect.Request[managementv1.ListProfilesRequest]) (*connect.Response[managementv1.ListProfilesResponse], error) {
	if err := s.validateAuth(req.Header()); err != nil {
		return nil, err
	}

	profiles := s.profileFactory.GetAvailableProfiles()
	return connect.NewResponse(&managementv1.ListProfilesResponse{
		Profiles: profiles,
	}), nil
}

// ReloadProfiles implements managementv1connect.ManagementServiceHandler
func (s *Service) ReloadProfiles(ctx context.Context, req *connect.Request[managementv1.ReloadProfilesRequest]) (*connect.Response[managementv1.ReloadProfilesResponse], error) {
	if err := s.validateAuth(req.Header()); err != nil {
		return nil, err
	}

	err := s.profileFactory.ReloadProfiles()
	if err != nil {
		s.logger.Error("Failed to reload profiles via API", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reload profiles"))
	}

	s.logger.Info("Reloaded profiles via API")
	return connect.NewResponse(&managementv1.ReloadProfilesResponse{
		Status:  "success",
		Message: "Profiles reloaded successfully",
	}), nil
}

// GetStats implements managementv1connect.ManagementServiceHandler
func (s *Service) GetStats(ctx context.Context, req *connect.Request[managementv1.GetStatsRequest]) (*connect.Response[managementv1.GetStatsResponse], error) {
	if err := s.validateAuth(req.Header()); err != nil {
		return nil, err
	}

	return connect.NewResponse(&managementv1.GetStatsResponse{
		TotalRequests:  0, // TODO: Link with ports.StatsCollector
		ActiveRequests: int64(runtime.NumGoroutine()),
		ErrorCount:     0,
		StartTime:      timestamppb.New(s.startTime),
		Version:        version.Version,
	}), nil
}

func (s *Service) validateAuth(header http.Header) error {
	if s.token == "" {
		return nil
	}
	auth := header.Get("Authorization")
	if auth != "Bearer "+s.token {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token"))
	}
	return nil
}
