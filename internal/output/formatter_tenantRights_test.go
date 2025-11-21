package output

import (
	"bytes"
	"strings"
	"testing"
)

// TestUserListItem is a test struct matching the UserListItem structure
type TestUserListItem struct {
	Username     string            `json:"username"`
	Email        string            `json:"email"`
	Admin        bool              `json:"admin"`
	UserType     string            `json:"userType"`
	TenantRights map[string]string `json:"tenantRights,omitempty"`
}

func TestFormatTenantRightsMap(t *testing.T) {
	tests := []struct {
		name     string
		users    []TestUserListItem
		wantRows int
		contains []string
	}{
		{
			name: "user with 3 tenant rights",
			users: []TestUserListItem{
				{
					Username: "user1",
					Email:    "user1@example.com",
					Admin:    true,
					UserType: "INTERNAL",
					TenantRights: map[string]string{
						"test-tenant":     "Admin",
						"test-tenant2":    "Admin",
						"tenant-todelete": "Admin",
					},
				},
			},
			wantRows: 1,
			contains: []string{"user1", "user1@example.com", "true", "INTERNAL"},
		},
		{
			name: "user with more than 3 tenant rights shows count",
			users: []TestUserListItem{
				{
					Username: "user2",
					Email:    "user2@example.com",
					Admin:    false,
					UserType: "INTERNAL",
					TenantRights: map[string]string{
						"tenant1": "Read",
						"tenant2": "Write",
						"tenant3": "Admin",
						"tenant4": "Read",
						"tenant5": "Write",
					},
				},
			},
			wantRows: 1,
			contains: []string{"user2", "[5 items]"},
		},
		{
			name: "user with no tenant rights",
			users: []TestUserListItem{
				{
					Username: "user3",
					Email:    "user3@example.com",
					Admin:    false,
					UserType: "OIDC",
				},
			},
			wantRows: 1,
			contains: []string{"user3", "-"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := PrintTo(&buf, tt.users, Table)
			if err != nil {
				t.Fatalf("PrintTo() error = %v", err)
			}

			output := buf.String()
			lines := strings.Split(strings.TrimSpace(output), "\n")

			// Check that we have the expected number of data rows (+1 for header)
			if len(lines) != tt.wantRows+1 {
				t.Errorf("Expected %d lines (including header), got %d", tt.wantRows+1, len(lines))
			}

			// Check that output contains expected strings
			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("Output should contain %q, got:\n%s", want, output)
				}
			}

			// For user with 3 tenants, verify format shows tenant:level pairs
			if tt.name == "user with 3 tenant rights" {
				// Should contain colon-separated tenant:level format
				hasFormat := strings.Contains(output, "test-tenant:Admin") ||
					strings.Contains(output, "test-tenant2:Admin") ||
					strings.Contains(output, "tenant-todelete:Admin")
				if !hasFormat {
					t.Errorf("Output should contain tenant:level format, got:\n%s", output)
				}
			}
		})
	}
}

func TestFormatTenantRightsMultipleUsers(t *testing.T) {
	users := []TestUserListItem{
		{
			Username: "admin1",
			Email:    "admin1@example.com",
			Admin:    true,
			UserType: "INTERNAL",
			TenantRights: map[string]string{
				"tenant-a": "Admin",
				"tenant-b": "Admin",
			},
		},
		{
			Username: "user1",
			Email:    "user1@example.com",
			Admin:    false,
			UserType: "INTERNAL",
			TenantRights: map[string]string{
				"tenant-x": "Read",
				"tenant-y": "Write",
				"tenant-z": "Admin",
				"tenant-w": "Read",
			},
		},
	}

	var buf bytes.Buffer
	err := PrintTo(&buf, users, Table)
	if err != nil {
		t.Fatalf("PrintTo() error = %v", err)
	}

	output := buf.String()

	// Check both users are present
	if !strings.Contains(output, "admin1") {
		t.Error("Output should contain admin1")
	}
	if !strings.Contains(output, "user1") {
		t.Error("Output should contain user1")
	}

	// Second user should show [4 items]
	if !strings.Contains(output, "[4 items]") {
		t.Errorf("Output should contain '[4 items]', got:\n%s", output)
	}
}
