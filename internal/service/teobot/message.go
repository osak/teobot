package teobot

import (
	"fmt"
	"strings"

	"github.com/osak/teobot/internal/service/chatgpt"
)

func (m *Message) ToInputText() string {
	builder := strings.Builder{}
	builder.WriteString(m.Text)
	builder.WriteString("\n")
	for ch, meta := range m.RawMeta {
		builder.WriteString(fmt.Sprintf("<metadata channel=\"%s\">\n", ch))
		if m, ok := meta.(string); ok {
			builder.WriteString(m)
			builder.WriteString("\n")
		}
		builder.WriteString("</metadata>\n")
	}
	return builder.String()
}

func (m *Message) ToChatGptMessage(role chatgpt.Role) chatgpt.Message {
	var contents []chatgpt.MessageContent
	if role == chatgpt.RoleAssistant {
		contents = append(contents, chatgpt.OutputText{Text: m.ToInputText()})
	} else {
		contents = append(contents, chatgpt.InputText{Text: m.ToInputText()})
	}
	for _, url := range m.ImageUrls {
		contents = append(contents, chatgpt.InputImage{ImageUrl: url, Detail: "auto"})
	}
	return chatgpt.Message{
		Role:    string(role),
		Content: contents,
	}
}
