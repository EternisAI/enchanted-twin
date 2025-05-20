package prompts

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed templates/scheduled_task_system_prompt.tmpl
var scheduledTaskSystemPromptTemplate string

type ScheduledTaskSystemPrompt struct {
	UserName       *string
	Bio            *string
	ChatID         *string
	CurrentTime    string
	EmailAccounts  []string
	PreviousResult *string
}

func BuildScheduledTaskSystemPrompt(data ScheduledTaskSystemPrompt) (string, error) {
	systemPromptTmpl := template.Must(template.New("system_prompt").Parse(scheduledTaskSystemPromptTemplate))
	var buf bytes.Buffer
	if err := systemPromptTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
