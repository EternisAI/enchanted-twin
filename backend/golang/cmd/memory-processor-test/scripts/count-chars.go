package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type PromptEntry struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	Content string `json:"content"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run count-chars.go <prompts.jsonl>")
		fmt.Println("Example: go run count-chars.go ../pipeline_output/X_2_chatgpt_prompts.jsonl")
		os.Exit(1)
	}

	filename := os.Args[1]

	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large email content
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size
	totalChars := 0
	lineCount := 0

	fmt.Printf("📊 Counting characters in: %s\n", filename)
	fmt.Println()

	for scanner.Scan() {
		lineCount++

		var prompt PromptEntry
		if err := json.Unmarshal(scanner.Bytes(), &prompt); err != nil {
			fmt.Printf("Error parsing line %d: %v\n", lineCount, err)
			continue
		}

		charCount := len(prompt.Content)
		totalChars += charCount

		// Show progress every 50 entries
		if lineCount%50 == 0 {
			fmt.Printf("📝 Processed %d entries... (running total: %s chars)\n",
				lineCount, formatNumber(totalChars))
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("✅ Results for %s:\n", filename)
	fmt.Printf("   📄 Total entries: %s\n", formatNumber(lineCount))
	fmt.Printf("   🔤 Total characters: %s\n", formatNumber(totalChars))
	fmt.Printf("   📊 Average chars per entry: %s\n", formatNumber(totalChars/max(lineCount, 1)))
	fmt.Printf("   📏 Estimated size: %.2f MB\n", float64(totalChars)/1024/1024)
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	} else {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
