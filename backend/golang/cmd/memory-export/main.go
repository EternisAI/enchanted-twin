package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

func main() {
	var (
		action     = flag.String("action", "export", "Action: export or import")
		outputFile = flag.String("output", "", "Output JSONL file for export")
		inputFile  = flag.String("input", "", "Input JSONL file for import")
		count      = flag.Int("count", 5, "Number of sample facts to create (export only)")
	)
	flag.Parse()

	switch *action {
	case "export":
		if *outputFile == "" {
			timestamp := time.Now().Format("20060102-150405")
			*outputFile = fmt.Sprintf("memory-export-%s.jsonl", timestamp)
		}
		exportSampleMemoryFacts(*outputFile, *count)
	case "import":
		if *inputFile == "" {
			fmt.Println("❌ Input file is required for import action")
			os.Exit(1)
		}
		importMemoryFacts(*inputFile)
	default:
		fmt.Printf("❌ Unknown action: %s (use 'export' or 'import')\n", *action)
		os.Exit(1)
	}
}

func exportSampleMemoryFacts(outputFile string, count int) {
	fmt.Printf("🔄 Creating %d sample memory facts and exporting to: %s\n", count, outputFile)

	facts := make([]memory.MemoryFact, count)
	categories := []string{"health", "career", "preferences", "goals", "relationships"}
	subjects := []string{"Physical Health & Fitness", "Career & Professional Life", "Media & Culture Tastes", "Goals & Projects", "Friends & Social Network"}

	for i := 0; i < count; i++ {
		facts[i] = memory.MemoryFact{
			ID:          uuid.New().String(),
			Content:     fmt.Sprintf("Sample memory fact #%d about personal interests and activities", i+1),
			Category:    categories[i%len(categories)],
			Subject:     subjects[i%len(subjects)],
			Attribute:   fmt.Sprintf("sample_attribute_%d", i+1),
			Value:       fmt.Sprintf("sample value %d", i+1),
			Importance:  (i % 3) + 1,
			Sensitivity: []string{"low", "medium", "high"}[i%3],
			Timestamp:   time.Now().Add(-time.Duration(i*24) * time.Hour),
			Source:      "memory_export_tool",
			Metadata: map[string]string{
				"tool":   "memory_export_tool",
				"sample": "true",
				"index":  fmt.Sprintf("%d", i+1),
			},
		}
	}

	if err := memory.ExportMemoryFactsJSON(facts, outputFile); err != nil {
		fmt.Printf("❌ Failed to export memory facts: %v\n", err)
		return
	}

	fmt.Printf("✅ Successfully exported %d sample memory facts to %s\n", len(facts), outputFile)
	fmt.Printf("🔧 This demonstrates the ExportMemoryFactsJSON utility function usage.\n")

	// Show statistics
	categoriesCount := make(map[string]int)
	subjectsCount := make(map[string]int)
	for _, fact := range facts {
		categoriesCount[fact.Category]++
		subjectsCount[fact.Subject]++
	}

	fmt.Printf("\n📈 Export Statistics:\n")
	fmt.Printf("Categories: %d unique\n", len(categoriesCount))
	fmt.Printf("Subjects: %d unique\n", len(subjectsCount))

	// Show categories
	fmt.Printf("\nCategories:\n")
	for cat, count := range categoriesCount {
		fmt.Printf("  %s: %d facts\n", cat, count)
	}
}

func importMemoryFacts(inputFile string) {
	fmt.Printf("🔄 Importing memory facts from: %s\n", inputFile)

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		fmt.Printf("❌ Input file does not exist: %s\n", inputFile)
		return
	}

	facts, err := memory.LoadMemoryFactsFromJSON(inputFile)
	if err != nil {
		fmt.Printf("❌ Failed to load memory facts: %v\n", err)
		return
	}

	fmt.Printf("📊 Loaded %d memory facts from file\n", len(facts))
	fmt.Printf("🔧 This demonstrates the LoadMemoryFactsFromJSON utility function usage.\n")

	if len(facts) == 0 {
		fmt.Println("⚠️  No facts found in file")
		return
	}

	categories := make(map[string]int)
	subjects := make(map[string]int)
	for _, fact := range facts {
		categories[fact.Category]++
		subjects[fact.Subject]++
		fmt.Printf("📝 Loaded fact: %s (Category: %s, Subject: %s)\n",
			fact.ID, fact.Category, fact.Subject)
	}

	fmt.Printf("\n📈 Import Statistics:\n")
	fmt.Printf("Categories: %d unique\n", len(categories))
	fmt.Printf("Subjects: %d unique\n", len(subjects))

	fmt.Printf("\nCategories:\n")
	for cat, count := range categories {
		fmt.Printf("  %s: %d facts\n", cat, count)
	}

	fmt.Printf("✅ Successfully loaded and analyzed %d memory facts\n", len(facts))
	fmt.Printf("ℹ️  Note: This tool demonstrates import functionality - actual storage would require additional setup\n")
}
