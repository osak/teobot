package teobot

import "testing"

func TestToInputText(t *testing.T) {
	type testCase struct {
		message  Message
		expected string
	}
	testCases := []testCase{
		{
			message: Message{
				Text: "test1",
				RawMeta: map[ChannelType]any{
					ChannelTypeMastodon: "Account: osak\nPosted At: 2026-01-01T12:34:56+09:00",
				},
			},
			expected: "test1\n<metadata channel=\"mastodon\">\nAccount: osak\nPosted At: 2026-01-01T12:34:56+09:00\n</metadata>\n",
		},
	}

	for i, tc := range testCases {
		res := tc.message.ToInputText()
		if res != tc.expected {
			t.Errorf("%d: text mismatch; expected %s, got %s", i, tc.expected, res)
		}
	}
}
