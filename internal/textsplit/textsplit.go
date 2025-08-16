package textsplit

import (
	"github.com/osak/teobot/internal/text"
)

// TextSplitService provides text splitting functionality
type TextSplitService struct {
	// We're not actually using ChatGPT, this is kept for API compatibility
	// with the rest of the code that expects this dependency
}

// NewTextSplitService creates a new text splitting service
func NewTextSplitService(_ interface{}) *TextSplitService {
	return &TextSplitService{}
}

// SplitText splits a text into multiple chunks using a simple line-based approach
func (tss *TextSplitService) SplitText(t *text.Text, maxPartLen int) ([]*text.Text, error) {
	// Split the text into lines
	lines := t.Split("\n")

	// Initialize the result with first empty part
	parts := []*text.Text{text.New("")}

	// Distribute lines to parts
	for _, line := range lines {
		lastIdx := len(parts) - 1

		// If adding the line to the current part doesn't exceed maxPartLen, add it
		if parts[lastIdx].Len()+1+line.Len() <= maxPartLen {
			// Only add newline if the part isn't empty
			if parts[lastIdx].Len() > 0 {
				parts[lastIdx] = text.ConcatString(parts[lastIdx], "\n")
			}
			parts[lastIdx] = text.Concat(parts[lastIdx], line)
		} else {
			// Otherwise, start a new part
			parts = append(parts, line)
		}
	}

	return parts, nil
}
