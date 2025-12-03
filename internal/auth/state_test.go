package auth

import (
	"regexp"
	"testing"
)

// TestGenerateState tests the basic functionality of state generation.
func TestGenerateState(t *testing.T) {
	state, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState() returned error: %v", err)
	}

	// Check that state is not empty
	if state == "" {
		t.Error("GenerateState() returned empty string")
	}

	// Check length: 32 bytes base64url encoded = 43 characters
	// Formula: ceil(32 * 8 / 6) = 43 characters (no padding)
	expectedLength := 43
	if len(state) != expectedLength {
		t.Errorf("GenerateState() returned state with length %d, want %d", len(state), expectedLength)
	}

	// Check that state contains only valid base64url characters
	// Valid characters: A-Z, a-z, 0-9, -, _
	base64urlPattern := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	if !base64urlPattern.MatchString(state) {
		t.Errorf("GenerateState() returned state with invalid characters: %s", state)
	}

	// Check that there are no padding characters (=)
	for _, c := range state {
		if c == '=' {
			t.Errorf("GenerateState() returned state with padding character: %s", state)
		}
	}
}

// TestGenerateStateUniqueness tests that each call produces a unique state.
// While not a guarantee (there's an infinitesimal chance of collision with 256 bits),
// generating duplicates would indicate a problem with the random number generator.
func TestGenerateStateUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		state, err := GenerateState()
		if err != nil {
			t.Fatalf("GenerateState() returned error on iteration %d: %v", i, err)
		}

		if seen[state] {
			t.Errorf("GenerateState() produced duplicate state on iteration %d: %s", i, state)
		}
		seen[state] = true
	}
}

// TestGenerateStateEntropy verifies the state has sufficient randomness.
// This is a basic check - we verify that different parts of the state vary.
func TestGenerateStateEntropy(t *testing.T) {
	// Generate multiple states and check character distribution
	states := make([]string, 100)
	for i := 0; i < 100; i++ {
		state, err := GenerateState()
		if err != nil {
			t.Fatalf("GenerateState() returned error: %v", err)
		}
		states[i] = state
	}

	// Check that first characters vary (not all starting with same char)
	firstChars := make(map[rune]int)
	for _, state := range states {
		firstChars[rune(state[0])]++
	}

	// If all 100 states start with the same character, something is wrong
	if len(firstChars) < 5 {
		t.Errorf("GenerateState() shows poor entropy: first characters have only %d unique values", len(firstChars))
	}
}

// TestValidateStateFormat tests the state format validation function.
func TestValidateStateFormat(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  bool
	}{
		{
			name:  "valid 43 char state - all letters",
			state: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq", // exactly 43 chars
			want:  true,
		},
		{
			name:  "valid with numbers and special",
			state: "ABCD1234-_abcd1234-_ABCD1234-_abcd1234-_ABC", // exactly 43 chars
			want:  true,
		},
		{
			name:  "too short",
			state: "abc123",
			want:  false,
		},
		{
			name:  "too long",
			state: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrst", // 46 chars
			want:  false,
		},
		{
			name:  "empty string",
			state: "",
			want:  false,
		},
		{
			name:  "invalid character +",
			state: "ABCDEFGHIJKLMNOPQRSTUVWXYZ+bcdefghijklmno", // + is not base64url
			want:  false,
		},
		{
			name:  "invalid character /",
			state: "ABCDEFGHIJKLMNOPQRSTUVWXYZ/bcdefghijklmno", // / is not base64url
			want:  false,
		},
		{
			name:  "invalid character =",
			state: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijk=mno", // = is padding
			want:  false,
		},
		{
			name:  "invalid character space",
			state: "ABCDEFGHIJKLMNOPQRSTUVWXYZ bcdefghijklmno", // space is invalid
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateStateFormat(tt.state)
			if got != tt.want {
				t.Errorf("ValidateStateFormat(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestValidateStateFormatWithGenerated verifies that generated states pass validation.
func TestValidateStateFormatWithGenerated(t *testing.T) {
	for i := 0; i < 100; i++ {
		state, err := GenerateState()
		if err != nil {
			t.Fatalf("GenerateState() returned error: %v", err)
		}

		if !ValidateStateFormat(state) {
			t.Errorf("ValidateStateFormat() returned false for generated state: %s (len=%d)", state, len(state))
		}
	}
}

// TestIsBase64URLChar tests the character validation helper.
func TestIsBase64URLChar(t *testing.T) {
	// Valid base64url characters
	validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	for _, c := range validChars {
		if !isBase64URLChar(c) {
			t.Errorf("isBase64URLChar(%q) = false, want true", c)
		}
	}

	// Invalid characters
	invalidChars := "+/=!@#$%^&*() \t\n"
	for _, c := range invalidChars {
		if isBase64URLChar(c) {
			t.Errorf("isBase64URLChar(%q) = true, want false", c)
		}
	}
}
