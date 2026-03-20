package mastodon

import (
	"testing"
	"time"

	"github.com/osak/teobot/internal/service/teobot"
	"github.com/r3labs/diff/v3"
)

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

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
					Acct:        "osa_k",
					DisplayName: "osak",
				},
				Visibility: "public",
				CreatedAt:  "2026-01-22T23:45:01+09:00",
			},
			expected: teobot.Message{
				Text:         "Hello\n<metadata>\nAccount: osa_k (osak)\nPosted At: 2026-01-22T23:45:01+09:00\nVisibility: public\n</metadata>\n",
				PrivacyLevel: teobot.PrivacyLevelPublic,
				User: &teobot.User{
					Name: "osa_k",
				},
				Timestamp: mustParseTime("2026-01-22T23:45:01+09:00"),
				RawMeta: map[teobot.ChannelType]any{
					teobot.ChannelTypeMastodon: map[string]string{
						"status_id": "foo",
					},
				},
			},
		},
	}

	binding := TeobotBinding{}
	for i, tc := range testCases {
		msg, err := binding.convertToMessage(&tc.status, &teobot.User{Name: "osa_k"})
		if err != nil {
			t.Errorf("failed at test %d: %+v", i, err)
		}
		changelog, err := diff.Diff(tc.expected, *msg)
		if err != nil {
			panic(err)
		}
		if len(changelog) > 0 {
			t.Errorf("%d: got %+v\ndiff:%+v", i, msg, changelog)
		}
	}
}
