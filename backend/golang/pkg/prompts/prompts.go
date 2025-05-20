package prompts

import (
	_ "embed"
)

type PromptsService struct{}

func NewPromptsService() *PromptsService {
	return &PromptsService{}
}
