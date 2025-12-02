package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
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
