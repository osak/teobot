package text

import (
	"strings"
	"unicode/utf8"
)

type Text struct {
	raw        string
	runeLength int
}

func New(raw string) *Text {
	return &Text{
		raw:        raw,
		runeLength: -1, // Calculate lazily
	}
}

// Len returns the length of the text in runes.
func (t *Text) Len() int {
	if t.runeLength == -1 {
		t.runeLength = utf8.RuneCountInString(t.raw)
	}
	return t.runeLength
}

// Substring returns a raw string (implements Stringer)
func (t *Text) String() string {
	return t.raw
}

func (t *Text) ConcatString(s string) *Text {
	return New(t.raw + s)
}

func Concat(t *Text, u *Text) *Text {
	return New(t.String() + u.String())
}

func ConcatString(t *Text, s string) *Text {
	return New(t.String() + s)
}

func (t *Text) Split(sep string) []*Text {
	parts := strings.Split(t.raw, sep)
	res := make([]*Text, len(parts))
	for i, part := range parts {
		res[i] = New(part)
	}
	return res
}

func ReplaceAll(s *Text, old, new string) *Text {
	return New(strings.ReplaceAll(s.String(), old, new))
}
