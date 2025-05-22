package prompts

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed templates/identity_personality_system_prompt.tmpl
var identityPersonalitySystemPromptTemplate string

func BuildIdentityPersonalitySystemPrompt() (string, error) {
	tmpl := template.Must(template.New("identity_personality_system_prompt").Parse(identityPersonalitySystemPromptTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return "", err
	}
	return buf.String(), nil
}
