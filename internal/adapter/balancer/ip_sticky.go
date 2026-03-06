package balancer

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/thushan/olla/internal/core/constants"
	"github.com/thushan/olla/internal/core/domain"
	"github.com/thushan/olla/internal/core/ports"
)

// IPStickySelector implements consistent hashing based on client IP address.
// This ensures that requests from the same IP are routed to the same backend
// as long as that backend is healthy and available.
type IPStickySelector struct {
	statsCollector ports.StatsCollector
	fallback       *RoundRobinSelector
}

func NewIPStickySelector(statsCollector ports.StatsCollector) *IPStickySelector {
	return &IPStickySelector{
		statsCollector: statsCollector,
		fallback:       NewRoundRobinSelector(statsCollector),
	}
}

func (s *IPStickySelector) Name() string {
	return DefaultBalancerIPSticky
}

// Select chooses an endpoint based on the client IP address.
// If the IP address is not available in the context, it falls back to Round Robin.
func (s *IPStickySelector) Select(ctx context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints available")
	}

	// Filter for routable endpoints first
	routable := make([]*domain.Endpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Status.IsRoutable() {
			routable = append(routable, endpoint)
		}
	}

	if len(routable) == 0 {
		return nil, fmt.Errorf("no routable endpoints available")
	}

	// Extract client IP from context
	clientIP, _ := ctx.Value(constants.ContextClientIPKey).(string)
	if clientIP == "" {
		// Fallback to round robin if client IP is not provided
		return s.fallback.Select(ctx, routable)
	}

	// Calculate deterministic index based on client IP
	h := fnv.New32a()
	_, _ = h.Write([]byte(clientIP))
	hash := h.Sum32()

	index := hash % uint32(len(routable))
	return routable[index], nil
}

func (s *IPStickySelector) IncrementConnections(endpoint *domain.Endpoint) {
	s.statsCollector.RecordConnection(endpoint, 1)
}

func (s *IPStickySelector) DecrementConnections(endpoint *domain.Endpoint) {
	s.statsCollector.RecordConnection(endpoint, -1)
}
