package izanami

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Health(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/_health", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthStatus{
			Status:  "UP",
			Version: "1.0.0",
		})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "test-jwt-token",
		Timeout:  30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	health, err := Health(client, ctx, ParseHealthStatus)

	assert.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, "UP", health.Status)
	assert.Equal(t, "1.0.0", health.Version)
}
