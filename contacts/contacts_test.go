package contacts

import "testing"

func TestNewStoreReportsAnInaccessibleDatabasePath(t *testing.T) {
	_, err := NewStore(t.TempDir() + "/missing.db")
	if err == nil {
		t.Fatal("NewStore succeeded with a missing database")
	}
}

func TestNormalizePhoneNumberMatchesQt56Minimization(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"international formatted": {"+421 123 456 789", "23456789"},
		"international compact":   {"+421987654321", "87654321"},
		"short number":            {"12345", "12345"},
		"formatting punctuation":  {"(12)34[567]890", "34567890"},
		"unicode digit":           {"1234５67890", "34567890"},
		"SIP identifier":          {"sips:+421123456789@example.test", "23456789"},
		"DTMF suffix":             {"+421123456789X12", "23456789p12"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := normalizePhoneNumber(test.input); got != test.want {
				t.Fatalf("normalizePhoneNumber(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
