package config

import "testing"

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Basic cases
		{"MyVault", "myvault"},
		{"my_vault", "my_vault"},
		{"my-vault", "my_vault"},

		// Spaces
		{"My Obsidian Vault", "my_obsidian_vault"},
		{"Notes  and   Things", "notes_and_things"},

		// Special characters
		{"My Vault (2024)", "my_vault_2024"},
		{"Notes & Ideas", "notes_ideas"},
		{"Vault@Home!", "vaulthome"},

		// Unicode
		{"My Café Notes", "my_caf_notes"},
		{"日本語Vault", "vault"},

		// Starts with number
		{"2024 Notes", "vault_2024_notes"},
		{"123", "vault_123"},

		// Edge cases
		{"", "vault"},
		{"___", "vault"},
		{"---", "vault"},
		{"   ", "vault"},

		// Leading/trailing cleanup
		{"_vault_", "vault"},
		{"-vault-", "vault"},
		{" vault ", "vault"},

		// Multiple underscores/hyphens
		{"my--vault", "my_vault"},
		{"my__vault", "my_vault"},
		{"my - vault", "my_vault"},

		// Long names (63 char limit)
		{
			"ThisIsAReallyLongVaultNameThatExceedsThePostgreSQLIdentifierLimitOfSixtyThreeCharacters",
			"thisisareallylongvaultnamethatexceedsthepostgresqlidentifierlim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeIdentifier(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeIdentifier_MaxLength(t *testing.T) {
	// Test that result never exceeds 63 characters
	longName := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"

	result := SanitizeIdentifier(longName)
	if len(result) > 63 {
		t.Errorf("result length %d exceeds 63: %q", len(result), result)
	}
}

func TestSanitizeIdentifier_ValidIdentifier(t *testing.T) {
	// Test that result is always a valid PostgreSQL identifier
	testCases := []string{
		"My Vault",
		"123",
		"",
		"___test___",
		"valid_name",
		"UPPERCASE",
	}

	for _, tc := range testCases {
		result := SanitizeIdentifier(tc)

		// Must not be empty
		if result == "" {
			t.Errorf("SanitizeIdentifier(%q) returned empty string", tc)
			continue
		}

		// Must start with letter
		if result[0] < 'a' || result[0] > 'z' {
			if result[0] != '_' {
				t.Errorf("SanitizeIdentifier(%q) = %q, doesn't start with letter", tc, result)
			}
		}

		// Must only contain valid characters
		for _, c := range result {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
				t.Errorf("SanitizeIdentifier(%q) = %q, contains invalid character %q", tc, result, c)
			}
		}
	}
}
