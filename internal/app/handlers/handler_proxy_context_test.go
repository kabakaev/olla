package handlers

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thushan/olla/internal/core/domain"
)

func TestFilterEndpointsByContextLength(t *testing.T) {
	// Create mock logger
	styledLog := &mockStyledLogger{}

	// Create test endpoints
	endpoint1URL, _ := url.Parse("http://localhost:11434")
	endpoint2URL, _ := url.Parse("http://localhost:11435")
	endpoint3URL, _ := url.Parse("http://localhost:11436")

	endpoints := []*domain.Endpoint{
		{
			Name:             "small-context",
			URL:              endpoint1URL,
			MaxContextLength: 8192,
		},
		{
			Name:             "large-context",
			URL:              endpoint2URL,
			MaxContextLength: 131072,
		},
		{
			Name:             "unlimited-context",
			URL:              endpoint3URL,
			MaxContextLength: 0,
		},
	}

	app := &Application{}

	tests := []struct {
		name              string
		tokenCount        int
		expectedEndpoints int
		expectedNames     []string
	}{
		{
			name:              "Small request fits all",
			tokenCount:        100,
			expectedEndpoints: 3,
			expectedNames:     []string{"small-context", "large-context", "unlimited-context"},
		},
		{
			name:              "Medium request fits large and unlimited",
			tokenCount:        10000,
			expectedEndpoints: 2,
			expectedNames:     []string{"large-context", "unlimited-context"},
		},
		{
			name:              "Huge request fits only unlimited",
			tokenCount:        200000,
			expectedEndpoints: 1,
			expectedNames:     []string{"unlimited-context"},
		},
		{
			name:              "Zero token count (no filtering)",
			tokenCount:        0,
			expectedEndpoints: 3,
			expectedNames:     []string{"small-context", "large-context", "unlimited-context"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := app.filterEndpointsByContextLength(endpoints, tt.tokenCount, styledLog)
			assert.Len(t, filtered, tt.expectedEndpoints)

			names := make([]string, 0, len(filtered))
			for _, e := range filtered {
				names = append(names, e.Name)
			}
			
			if len(tt.expectedNames) > 0 {
				assert.ElementsMatch(t, tt.expectedNames, names)
			}
		})
	}
}
