package output

import (
	"fmt"
	"testing"
)

func TestExampleUserTableOutput(t *testing.T) {
	users := []TestUserListItem{
		{
			Username: "97845a",
			Email:    "mickael.gauvin@gmail.com",
			Admin:    true,
			UserType: "INTERNAL",
			TenantRights: map[string]string{
				"test-tenant":     "Admin",
				"test-tenant2":    "Admin",
				"tenant-todelete": "Admin",
			},
		},
		{
			Username: "RESERVED_ADMIN_USER",
			Email:    "foo.bar@somemail.com",
			Admin:    true,
			UserType: "INTERNAL",
			TenantRights: map[string]string{
				"test-tenant":     "Admin",
				"test-tenant2":    "Admin",
				"tenant-todelete": "Admin",
			},
		},
		{
			Username: "user_with_many_tenants",
			Email:    "many@example.com",
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
		{
			Username: "user_no_tenants",
			Email:    "notenant@example.com",
			Admin:    false,
			UserType: "OIDC",
		},
	}

	fmt.Println("\n=== Example Table Output ===")
	if err := Print(users, Table); err != nil {
		t.Fatalf("Print() error = %v", err)
	}
	fmt.Println("=== End Table Output ===")
}
