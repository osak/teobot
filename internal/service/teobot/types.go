package teobot

import (
	"context"
	"maps"
	"time"

	"github.com/google/uuid"
)

type ChannelType string
type PrivacyLevel string

const (
	ChannelTypeMastodon ChannelType = "mastodon"
)
const (
	PrivacyLevelPublic  PrivacyLevel = "public"
	PrivacyLevelPrivate PrivacyLevel = "private"
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

func (u *User) Equal(other *User) bool {
	return u.ID == other.ID &&
		u.Name == other.Name &&
		maps.Equal(u.RawMeta, other.RawMeta)
}

type Message struct {
	ID           uuid.UUID
	Text         string
	ImageUrls    []string
	PrivacyLevel PrivacyLevel
	User         *User
	Timestamp    time.Time
	RawMeta      map[ChannelType]any
}

func (m *Message) Equal(other *Message) bool {
	return m.ID == other.ID &&
		m.Text == other.Text &&
		m.PrivacyLevel == other.PrivacyLevel &&
		m.User.Equal(other.User) &&
		m.Timestamp == other.Timestamp &&
		maps.Equal(m.RawMeta, other.RawMeta)
}

type Context struct {
	RunCtx context.Context
}
