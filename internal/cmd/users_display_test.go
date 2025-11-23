package cmd

import (
	"os"
	"testing"

	"github.com/webskin/izanami-go-cli/internal/izanami"
)

func TestPrintUserDetails(t *testing.T) {
	// Test user with complex rights structure
	defaultTenant := "test-tenant"
	defaultProjectRight := "Read"

	user := &izanami.User{
		Username:      "RESERVED_ADMIN_USER",
		Email:         "foo.bar@somemail.com",
		Admin:         true,
		UserType:      "INTERNAL",
		DefaultTenant: &defaultTenant,
		Rights: izanami.UserRights{
			Tenants: map[string]izanami.TenantRight{
				"test-tenant": {
					Level: "Admin",
					Projects: map[string]izanami.ProjectRight{
						"toto":          {Level: "Admin"},
						"test-project":  {Level: "Admin"},
						"test-project2": {Level: "Admin"},
					},
					Keys: map[string]izanami.GeneralAtomicRight{
						"test-key2-tenant-wide":     {Level: "Admin"},
						"my-project-key-from-cli":   {Level: "Admin"},
						"my-project-key-from-cli-2": {Level: "Admin"},
					},
					Webhooks:            map[string]izanami.GeneralAtomicRight{},
					DefaultProjectRight: &defaultProjectRight,
				},
				"test-tenant2": {
					Level: "Admin",
					Projects: map[string]izanami.ProjectRight{
						"test-project-tenant2": {Level: "Admin"},
					},
					Keys:     map[string]izanami.GeneralAtomicRight{},
					Webhooks: map[string]izanami.GeneralAtomicRight{},
				},
				"tenant-todelete": {
					Level:    "Admin",
					Projects: map[string]izanami.ProjectRight{},
					Keys:     map[string]izanami.GeneralAtomicRight{},
					Webhooks: map[string]izanami.GeneralAtomicRight{},
				},
			},
		},
	}

	// This will print to stderr, showing the fancy table format
	if err := printUserDetails(os.Stderr, user); err != nil {
		t.Fatalf("printUserDetails() error = %v", err)
	}
}

func TestPrintUserDetailsWithManyItems(t *testing.T) {
	// Test user with > 3 items in each category
	user := &izanami.User{
		Username: "user_with_many_rights",
		Email:    "many@example.com",
		Admin:    false,
		UserType: "INTERNAL",
		Rights: izanami.UserRights{
			Tenants: map[string]izanami.TenantRight{
				"main-tenant": {
					Level: "Write",
					Projects: map[string]izanami.ProjectRight{
						"proj1": {Level: "Read"},
						"proj2": {Level: "Write"},
						"proj3": {Level: "Admin"},
						"proj4": {Level: "Update"},
						"proj5": {Level: "Read"},
					},
					Keys: map[string]izanami.GeneralAtomicRight{
						"key1": {Level: "Read"},
						"key2": {Level: "Write"},
						"key3": {Level: "Admin"},
						"key4": {Level: "Read"},
					},
					Webhooks: map[string]izanami.GeneralAtomicRight{
						"webhook1": {Level: "Read"},
						"webhook2": {Level: "Write"},
						"webhook3": {Level: "Admin"},
						"webhook4": {Level: "Write"},
					},
				},
			},
		},
	}

	if err := printUserDetails(os.Stderr, user); err != nil {
		t.Fatalf("printUserDetails() error = %v", err)
	}
}

func TestPrintUserDetailsNoRights(t *testing.T) {
	// Test user with no rights
	user := &izanami.User{
		Username: "basic_user",
		Email:    "basic@example.com",
		Admin:    false,
		UserType: "OIDC",
		Rights: izanami.UserRights{
			Tenants: map[string]izanami.TenantRight{},
		},
	}

	if err := printUserDetails(os.Stderr, user); err != nil {
		t.Fatalf("printUserDetails() error = %v", err)
	}
}
