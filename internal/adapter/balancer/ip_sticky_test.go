package balancer

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/thushan/olla/internal/core/constants"
	"github.com/thushan/olla/internal/core/domain"
)

func TestNewIPStickySelector(t *testing.T) {
	selector := NewIPStickySelector(NewTestStatsCollector())

	if selector == nil {
		t.Fatal("NewIPStickySelector returned nil")
	}

	if selector.Name() != DefaultBalancerIPSticky {
		t.Errorf("Expected name '%s', got %q", DefaultBalancerIPSticky, selector.Name())
	}
}

func TestIPStickySelector_Select_NoEndpoints(t *testing.T) {
	selector := NewIPStickySelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoint, err := selector.Select(ctx, []*domain.Endpoint{})
	if err == nil {
		t.Error("Expected error for empty endpoints")
	}
	if endpoint != nil {
		t.Error("Expected nil endpoint for empty slice")
	}
}

func TestIPStickySelector_Select_StickyDistribution(t *testing.T) {
	selector := NewIPStickySelector(NewTestStatsCollector())
	
	endpoints := []*domain.Endpoint{
		createStickyEndpoint("endpoint-1", 11434, domain.StatusHealthy),
		createStickyEndpoint("endpoint-2", 11435, domain.StatusHealthy),
		createStickyEndpoint("endpoint-3", 11436, domain.StatusHealthy),
	}

	// Test case: same IP should always get the same endpoint
	ips := []string{"192.168.1.1", "10.0.0.5", "172.16.0.100"}
	
	for _, ip := range ips {
		ctx := context.WithValue(context.Background(), constants.ContextClientIPKey, ip)
		
		// First selection
		first, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("First select for IP %s failed: %v", ip, err)
		}
		
		// Subsequent selections for same IP
		for i := 0; i < 10; i++ {
			next, err := selector.Select(ctx, endpoints)
			if err != nil {
				t.Fatalf("Subsequent select %d for IP %s failed: %v", i, ip, err)
			}
			if next.Name != first.Name {
				t.Errorf("IP %s: expected sticky endpoint %s, but got %s", ip, first.Name, next.Name)
			}
		}
	}
}

func TestIPStickySelector_Select_FallbackToRoundRobin(t *testing.T) {
	selector := NewIPStickySelector(NewTestStatsCollector())
	
	endpoints := []*domain.Endpoint{
		createStickyEndpoint("endpoint-1", 11434, domain.StatusHealthy),
		createStickyEndpoint("endpoint-2", 11435, domain.StatusHealthy),
	}

	// Context without IP should trigger fallback to round robin
	ctx := context.Background()
	
	expectedOrder := []string{"endpoint-1", "endpoint-2", "endpoint-1", "endpoint-2"}
	
	for i, expected := range expectedOrder {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("Select %d failed: %v", i, err)
		}
		if endpoint.Name != expected {
			t.Errorf("Selection %d: expected %s (fallback), got %s", i, expected, endpoint.Name)
		}
	}
}

func TestIPStickySelector_FailoverBehavior(t *testing.T) {
	selector := NewIPStickySelector(NewTestStatsCollector())
	ip := "192.168.1.1"
	ctx := context.WithValue(context.Background(), constants.ContextClientIPKey, ip)
	
	endpoints := []*domain.Endpoint{
		createStickyEndpoint("endpoint-1", 11434, domain.StatusHealthy),
		createStickyEndpoint("endpoint-2", 11435, domain.StatusHealthy),
		createStickyEndpoint("endpoint-3", 11436, domain.StatusHealthy),
	}

	// 1. Identify which endpoint is sticky for this IP
	original, _ := selector.Select(ctx, endpoints)
	
	// 2. Mark that endpoint as offline
	for _, ep := range endpoints {
		if ep.Name == original.Name {
			ep.Status = domain.StatusOffline
		}
	}
	
	// 3. Select again - should get a different (routable) endpoint
	next, err := selector.Select(ctx, endpoints)
	if err != nil {
		t.Fatalf("Select after failover failed: %v", err)
	}
	if next.Name == original.Name {
		t.Errorf("Should not have selected offline endpoint %s", original.Name)
	}
	
	// 4. The new selection should also be sticky while original is down
	for i := 0; i < 5; i++ {
		repeat, _ := selector.Select(ctx, endpoints)
		if repeat.Name != next.Name {
			t.Errorf("Subsequent selection after failover should be sticky, expected %s, got %s", next.Name, repeat.Name)
		}
	}
}

func createStickyEndpoint(name string, port int, status domain.EndpointStatus) *domain.Endpoint {
	testURL, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))
	return &domain.Endpoint{
		Name:      name,
		URL:       testURL,
		URLString: testURL.String(),
		Status:    status,
	}
}
