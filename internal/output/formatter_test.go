package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Count   int    `json:"count"`
}

func TestPrintJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		expected string
	}{
		{
			name: "simple struct",
			data: testStruct{
				Name:    "test",
				Enabled: true,
				Count:   42,
			},
			expected: `{
  "name": "test",
  "enabled": true,
  "count": 42
}`,
		},
		{
			name: "slice of structs",
			data: []testStruct{
				{Name: "item1", Enabled: true, Count: 1},
				{Name: "item2", Enabled: false, Count: 2},
			},
			expected: `[
  {
    "name": "item1",
    "enabled": true,
    "count": 1
  },
  {
    "name": "item2",
    "enabled": false,
    "count": 2
  }
]`,
		},
		{
			name: "map",
			data: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
			},
			expected: `{
  "key1": "value1",
  "key2": 123
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := PrintTo(&buf, tt.data, JSON)
			require.NoError(t, err)

			// Normalize whitespace for comparison
			expected := strings.TrimSpace(tt.expected)
			actual := strings.TrimSpace(buf.String())
			assert.Equal(t, expected, actual)
		})
	}
}

func TestPrintTable(t *testing.T) {
	tests := []struct {
		name           string
		data           interface{}
		expectedIncludes []string
	}{
		{
			name: "slice of structs",
			data: []testStruct{
				{Name: "feature1", Enabled: true, Count: 10},
				{Name: "feature2", Enabled: false, Count: 20},
			},
			expectedIncludes: []string{"NAME", "ENABLED", "COUNT", "feature1", "feature2", "true", "false", "10", "20"},
		},
		{
			name: "single struct",
			data: testStruct{
				Name:    "test-feature",
				Enabled: true,
				Count:   5,
			},
			expectedIncludes: []string{"test-feature", "true", "5"},
		},
		{
			name: "empty slice",
			data: []testStruct{},
			expectedIncludes: []string{"No results found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := PrintTo(&buf, tt.data, Table)
			require.NoError(t, err)

			output := buf.String()
			for _, expected := range tt.expectedIncludes {
				assert.Contains(t, output, expected, "Output should contain: %s", expected)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "boolean true",
			input:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "false",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "string",
			input:    "test string",
			expected: "test string",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "slice",
			input:    []string{"a", "b", "c"},
			expected: "[a b c]",
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			// Create a simple struct with the value
			data := struct {
				Value interface{} `json:"value"`
			}{Value: tt.input}

			err := PrintTo(&buf, data, Table)
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, tt.expected)
		})
	}
}

func TestPrintInvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	data := testStruct{Name: "test", Enabled: true, Count: 1}

	err := PrintTo(&buf, data, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}
