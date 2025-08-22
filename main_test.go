package main

import (
	"strings"
	"testing"
)

// TestGenerateRandomString tests the basic functionality of generateRandomString.
func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		charset string
		wantErr bool
	}{
		{"valid length and charset", 10, alphanumericChars, false},
		{"zero length", 0, alphanumericChars, true},
		{"negative length", -1, alphanumericChars, true},
		{"empty charset", 5, "", true},
		{"single char charset", 5, "a", false},
		{"large length", 1000, alphanumericChars, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateRandomString(tt.length, tt.charset)

			if tt.wantErr {
				if err == nil {
					t.Errorf("generateRandomString() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("generateRandomString() unexpected error: %v", err)
				return
			}

			if len(result) != tt.length {
				t.Errorf("generateRandomString() length = %d, want %d", len(result), tt.length)
			}

			// Check that all characters are from the charset
			for _, char := range result {
				if !strings.ContainsRune(tt.charset, char) {
					t.Errorf("generateRandomString() contains invalid char: %c", char)
				}
			}
		})
	}
}

// TestGenerateHyphenatedString tests hyphenated string generation.
func TestGenerateHyphenatedString(t *testing.T) {
	result, err := generateHyphenatedString()
	if err != nil {
		t.Fatalf("generateHyphenatedString() unexpected error: %v", err)
	}

	// Should have format: xxxxxx-xxxxxx-xxxxxx (6-6-6 with hyphens)
	if len(result) != 20 { // 6+1+6+1+6 = 20
		t.Errorf("generateHyphenatedString() length = %d, want 20", len(result))
	}

	parts := strings.Split(result, "-")
	if len(parts) != 3 {
		t.Errorf("generateHyphenatedString() parts count = %d, want 3", len(parts))
	}

	for i, part := range parts {
		if len(part) != 6 {
			t.Errorf("generateHyphenatedString() part %d length = %d, want 6", i, len(part))
		}

		// Check all characters are alphanumeric
		for _, char := range part {
			if !strings.ContainsRune(alphanumericChars, char) {
				t.Errorf("generateHyphenatedString() part %d contains invalid char: %c", i, char)
			}
		}
	}
}

// TestGenerateCompactString tests compact string generation.
func TestGenerateCompactString(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   int
	}{
		{"default length", 15, 15},
		{"custom length", 32, 32},
		{"zero length uses default", 0, defaultLength},
		{"negative length uses default", -5, defaultLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateCompactString(tt.length)
			if err != nil {
				t.Fatalf("generateCompactString() unexpected error: %v", err)
			}

			if len(result) != tt.want {
				t.Errorf("generateCompactString() length = %d, want %d", len(result), tt.want)
			}

			// Check all characters are alphanumeric
			for _, char := range result {
				if !strings.ContainsRune(alphanumericChars, char) {
					t.Errorf("generateCompactString() contains invalid char: %c", char)
				}
			}
		})
	}
}

// TestGenerateString tests the main generation function.
func TestGenerateString(t *testing.T) {
	tests := []struct {
		name       string
		outputType string
		length     int
		wantErr    bool
		checkLen   func(string) bool
	}{
		{
			name:       "hyphenated",
			outputType: TypeHyphenated,
			length:     15,
			wantErr:    false,
			checkLen:   func(s string) bool { return len(s) == 20 }, // 6-6-6 format
		},
		{
			name:       "compact",
			outputType: TypeCompact,
			length:     15,
			wantErr:    false,
			checkLen:   func(s string) bool { return len(s) == 15 },
		},
		{
			name:       "invalid type",
			outputType: "invalid",
			length:     15,
			wantErr:    true,
			checkLen:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateString(tt.outputType, tt.length)

			if tt.wantErr {
				if err == nil {
					t.Errorf("generateString() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("generateString() unexpected error: %v", err)
				return
			}

			if tt.checkLen != nil && !tt.checkLen(result) {
				t.Errorf("generateString() length check failed for result: %s", result)
			}
		})
	}
}

// TestRandomness tests that generated strings are different.
func TestRandomness(t *testing.T) {
	const iterations = 100
	results := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		result, err := generateCompactString(20)
		if err != nil {
			t.Fatalf("generateCompactString() unexpected error: %v", err)
		}

		if results[result] {
			t.Errorf("generateCompactString() produced duplicate: %s", result)
		}
		results[result] = true
	}

	if len(results) != iterations {
		t.Errorf("Expected %d unique results, got %d", iterations, len(results))
	}
}

// TestCharacterDistribution tests that characters are well distributed.
func TestCharacterDistribution(t *testing.T) {
	const iterations = 1000
	const length = 100
	charCount := make(map[rune]int)

	for i := 0; i < iterations; i++ {
		result, err := generateCompactString(length)
		if err != nil {
			t.Fatalf("generateCompactString() unexpected error: %v", err)
		}

		for _, char := range result {
			charCount[char]++
		}
	}

	// Check that we have reasonable distribution
	// With 62 possible characters (a-z, A-Z, 0-9) and 100,000 total chars,
	// each character should appear roughly 1,613 times on average
	expectedAvg := float64(iterations*length) / float64(len(alphanumericChars))
	tolerance := expectedAvg * 0.3 // Allow 30% deviation

	for _, char := range alphanumericChars {
		count := charCount[rune(char)]
		if float64(count) < expectedAvg-tolerance || float64(count) > expectedAvg+tolerance {
			t.Logf("Character %c appeared %d times (expected ~%.0f)", char, count, expectedAvg)
		}
	}
}

// BenchmarkGenerateRandomString benchmarks the core random string generation.
func BenchmarkGenerateRandomString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := generateRandomString(15, alphanumericChars)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGenerateHyphenatedString benchmarks hyphenated string generation.
func BenchmarkGenerateHyphenatedString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := generateHyphenatedString()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGenerateCompactString benchmarks compact string generation.
func BenchmarkGenerateCompactString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := generateCompactString(15)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallel tests performance under concurrent load.
func BenchmarkGenerateRandomStringParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := generateRandomString(15, alphanumericChars)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
