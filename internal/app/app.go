package app

import (
	"context"
	"fmt"
	"time"

	"github.com/thushan/olla/internal/adapter/registry/profile"
	"github.com/thushan/olla/internal/app/services"
	"github.com/thushan/olla/internal/config"
	"github.com/thushan/olla/internal/logger"
)

// CreateAndStartServiceManager initialises the service orchestration layer, establishing
// the dependency graph and bootstrapping all services in the correct order. This ensures
// health checks and discovery run before the proxy accepts traffic, maintaining the
// original startup behaviour where endpoints are validated immediately.
func CreateAndStartServiceManager(ctx context.Context, cfg *config.Config, logger logger.StyledLogger) (*services.ServiceManager, error) {
	startTime := time.Now()
	defer func() {
		logger.Debug("Service manager startup completed", "duration", time.Since(startTime))
	}()

	manager := services.NewServiceManager(logger)

	if err := registerServices(manager, cfg, logger); err != nil {
		return nil, fmt.Errorf("failed to register services: %w", err)
	}

	// Services are started in dependency order, with stats initialised first,
	// followed by security and discovery (which runs initial health checks),
	// then proxy and finally HTTP. This preserves the critical startup sequence.
	if err := manager.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start services: %w", err)
	}

	return manager, nil
}

// registerServices establishes the service dependency graph using a two-phase approach:
// registration followed by dependency injection. This pattern allows circular dependency
// resolution and ensures services can reference each other without initialisation races.
func registerServices(manager *services.ServiceManager, cfg *config.Config, logger logger.StyledLogger) error {
	profileFactory, err := initProfileFactory(logger)
	if err != nil {
		return err
	}

	if err := performRegistration(manager, cfg, logger, profileFactory); err != nil {
		return err
	}

	performWiring(manager.GetRegistry())
	return nil
}

func initProfileFactory(logger logger.StyledLogger) (*profile.Factory, error) {
	profileFactory, err := profile.NewFactoryWithDefaults()
	if err == nil {
		return profileFactory, nil
	}

	// Fallback to empty factory
	profileFactory, err = profile.NewFactory("")
	if err != nil {
		return nil, fmt.Errorf("failed to create profile factory: %w", err)
	}

	logger.Warn("Failed to load profiles from default location, using built-in", "error", err)
	return profileFactory, nil
}

func performRegistration(manager *services.ServiceManager, cfg *config.Config, logger logger.StyledLogger, profileFactory *profile.Factory) error {
	statsService := services.NewStatsService(logger)
	if err := manager.Register(statsService); err != nil {
		return fmt.Errorf("failed to register stats service: %w", err)
	}

	// Register Management Service
	managementService := services.NewManagementService(
		&cfg.Management,
		logger,
		profileFactory,
	)
	if err := manager.Register(managementService); err != nil {
		return fmt.Errorf("failed to register management service: %w", err)
	}

	// Security service requires stats collector, but we defer resolution to avoid
	// accessing uninitialised components. The nil parameter is intentional.
	securityService := services.NewSecurityService(
		&cfg.Server,
		nil,
		logger,
	)
	if err := manager.Register(securityService); err != nil {
		return fmt.Errorf("failed to register security service: %w", err)
	}

	discoveryService := services.NewDiscoveryService(
		&cfg.Discovery,
		&cfg.ModelRegistry,
		nil,
		logger,
		profileFactory,
	)
	if err := manager.Register(discoveryService); err != nil {
		return fmt.Errorf("failed to register discovery service: %w", err)
	}

	proxyService := services.NewProxyServiceWrapper(
		&cfg.Proxy,
		logger,
	)
	if err := manager.Register(proxyService); err != nil {
		return fmt.Errorf("failed to register proxy service: %w", err)
	}

	httpService := services.NewHTTPService(
		&cfg.Server,
		cfg,
		logger,
		profileFactory,
	)
	if err := manager.Register(httpService); err != nil {
		return fmt.Errorf("failed to register HTTP service: %w", err)
	}

	return nil
}

func performWiring(registry *services.ServiceRegistry) {
	stats, _ := registry.GetStats()
	security, _ := registry.GetSecurity()
	discovery, _ := registry.GetDiscovery()
	proxy, _ := registry.GetProxy()
	http, _ := registry.GetHTTP()

	if security != nil && stats != nil {
		security.SetStatsService(stats)
	}

	if discovery != nil && stats != nil {
		discovery.SetStatsService(stats)
	}

	if proxy != nil {
		if stats != nil {
			proxy.SetStatsService(stats)
		}
		if discovery != nil {
			proxy.SetDiscoveryService(discovery)
		}
		if security != nil {
			proxy.SetSecurityService(security)
		}
	}

	if http != nil {
		if stats != nil {
			http.SetStatsService(stats)
		}
		if proxy != nil {
			http.SetProxyService(proxy)
		}
		if discovery != nil {
			http.SetDiscoveryService(discovery)
		}
		if security != nil {
			http.SetSecurityService(security)
		}
	}
}
