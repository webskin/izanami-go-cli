package izanami

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Export(t *testing.T) {
	// NDJSON export format
	exportData := `{"type":"feature","data":{"id":"feature-1","name":"feature-1","enabled":true}}
{"type":"project","data":{"id":"project-1","name":"project-1"}}
`

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/_export", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/x-ndjson", r.Header.Get("Accept"))

		// Verify request body has export options
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, true, body["allProjects"])
		assert.Equal(t, true, body["allKeys"])
		assert.Equal(t, true, body["allWebhooks"])
		assert.Equal(t, true, body["userRights"])

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(exportData))
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
	result, err := client.Export(ctx, "test-tenant")

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "feature-1")
	assert.Contains(t, result, "project-1")
}

func TestClient_Export_EmptyResult(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/empty-tenant/_export", r.URL.Path)
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		// Empty response body
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
	result, err := client.Export(ctx, "empty-tenant")

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestClient_Export_NotFound(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Tenant not found"})
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
	result, err := client.Export(ctx, "nonexistent-tenant")

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestClient_Export_Unauthorized(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "invalid-token",
		Timeout:  30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.Export(ctx, "test-tenant")

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestClient_Export_ServerError(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Internal server error"})
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
	result, err := client.Export(ctx, "test-tenant")

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestClient_ImportV2(t *testing.T) {
	// Create a temporary file for import
	tempDir := t.TempDir()
	importFile := filepath.Join(tempDir, "import.ndjson")
	importData := `{"type":"feature","data":{"id":"feature-1","name":"feature-1","enabled":true}}
{"type":"project","data":{"id":"project-1","name":"project-1"}}
`
	err := os.WriteFile(importFile, []byte(importData), 0600)
	require.NoError(t, err)

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/_import", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "2", r.URL.Query().Get("version"))

		// Verify multipart form data
		contentType := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(contentType)
		assert.NoError(t, err)
		assert.Equal(t, "multipart/form-data", mediaType)

		// Parse the multipart form
		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		assert.NoError(t, err)
		assert.Equal(t, "export", part.FormName())

		// Read file content
		content, err := io.ReadAll(part)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "feature-1")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ImportV2Response{
			Messages: []string{"Import completed successfully"},
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
	result, err := client.ImportV2(ctx, "test-tenant", importFile, ImportRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Messages, 1)
	assert.Equal(t, "Import completed successfully", result.Messages[0])
}

func TestClient_ImportV1_WithAllOptions(t *testing.T) {
	// Create a temporary file for import
	tempDir := t.TempDir()
	importFile := filepath.Join(tempDir, "import.ndjson")
	err := os.WriteFile(importFile, []byte(`{"type":"feature"}`), 0600)
	require.NoError(t, err)

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/_import", r.URL.Path)

		// Verify query parameters for V1 import
		query := r.URL.Query()
		assert.Equal(t, "1", query.Get("version"))
		assert.Equal(t, "OVERWRITE", query.Get("conflict"))
		assert.Equal(t, "Europe/Paris", query.Get("timezone"))
		assert.Equal(t, "true", query.Get("deduceProject"))
		assert.Equal(t, "true", query.Get("create"))
		assert.Equal(t, "my-project", query.Get("project"))
		assert.Equal(t, "100", query.Get("projectPartSize"))
		assert.Equal(t, "true", query.Get("inlineScript"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(ImportV1Response{
			ID: "import-456",
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
	importReq := ImportRequest{
		Conflict:        "OVERWRITE",
		Timezone:        "Europe/Paris",
		DeduceProject:   true,
		CreateProjects:  true,
		Project:         "my-project",
		ProjectPartSize: 100,
		InlineScript:    true,
	}

	result, err := client.ImportV1(ctx, "test-tenant", importFile, importReq)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "import-456", result.ID)
}

func TestClient_ImportV2_WithConflict(t *testing.T) {
	// Create a temporary file for import
	tempDir := t.TempDir()
	importFile := filepath.Join(tempDir, "import.ndjson")
	err := os.WriteFile(importFile, []byte(`{"type":"feature"}`), 0600)
	require.NoError(t, err)

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// V2 import only uses version and conflict
		assert.Equal(t, "2", query.Get("version"))
		assert.Equal(t, "SKIP", query.Get("conflict"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ImportV2Response{
			Messages: []string{"Import completed"},
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
	importReq := ImportRequest{
		Conflict: "SKIP",
	}

	result, err := client.ImportV2(ctx, "test-tenant", importFile, importReq)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestClient_ImportV2_NotFound(t *testing.T) {
	// Create a temporary file for import
	tempDir := t.TempDir()
	importFile := filepath.Join(tempDir, "import.ndjson")
	err := os.WriteFile(importFile, []byte(`{"type":"feature"}`), 0600)
	require.NoError(t, err)

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Tenant not found"})
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
	result, err := client.ImportV2(ctx, "nonexistent-tenant", importFile, ImportRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClient_ImportV2_Unauthorized(t *testing.T) {
	// Create a temporary file for import
	tempDir := t.TempDir()
	importFile := filepath.Join(tempDir, "import.ndjson")
	err := os.WriteFile(importFile, []byte(`{"type":"feature"}`), 0600)
	require.NoError(t, err)

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
	})
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Username: "test-user",
		JwtToken: "invalid-token",
		Timeout:  30,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.ImportV2(ctx, "test-tenant", importFile, ImportRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClient_ImportV2_BadRequest(t *testing.T) {
	// Create a temporary file for import
	tempDir := t.TempDir()
	importFile := filepath.Join(tempDir, "import.ndjson")
	err := os.WriteFile(importFile, []byte(`invalid json`), 0600)
	require.NoError(t, err)

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid import file format"})
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
	result, err := client.ImportV2(ctx, "test-tenant", importFile, ImportRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClient_ImportV2_ServerError(t *testing.T) {
	// Create a temporary file for import
	tempDir := t.TempDir()
	importFile := filepath.Join(tempDir, "import.ndjson")
	err := os.WriteFile(importFile, []byte(`{"type":"feature"}`), 0600)
	require.NoError(t, err)

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Internal server error"})
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
	result, err := client.ImportV2(ctx, "test-tenant", importFile, ImportRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClient_ImportV2_Conflict(t *testing.T) {
	// Create a temporary file for import
	tempDir := t.TempDir()
	importFile := filepath.Join(tempDir, "import.ndjson")
	err := os.WriteFile(importFile, []byte(`{"type":"feature"}`), 0600)
	require.NoError(t, err)

	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict) // 409 Conflict
		json.NewEncoder(w).Encode(ImportV2Response{
			Messages: []string{"Some items imported"},
			Conflicts: []ImportConflict{
				{ID: "feature-1-id", Name: "feature-1", Description: "First feature"},
				{ID: "feature-2-id", Name: "feature-2", Description: ""},
			},
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
	result, err := client.ImportV2(ctx, "test-tenant", importFile, ImportRequest{})

	// Should return error but also populate result
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Conflicts, 2)
	assert.Equal(t, "feature-1", result.Conflicts[0].Name)
	assert.Equal(t, "feature-1-id", result.Conflicts[0].ID)
}

func TestClient_ImportV2_ConflictOptions(t *testing.T) {
	conflictOptions := []string{"FAIL", "SKIP", "OVERWRITE"}

	for _, conflict := range conflictOptions {
		t.Run("conflict_"+conflict, func(t *testing.T) {
			tempDir := t.TempDir()
			importFile := filepath.Join(tempDir, "import.ndjson")
			err := os.WriteFile(importFile, []byte(`{"type":"feature"}`), 0600)
			require.NoError(t, err)

			server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, conflict, r.URL.Query().Get("conflict"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(ImportV2Response{
					Messages: []string{"Import completed with " + conflict},
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
			result, err := client.ImportV2(ctx, "test-tenant", importFile, ImportRequest{
				Conflict: conflict,
			})

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Contains(t, result.Messages[0], conflict)
		})
	}
}

func TestClient_GetImportStatus(t *testing.T) {
	server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/admin/tenants/test-tenant/_import/import-123", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ImportV1Status{
			ID:       "import-123",
			Status:   "Success",
			Features: 10,
			Users:    5,
			Scripts:  2,
			Keys:     3,
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
	status, err := client.GetImportStatus(ctx, "test-tenant", "import-123")

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "import-123", status.ID)
	assert.Equal(t, "Success", status.Status)
	assert.Equal(t, 10, status.Features)
}
