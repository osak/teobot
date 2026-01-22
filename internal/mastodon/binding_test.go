package mastodon

import (
	"testing"
	"time"

	"github.com/osak/teobot/internal/service/chatgpt"
	"github.com/osak/teobot/internal/service/teobot"
)

func mustParseTime()

func TestConvertToMessage(t *testing.T) {
	type testCase struct {
		status   Status
		expected teobot.Message
	}
	testCases := []testCase{
		{
			status: Status{
				ID:      "foo",
				Content: "Hello",
				Account: Account{
					Acct: "osa_k",
				},
				Visibility: "public",
				CreatedAt:  "2026-01-22 23:45:01+09:00",
			},
			expected: teobot.Message{
				Text:         "Hello",
				PrivacyLevel: teobot.PrivacyLevelPublic,
				User: &teobot.User{
					Name: "osa_k",
				},
				Timestamp: time.Parse(time.RFC3339, "2026-01-22 23:45:01+09:00"),
			},
		},
	}

	binding := TeobotBinding{}
	for i, tc := range testCases {
		msg, err := binding.convertToMessage(&tc.status)
		if err != nil {
			t.Errorf("failed at test %d: %+v", i, err)
		}
		if msg != tc.expected {
			t.Errorf()
		}
	}
}
