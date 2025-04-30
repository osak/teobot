package textsplit

import (
	"strings"
	"unicode/utf8"
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
func (t *TextSplitService) SplitText(text string, numParts int) ([]string, error) {
	// Split the text into lines
	lines := strings.Split(text, "\n")

	// Calculate maximum length per part
	maxPartLen := (utf8.RuneCountInString(text) + numParts - 1) / numParts // Equivalent to Math.ceil(text.length / numParts)

	// Initialize the result with first empty part
	parts := []string{""}

	// Distribute lines to parts
	for _, line := range lines {
		lastIdx := len(parts) - 1

		// If adding the line to the current part doesn't exceed maxPartLen, add it
		if len(parts[lastIdx])+1+utf8.RuneCountInString(line) < maxPartLen {
			// Only add newline if the part isn't empty
			if len(parts[lastIdx]) > 0 {
				parts[lastIdx] += "\n"
			}
			parts[lastIdx] += line
		} else {
			// Otherwise, start a new part
			parts = append(parts, line)
		}
	}

	return parts, nil
}
