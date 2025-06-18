package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimWithEllipsis(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		maxLength int
		centered  bool
		dots      int
		expected  string
	}{
		{
			name:      "No trimming required",
			s:         "Hello",
			maxLength: 10,
			centered:  false,
			dots:      3,
			expected:  "Hello",
		},
		{
			name:      "No trimming required (centered)",
			s:         "Hello",
			maxLength: 10,
			centered:  true,
			dots:      3,
			expected:  "Hello",
		},
		{
			name:      "Exact length, no trimming required",
			s:         "HelloThere",
			maxLength: 10,
			centered:  false,
			dots:      3,
			expected:  "HelloThere",
		},
		{
			name:      "Exact length, no trimming required (centered)",
			s:         "HelloThere",
			maxLength: 10,
			centered:  true,
			dots:      3,
			expected:  "HelloThere",
		},
		{
			name:      "Prefix trimming",
			s:         "HelloWorldExample",
			maxLength: 10,
			centered:  false,
			dots:      3,
			expected:  "HelloWo...",
		},
		{
			name:      "Centered trimming with equal prefix and suffix",
			s:         "Hello Everyone",
			maxLength: 10,
			centered:  true,
			dots:      2,
			expected:  "Hell..yone",
		},
		{
			name:      "Centered trimming unequal prefix & suffix (must show extra prefix)",
			s:         "Hello everyone",
			maxLength: 10,
			centered:  true,
			dots:      3,
			expected:  "Hell...one",
		},
		{
			name:      "Zero max length",
			s:         "Hello",
			maxLength: 0,
			centered:  false,
			dots:      3,
			expected:  "",
		},
		{
			name:      "Zero max length (centered)",
			s:         "Hello",
			maxLength: 0,
			centered:  true,
			dots:      3,
			expected:  "",
		},
		{
			name:      "Dots zero",
			s:         "HelloWorldExample",
			maxLength: 10,
			centered:  false,
			dots:      0,
			expected:  "HelloWorld",
		},
		{
			name:      "Dots zero (centered)",
			s:         "HelloWorldExample",
			maxLength: 10,
			centered:  true,
			dots:      0,
			expected:  "HelloWorld",
		},
		{
			name:      "Max length and dots zero",
			s:         "HelloWorldExample",
			maxLength: 0,
			centered:  true,
			dots:      0,
			expected:  "",
		},
		{
			name:      "Max length and dots zero (centered)",
			s:         "HelloWorldExample",
			maxLength: 0,
			centered:  true,
			dots:      0,
			expected:  "",
		},
		{
			name:      "Dots larger than string",
			s:         "Hi",
			maxLength: 5,
			centered:  false,
			dots:      5,
			expected:  "Hi",
		},
		{
			name:      "Dots larger than string (centered)",
			s:         "Hi",
			maxLength: 5,
			centered:  true,
			dots:      5,
			expected:  "Hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimWithEllipsis(tt.s, tt.maxLength, tt.centered, tt.dots)
			assert.Equal(t, tt.expected, result, "TrimWithEllipsis failed for case: %s", tt.name)
		})
	}
}
