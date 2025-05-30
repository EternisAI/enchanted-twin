package prompts

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed templates/poke_message_prompt.tmpl
var pokeMessagePromptTemplate string

type PokeMessagePrompt struct {
	Identity string
	Memories string
}

func BuildPokeMessagePrompt(data PokeMessagePrompt) (string, error) {
	tmpl, err := template.New("poke_message").Parse(pokeMessagePromptTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
