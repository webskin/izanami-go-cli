package izanami

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Disable color output for consistent test assertions
	color.NoColor = true
}

func TestActivationTableView_FormatActive(t *testing.T) {
	tests := []struct {
		name   string
		active interface{}
		want   string
	}{
		// Boolean cases
		{
			name:   "bool true",
			active: true,
			want:   "true",
		},
		{
			name:   "bool false",
			active: false,
			want:   "false",
		},
		// String cases
		{
			name:   "string non-empty",
			active: "custom-value",
			want:   "custom-value",
		},
		{
			name:   "string empty",
			active: "",
			want:   "",
		},
		{
			name:   "string false",
			active: "false",
			want:   "false",
		},
		{
			name:   "string 0",
			active: "0",
			want:   "0",
		},
		{
			name:   "string true",
			active: "true",
			want:   "true",
		},
		// Integer cases
		{
			name:   "int positive",
			active: 42,
			want:   "42",
		},
		{
			name:   "int zero",
			active: 0,
			want:   "0",
		},
		{
			name:   "int negative",
			active: -1,
			want:   "-1",
		},
		{
			name:   "int64 positive",
			active: int64(100),
			want:   "100",
		},
		// Float cases
		{
			name:   "float positive",
			active: 3.14,
			want:   "3.14",
		},
		{
			name:   "float zero",
			active: 0.0,
			want:   "0",
		},
		// Nil case
		{
			name:   "nil value",
			active: nil,
			want:   "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := ActivationTableView{Active: tt.active}
			got := a.FormatActive()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestActivationsWithConditions_ToTableView(t *testing.T) {
	t.Run("empty map returns empty slice", func(t *testing.T) {
		activations := ActivationsWithConditions{}

		result := activations.ToTableView()

		assert.Empty(t, result)
	})

	t.Run("single activation", func(t *testing.T) {
		activations := ActivationsWithConditions{
			"feature1": {
				Name:    "Feature 1",
				Active:  true,
				Project: "project1",
			},
		}

		result := activations.ToTableView()

		assert.Len(t, result, 1)
		assert.Equal(t, "feature1", result[0].ID)
		assert.Equal(t, "Feature 1", result[0].Name)
		assert.Equal(t, "project1", result[0].Project)
	})

	t.Run("multiple activations sorted by name", func(t *testing.T) {
		activations := ActivationsWithConditions{
			"feature-z": {Name: "Zebra Feature", Active: true, Project: "proj"},
			"feature-a": {Name: "Alpha Feature", Active: false, Project: "proj"},
			"feature-m": {Name: "Middle Feature", Active: true, Project: "proj"},
		}

		result := activations.ToTableView()

		assert.Len(t, result, 3)
		assert.Equal(t, "Alpha Feature", result[0].Name)
		assert.Equal(t, "Middle Feature", result[1].Name)
		assert.Equal(t, "Zebra Feature", result[2].Name)
	})

	t.Run("preserves active value formatting", func(t *testing.T) {
		activations := ActivationsWithConditions{
			"feature1": {Name: "Feature 1", Active: "custom-value", Project: "proj"},
		}

		result := activations.ToTableView()

		assert.Equal(t, "custom-value", result[0].Active)
	})
}

func TestFeatureTestResults_ToTableView(t *testing.T) {
	t.Run("empty map returns empty slice", func(t *testing.T) {
		results := FeatureTestResults{}

		view := results.ToTableView()

		assert.Empty(t, view)
	})

	t.Run("single result without error", func(t *testing.T) {
		results := FeatureTestResults{
			"feature1": {Name: "Feature 1", Active: true, Project: "proj1"},
		}

		view := results.ToTableView()

		assert.Len(t, view, 1)
		assert.Equal(t, "feature1", view[0].ID)
		assert.Equal(t, "Feature 1", view[0].Name)
		assert.Empty(t, view[0].Error)
	})

	t.Run("result with error", func(t *testing.T) {
		results := FeatureTestResults{
			"feature1": {Name: "Feature 1", Active: nil, Project: "proj1", Error: "evaluation failed"},
		}

		view := results.ToTableView()

		assert.Len(t, view, 1)
		assert.Equal(t, "evaluation failed", view[0].Error)
	})

	t.Run("multiple results sorted by name", func(t *testing.T) {
		results := FeatureTestResults{
			"z-feature": {Name: "Zebra", Active: true, Project: "proj"},
			"a-feature": {Name: "Alpha", Active: false, Project: "proj"},
		}

		view := results.ToTableView()

		assert.Len(t, view, 2)
		assert.Equal(t, "Alpha", view[0].Name)
		assert.Equal(t, "Zebra", view[1].Name)
	})
}

func TestUserListItem_FormatTenantRights(t *testing.T) {
	t.Run("empty rights returns dash", func(t *testing.T) {
		user := UserListItem{TenantRights: nil}

		result := user.FormatTenantRights()

		assert.Equal(t, "-", result)
	})

	t.Run("empty map returns dash", func(t *testing.T) {
		user := UserListItem{TenantRights: map[string]string{}}

		result := user.FormatTenantRights()

		assert.Equal(t, "-", result)
	})

	t.Run("single tenant right", func(t *testing.T) {
		user := UserListItem{
			TenantRights: map[string]string{"tenant1": "Admin"},
		}

		result := user.FormatTenantRights()

		assert.Equal(t, "tenant1:Admin", result)
	})

	t.Run("two tenant rights", func(t *testing.T) {
		user := UserListItem{
			TenantRights: map[string]string{
				"tenant1": "Admin",
				"tenant2": "Read",
			},
		}

		result := user.FormatTenantRights()

		// Result contains both, order may vary due to map iteration
		assert.Contains(t, result, "tenant1:Admin")
		assert.Contains(t, result, "tenant2:Read")
		assert.Contains(t, result, ",")
	})

	t.Run("three tenant rights", func(t *testing.T) {
		user := UserListItem{
			TenantRights: map[string]string{
				"tenant1": "Admin",
				"tenant2": "Read",
				"tenant3": "Write",
			},
		}

		result := user.FormatTenantRights()

		assert.Contains(t, result, "tenant1:Admin")
		assert.Contains(t, result, "tenant2:Read")
		assert.Contains(t, result, "tenant3:Write")
	})

	t.Run("more than three rights shows count", func(t *testing.T) {
		user := UserListItem{
			TenantRights: map[string]string{
				"tenant1": "Admin",
				"tenant2": "Read",
				"tenant3": "Write",
				"tenant4": "Admin",
			},
		}

		result := user.FormatTenantRights()

		assert.Equal(t, "[4 items]", result)
	})
}

func TestContext_ToTableView(t *testing.T) {
	t.Run("without parent path", func(t *testing.T) {
		ctx := Context{
			Name:        "production",
			Project:     "proj1",
			IsProtected: true,
			Global:      false,
		}

		view := ctx.ToTableView("")

		assert.Equal(t, "production", view.Path)
		assert.Equal(t, "production", view.Name)
		assert.Equal(t, "proj1", view.Project)
		assert.True(t, view.IsProtected)
		assert.False(t, view.Global)
	})

	t.Run("with parent path", func(t *testing.T) {
		ctx := Context{
			Name:   "staging",
			Global: true,
		}

		view := ctx.ToTableView("env")

		assert.Equal(t, "env/staging", view.Path)
		assert.Equal(t, "staging", view.Name)
	})

	t.Run("context with explicit Path field", func(t *testing.T) {
		ctx := Context{
			Name: "prod",
			Path: "custom/path/prod",
		}

		view := ctx.ToTableView("parent")

		assert.Equal(t, "custom/path/prod", view.Path) // Path field overrides
		assert.Equal(t, "prod", view.Name)
	})

	t.Run("includes overloads", func(t *testing.T) {
		ctx := Context{
			Name: "test",
			Overloads: []FeatureOverload{
				{ID: "f1", Name: "Feature 1", Enabled: true},
			},
		}

		view := ctx.ToTableView("")

		assert.Len(t, view.Overloads, 1)
		assert.Equal(t, "f1", view.Overloads[0].ID)
	})
}

func TestContext_ToTableViewSimple(t *testing.T) {
	t.Run("without parent path", func(t *testing.T) {
		ctx := Context{
			Name:        "production",
			Project:     "proj1",
			IsProtected: true,
			Global:      false,
		}

		view := ctx.ToTableViewSimple("")

		assert.Equal(t, "production", view.Path)
		assert.Equal(t, "production", view.Name)
		assert.Equal(t, "proj1", view.Project)
		assert.True(t, view.IsProtected)
		assert.False(t, view.Global)
	})

	t.Run("with parent path", func(t *testing.T) {
		ctx := Context{
			Name:   "staging",
			Global: true,
		}

		view := ctx.ToTableViewSimple("env")

		assert.Equal(t, "env/staging", view.Path)
		assert.Equal(t, "staging", view.Name)
	})

	t.Run("context with explicit Path field overrides", func(t *testing.T) {
		ctx := Context{
			Name: "prod",
			Path: "custom/path/prod",
		}

		view := ctx.ToTableViewSimple("parent")

		assert.Equal(t, "custom/path/prod", view.Path)
	})
}

func TestFlattenContextsForTable(t *testing.T) {
	t.Run("empty contexts returns empty slice", func(t *testing.T) {
		result := FlattenContextsForTable([]Context{})

		assert.Empty(t, result)
	})

	t.Run("single context without children", func(t *testing.T) {
		contexts := []Context{
			{Name: "production", Project: "proj1"},
		}

		result := FlattenContextsForTable(contexts)

		assert.Len(t, result, 1)
		assert.Equal(t, "production", result[0].Path)
	})

	t.Run("context with children", func(t *testing.T) {
		child := &Context{Name: "us-east", Project: "proj1"}
		contexts := []Context{
			{
				Name:     "production",
				Project:  "proj1",
				Children: []*Context{child},
			},
		}

		result := FlattenContextsForTable(contexts)

		assert.Len(t, result, 2)
		assert.Equal(t, "production", result[0].Path)
		assert.Equal(t, "production/us-east", result[1].Path)
	})

	t.Run("deeply nested contexts", func(t *testing.T) {
		grandchild := &Context{Name: "zone1"}
		child := &Context{Name: "us-east", Children: []*Context{grandchild}}
		contexts := []Context{
			{Name: "production", Children: []*Context{child}},
		}

		result := FlattenContextsForTable(contexts)

		assert.Len(t, result, 3)
		assert.Equal(t, "production", result[0].Path)
		assert.Equal(t, "production/us-east", result[1].Path)
		assert.Equal(t, "production/us-east/zone1", result[2].Path)
	})

	t.Run("sorted by Global then Path", func(t *testing.T) {
		contexts := []Context{
			{Name: "global-ctx", Global: true},
			{Name: "local-b", Global: false},
			{Name: "local-a", Global: false},
		}

		result := FlattenContextsForTable(contexts)

		assert.Len(t, result, 3)
		// Non-global first, sorted by path
		assert.Equal(t, "local-a", result[0].Path)
		assert.False(t, result[0].Global)
		assert.Equal(t, "local-b", result[1].Path)
		assert.False(t, result[1].Global)
		// Global last
		assert.Equal(t, "global-ctx", result[2].Path)
		assert.True(t, result[2].Global)
	})

	t.Run("handles nil children", func(t *testing.T) {
		contexts := []Context{
			{Name: "production", Children: []*Context{nil, {Name: "valid"}}},
		}

		result := FlattenContextsForTable(contexts)

		assert.Len(t, result, 2) // nil child is skipped
	})
}

func TestFlattenContextsForTableSimple(t *testing.T) {
	t.Run("empty contexts returns empty slice", func(t *testing.T) {
		result := FlattenContextsForTableSimple([]Context{})

		assert.Empty(t, result)
	})

	t.Run("flattens nested contexts", func(t *testing.T) {
		child := &Context{Name: "child"}
		contexts := []Context{
			{Name: "parent", Children: []*Context{child}},
		}

		result := FlattenContextsForTableSimple(contexts)

		assert.Len(t, result, 2)
		assert.Equal(t, "parent", result[0].Path)
		assert.Equal(t, "parent/child", result[1].Path)
	})

	t.Run("sorts by Global then Path", func(t *testing.T) {
		contexts := []Context{
			{Name: "z-global", Global: true},
			{Name: "a-local", Global: false},
		}

		result := FlattenContextsForTableSimple(contexts)

		assert.Len(t, result, 2)
		assert.Equal(t, "a-local", result[0].Path)
		assert.False(t, result[0].Global)
		assert.Equal(t, "z-global", result[1].Path)
		assert.True(t, result[1].Global)
	})
}

func TestFeatureOverload_FormatForTable(t *testing.T) {
	t.Run("enabled feature", func(t *testing.T) {
		f := FeatureOverload{Name: "feature1", Enabled: true}

		result := f.FormatForTable()

		assert.Equal(t, "feature1 (enabled)", result)
	})

	t.Run("disabled feature", func(t *testing.T) {
		f := FeatureOverload{Name: "feature1", Enabled: false}

		result := f.FormatForTable()

		assert.Equal(t, "feature1 (disabled)", result)
	})
}

func TestAuditEvent_ToTableView(t *testing.T) {
	t.Run("with name", func(t *testing.T) {
		e := AuditEvent{
			EventID:        123,
			ID:             "event-id",
			Name:           "Event Name",
			Type:           "FEATURE_CREATED",
			User:           "admin",
			Project:        "proj1",
			EmittedAt:      "2024-01-15T10:00:00Z",
			Authentication: "JWT",
		}

		view := e.ToTableView()

		assert.Equal(t, int64(123), view.EventID)
		assert.Equal(t, "Event Name", view.Name)
		assert.Equal(t, "FEATURE_CREATED", view.Type)
		assert.Equal(t, "admin", view.User)
		assert.Equal(t, "proj1", view.Project)
	})

	t.Run("without name falls back to ID", func(t *testing.T) {
		e := AuditEvent{
			EventID: 456,
			ID:      "fallback-id",
			Name:    "",
			Type:    "FEATURE_UPDATED",
		}

		view := e.ToTableView()

		assert.Equal(t, "fallback-id", view.Name)
	})
}

func TestLogsResponse_ToTableView(t *testing.T) {
	t.Run("empty events", func(t *testing.T) {
		r := LogsResponse{Events: []AuditEvent{}}

		views := r.ToTableView()

		assert.Empty(t, views)
	})

	t.Run("converts all events", func(t *testing.T) {
		r := LogsResponse{
			Events: []AuditEvent{
				{EventID: 1, Name: "Event 1", Type: "CREATE"},
				{EventID: 2, Name: "Event 2", Type: "UPDATE"},
			},
			Count: 2,
		}

		views := r.ToTableView()

		assert.Len(t, views, 2)
		assert.Equal(t, int64(1), views[0].EventID)
		assert.Equal(t, int64(2), views[1].EventID)
	})
}

func TestRightLevel_String(t *testing.T) {
	tests := []struct {
		level RightLevel
		want  string
	}{
		{RightLevelRead, "Read"},
		{RightLevelWrite, "Write"},
		{RightLevelAdmin, "Admin"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.level.String())
		})
	}
}
