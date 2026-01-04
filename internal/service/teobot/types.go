package teobot

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ChannelType string
type PrivacyLevel string

const (
	ChannelTypeMastodon ChannelType = "mastodon"
)
const (
	PrivacyLevelPublic PrivacyLevel = "public"
	PrivacyLevelDirect              = "direct"
)

type Thread struct {
	ID       uuid.UUID
	Messages []*Message
	RawMeta  map[ChannelType]any
}

type User struct {
	ID      uuid.UUID
	Name    string
	RawMeta map[ChannelType]any
}

type Message struct {
	ID           uuid.UUID
	Text         string
	PrivacyLevel PrivacyLevel
	User         *User
	Timestamp    time.Time
	RawMeta      map[ChannelType]any
}

type Context struct {
	RunCtx context.Context
}
