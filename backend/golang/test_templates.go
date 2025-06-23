package main

import (
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/testing/personalities"
)

func main() {
	generator := personalities.NewScenarioGenerator()
	library := generator.GetLibrary()

	templates := library.ListTemplates()
	fmt.Printf("Registered templates (%d): %v\n", len(templates), templates)

	// Test each template individually
	for _, templateKey := range []string{"ai_news", "creative_tool", "celebrity_gossip", "startup_funding", "technical_tutorial", "ai_startup_funding"} {
		template, exists := library.GetTemplate(templateKey)
		if exists {
			fmt.Printf("Template '%s': exists=%v, name='%s'\n", templateKey, exists, template.Name)
		} else {
			fmt.Printf("Template '%s': exists=%v, name='<not found>'\n", templateKey, exists)
		}
	}
}
