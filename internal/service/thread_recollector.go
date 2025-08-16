package service

import "github.com/osak/teobot/internal/mastodon"

type MastodonThreadRecollector struct {
	// MastodonClient is the client to interact with Mastodon
	Client *mastodon.Client
}

func (m *MastodonThreadRecollector) RecollectThread(id string) ([]*mastodon.Status, error) {
	ctx, err := m.Client.GetReplyTree(id)
	if err != nil {
		return nil, err
	}
	return ctx.Ancestors, nil
}
