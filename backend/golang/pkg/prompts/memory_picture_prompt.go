package prompts

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed templates/memory_picture_prompt.tmpl
var memoryPicturePromptTemplate string

type MemoryPicturePrompt struct {
	Memory   string
	Identity string
}

func BuildMemoryPicturePrompt(data MemoryPicturePrompt) (string, error) {
	tmpl, err := template.New("memory_picture").Parse(memoryPicturePromptTemplate)
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
