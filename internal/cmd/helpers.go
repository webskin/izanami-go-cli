package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
)

// Shared variables used by both admin and check commands
var (
	featureData       string
	featureUser       string
	featureContextStr string
)

// IsUUID checks if a string matches the UUID format (8-4-4-4-12)
func IsUUID(s string) bool {
	uuidPattern := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	matched, _ := regexp.MatchString(uuidPattern, s)
	return matched
}

// ensureLeadingSlash adds a leading slash to the context path if it's not empty and doesn't have one
func ensureLeadingSlash(context string) string {
	if context != "" && context[0] != '/' {
		return "/" + context
	}
	return context
}

// parseJSONData parses JSON data from a file, stdin, or string
func parseJSONData(dataStr string, target interface{}) error {
	var data []byte
	var err error

	if dataStr == "-" {
		// Read from stdin
		data, err = os.ReadFile("/dev/stdin")
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else if len(dataStr) > 0 && dataStr[0] == '@' {
		// Read from file
		data, err = os.ReadFile(dataStr[1:])
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", dataStr[1:], err)
		}
	} else {
		// Use string directly
		data = []byte(dataStr)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

// nowISO8601 returns the current time in ISO 8601 format
func nowISO8601() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// marshalJSON marshals data to JSON bytes
func marshalJSON(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// resolveTagsToUUIDs converts tag names to UUIDs.
// If a tag value is already a valid UUID, it's used as-is.
// Otherwise, it's treated as a tag name and looked up via the admin API.
func resolveTagsToUUIDs(ctx context.Context, client *izanami.Client, tenant string, tags []string, verbose bool, cmd *cobra.Command) ([]string, error) {
	if len(tags) == 0 {
		return tags, nil
	}

	resolved := make([]string, 0, len(tags))
	for _, t := range tags {
		// Check if it's already a valid UUID
		if IsUUID(t) {
			resolved = append(resolved, t)
			continue
		}

		// Not a UUID, look up by name
		if tenant == "" {
			return nil, fmt.Errorf("tag %q is not a UUID; --tenant is required to resolve tag names", t)
		}

		tag, err := izanami.GetTag(client, ctx, tenant, t, izanami.ParseTag)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve tag %q: %w", t, err)
		}

		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Resolved tag %q to UUID %s\n", t, tag.ID)
		}
		resolved = append(resolved, tag.ID)
	}

	return resolved, nil
}

// resolveProjectsToUUIDs converts project names to UUIDs.
// If a project value is already a valid UUID, it's used as-is.
// Otherwise, it's treated as a project name and looked up via the admin API.
func resolveProjectsToUUIDs(ctx context.Context, client *izanami.Client, tenant string, projects []string, verbose bool, cmd *cobra.Command) ([]string, error) {
	if len(projects) == 0 {
		return projects, nil
	}

	resolved := make([]string, 0, len(projects))
	for _, p := range projects {
		// Check if it's already a valid UUID
		if IsUUID(p) {
			resolved = append(resolved, p)
			continue
		}

		// Not a UUID, look up by name
		if tenant == "" {
			return nil, fmt.Errorf("project %q is not a UUID; --tenant is required to resolve project names", p)
		}

		project, err := izanami.GetProject(client, ctx, tenant, p, izanami.ParseProject)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve project %q: %w", p, err)
		}

		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Resolved project %q to UUID %s\n", p, project.ID)
		}
		resolved = append(resolved, project.ID)
	}

	return resolved, nil
}

// resolveFeaturesToUUIDs converts feature names to UUIDs.
// If a feature value is already a valid UUID, it's used as-is.
// Otherwise, it's treated as a feature name and looked up by listing features.
// Requires tenant and project to be defined for name resolution.
func resolveFeaturesToUUIDs(ctx context.Context, client *izanami.Client, tenant, project string, features []string, verbose bool, cmd *cobra.Command) ([]string, error) {
	if len(features) == 0 {
		return features, nil
	}

	// Check if any features need name resolution
	needsLookup := false
	for _, f := range features {
		if !IsUUID(f) {
			needsLookup = true
			break
		}
	}

	// If all are UUIDs, return as-is
	if !needsLookup {
		return features, nil
	}

	// Validate requirements for name resolution
	if tenant == "" {
		return nil, fmt.Errorf("--tenant is required to resolve feature names")
	}
	if project == "" {
		return nil, fmt.Errorf("--project is required to resolve feature names to UUIDs")
	}

	// List all features and build a name->ID map for the specified project
	allFeatures, err := izanami.ListFeatures(client, ctx, tenant, "", izanami.ParseFeatures)
	if err != nil {
		return nil, fmt.Errorf("failed to list features: %w", err)
	}

	// Build map of name -> ID for features in the specified project
	featureMap := make(map[string]string)
	for _, feat := range allFeatures {
		if feat.Project == project {
			featureMap[feat.Name] = feat.ID
		}
	}

	resolved := make([]string, 0, len(features))
	for _, f := range features {
		// Check if it's already a valid UUID
		if IsUUID(f) {
			resolved = append(resolved, f)
			continue
		}

		// Look up by name in the map
		id, ok := featureMap[f]
		if !ok {
			return nil, fmt.Errorf("feature %q not found in project %q", f, project)
		}

		if verbose {
			fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Resolved feature %q to UUID %s\n", f, id)
		}
		resolved = append(resolved, id)
	}

	return resolved, nil
}
