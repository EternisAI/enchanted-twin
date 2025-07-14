package prompts

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed templates/onboarding_system_prompt.tmpl
var onboardingSystemPromptTemplate string

type OnboardingSystemPrompt struct {
}

func BuildOnboardingSystemPrompt(data OnboardingSystemPrompt) (string, error) {
	systemPromptTmpl := template.Must(template.New("onboarding_system_prompt").Parse(onboardingSystemPromptTemplate))
	var buf bytes.Buffer
	if err := systemPromptTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
