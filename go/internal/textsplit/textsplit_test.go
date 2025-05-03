package textsplit

import (
	"testing"

	"github.com/osak/teobot/internal/text"
)

type splitTextTestCase struct {
	input      string
	maxPartLen int
	expected   []string
}

func TestSplitText(t *testing.T) {
	testCases := []splitTextTestCase{
		{
			input:      "おはよう\nこんにちは\nこんばんは",
			maxPartLen: 10,
			expected:   []string{"おはよう\nこんにちは", "こんばんは"},
		},
		{
			input:      "おはよう\nこんにちは\nこんばんは",
			maxPartLen: 9,
			expected:   []string{"おはよう", "こんにちは", "こんばんは"},
		},
		{
			input:      "こんにちはロボ",
			maxPartLen: 10,
			expected:   []string{"こんにちはロボ"},
		},
	}
	tss := NewTextSplitService(nil)

	for i, tc := range testCases {
		parts, err := tss.SplitText(text.New(tc.input), tc.maxPartLen)
		if err != nil {
			t.Errorf("Failed to SplitText on case %d: %v", i, err)
		}
		if len(tc.expected) != len(parts) {
			t.Errorf("Resulted parts differs in length: expected %d parts, got %v", len(tc.expected), parts)
		}
		for j, part := range parts {
			if tc.expected[j] != part.String() {
				t.Errorf("Part %d does not match: expected %s, got %s", j, tc.expected[j], part)
			}
		}
	}
}
