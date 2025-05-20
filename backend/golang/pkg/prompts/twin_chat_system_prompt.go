package prompts

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed templates/twin_chat_system_prompt.tmpl
var twinChatSystemPromptTemplate string

type TwinChatSystemPrompt struct {
	UserName      *string
	Bio           *string
	ChatID        *string
	CurrentTime   string
	EmailAccounts []string
	IsVoice       bool
}

func BuildTwinChatSystemPrompt(data TwinChatSystemPrompt) (string, error) {
	systemPromptTmpl := template.Must(template.New("system_prompt").Parse(twinChatSystemPromptTemplate))
	var buf bytes.Buffer
	if err := systemPromptTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
