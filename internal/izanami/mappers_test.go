package izanami

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentity(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr bool
	}{
		{
			name:    "returns bytes unchanged",
			input:   []byte(`{"key": "value"}`),
			want:    []byte(`{"key": "value"}`),
			wantErr: false,
		},
		{
			name:    "handles empty bytes",
			input:   []byte{},
			want:    []byte{},
			wantErr: false,
		},
		{
			name:    "handles nil bytes",
			input:   nil,
			want:    nil,
			wantErr: false,
		},
		{
			name:    "handles large bytes",
			input:   make([]byte, 10000),
			want:    make([]byte, 10000),
			wantErr: false,
		},
		{
			name:    "handles binary data",
			input:   []byte{0x00, 0xFF, 0x42, 0x13},
			want:    []byte{0x00, 0xFF, 0x42, 0x13},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Identity(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	t.Run("unmarshals slice of structs", func(t *testing.T) {
		input := []byte(`[{"name":"tenant1"},{"name":"tenant2"}]`)
		mapper := Unmarshal[[]Tenant]()

		result, err := mapper(input)

		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "tenant1", result[0].Name)
		assert.Equal(t, "tenant2", result[1].Name)
	})

	t.Run("unmarshals empty array", func(t *testing.T) {
		input := []byte(`[]`)
		mapper := Unmarshal[[]Tenant]()

		result, err := mapper(input)

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("unmarshals single struct", func(t *testing.T) {
		input := []byte(`{"name":"test-tenant","description":"Test description"}`)
		mapper := Unmarshal[Tenant]()

		result, err := mapper(input)

		require.NoError(t, err)
		assert.Equal(t, "test-tenant", result.Name)
		assert.Equal(t, "Test description", result.Description)
	})

	t.Run("returns error on malformed JSON", func(t *testing.T) {
		input := []byte(`{invalid json}`)
		mapper := Unmarshal[Tenant]()

		_, err := mapper(input)

		assert.Error(t, err)
	})

	t.Run("returns error on type mismatch", func(t *testing.T) {
		input := []byte(`"not an object"`)
		mapper := Unmarshal[Tenant]()

		_, err := mapper(input)

		assert.Error(t, err)
	})

	t.Run("unmarshals complex nested types", func(t *testing.T) {
		input := []byte(`{"name":"tenant","projects":[{"id":"proj1","name":"Project 1"}]}`)
		mapper := Unmarshal[Tenant]()

		result, err := mapper(input)

		require.NoError(t, err)
		assert.Equal(t, "tenant", result.Name)
		require.Len(t, result.Projects, 1)
		assert.Equal(t, "proj1", result.Projects[0].ID)
	})

	t.Run("unmarshals map type", func(t *testing.T) {
		input := []byte(`{"feature1":{"name":"Feature 1","active":true}}`)
		mapper := Unmarshal[ActivationsWithConditions]()

		result, err := mapper(input)

		require.NoError(t, err)
		assert.Contains(t, result, "feature1")
		assert.Equal(t, "Feature 1", result["feature1"].Name)
	})
}

func TestUnmarshalPtr(t *testing.T) {
	t.Run("returns pointer to unmarshaled struct", func(t *testing.T) {
		input := []byte(`{"name":"test-tenant","description":"Test description"}`)
		mapper := UnmarshalPtr[Tenant]()

		result, err := mapper(input)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-tenant", result.Name)
		assert.Equal(t, "Test description", result.Description)
	})

	t.Run("returns nil on malformed JSON", func(t *testing.T) {
		input := []byte(`{invalid}`)
		mapper := UnmarshalPtr[Tenant]()

		result, err := mapper(input)

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("unmarshals complex nested types", func(t *testing.T) {
		input := []byte(`{
			"id": "feat-1",
			"name": "Feature 1",
			"project": "proj1",
			"enabled": true,
			"conditions": [{"rule": {"type": "All"}}]
		}`)
		mapper := UnmarshalPtr[FeatureWithOverloads]()

		result, err := mapper(input)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "feat-1", result.ID)
		assert.Equal(t, "Feature 1", result.Name)
		assert.True(t, result.Enabled)
		require.Len(t, result.Conditions, 1)
		assert.Equal(t, "All", result.Conditions[0].Rule.Type)
	})

	t.Run("handles empty JSON object", func(t *testing.T) {
		input := []byte(`{}`)
		mapper := UnmarshalPtr[Tenant]()

		result, err := mapper(input)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "", result.Name)
	})

	t.Run("handles optional fields", func(t *testing.T) {
		input := []byte(`{"clientId":"key1","name":"Test Key"}`)
		mapper := UnmarshalPtr[APIKey]()

		result, err := mapper(input)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "key1", result.ClientID)
		assert.Equal(t, "Test Key", result.Name)
		assert.Empty(t, result.ClientSecret) // optional field not present
	})
}

func TestPredefinedMappers(t *testing.T) {
	t.Run("ParseTenants", func(t *testing.T) {
		input := []byte(`[{"name":"tenant1"},{"name":"tenant2"}]`)

		result, err := ParseTenants(input)

		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("ParseTenant", func(t *testing.T) {
		input := []byte(`{"name":"tenant1","description":"Test"}`)

		result, err := ParseTenant(input)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "tenant1", result.Name)
	})

	t.Run("ParseProjects", func(t *testing.T) {
		input := []byte(`[{"id":"proj1","name":"Project 1"}]`)

		result, err := ParseProjects(input)

		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "proj1", result[0].ID)
	})

	t.Run("ParseProject", func(t *testing.T) {
		input := []byte(`{"id":"proj1","name":"Project 1","description":"Desc"}`)

		result, err := ParseProject(input)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "proj1", result.ID)
	})

	t.Run("ParseAPIKeys", func(t *testing.T) {
		input := []byte(`[{"clientId":"key1","name":"Key 1"}]`)

		result, err := ParseAPIKeys(input)

		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "key1", result[0].ClientID)
	})

	t.Run("ParseUsers", func(t *testing.T) {
		input := []byte(`[{"username":"user1","email":"user1@example.com"}]`)

		result, err := ParseUsers(input)

		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "user1", result[0].Username)
	})

	t.Run("ParseFeatureCheckResult", func(t *testing.T) {
		input := []byte(`{"active":true,"name":"feature1","project":"proj1"}`)

		result, err := ParseFeatureCheckResult(input)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, true, result.Active)
		assert.Equal(t, "feature1", result.Name)
	})

	t.Run("ParseActivationsWithConditions", func(t *testing.T) {
		input := []byte(`{"feature1":{"name":"Feature 1","active":true,"project":"proj1"}}`)

		result, err := ParseActivationsWithConditions(input)

		require.NoError(t, err)
		assert.Contains(t, result, "feature1")
	})

	t.Run("ParseHealthStatus", func(t *testing.T) {
		input := []byte(`{"database":true,"status":"UP","version":"1.0.0"}`)

		result, err := ParseHealthStatus(input)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Database)
		assert.Equal(t, "UP", result.Status)
	})

	t.Run("ParseSearchResults", func(t *testing.T) {
		input := []byte(`[{"type":"FEATURE","name":"feature1","tenant":"tenant1"}]`)

		result, err := ParseSearchResults(input)

		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "FEATURE", result[0].Type)
	})
}
