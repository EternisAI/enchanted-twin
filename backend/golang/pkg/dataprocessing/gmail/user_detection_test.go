package gmail

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectUserEmailFromMbox(t *testing.T) {
	// Create a test mbox file with sample data similar to the user's export
	sampleMboxContent := `From 1828754568043628583@xxx Mon Apr 07 14:31:02 +0000 2025
X-GM-THIRD: 1828754568043628583
X-Gmail-Labels: =?UTF-8?Q?Forward_to_janedoe@live.fr,Bo=C3=AEte_de_r=C3=A9ception,Non_lus?=
Delivered-To: johndoe@gmail.com
Received: by 2002:a05:622a:68cd:b0:471:9721:748a with SMTP id ic13csp6102107qtb;
        Mon, 7 Apr 2025 07:31:03 -0700 (PDT)
From: "Meetup" <info@meetup.com>
To: johndoe@gmail.com
Subject: Test Email 1
Date: Mon, 07 Apr 2025 14:31:02 +0000

Test content 1

From 1827939445342606497@xxx Sat Mar 29 14:35:00 +0000 2025
X-GM-THIRD: 1827939445342606497
X-Gmail-Labels: =?UTF-8?Q?Forward_to_janedoe@live.fr,Bo=C3=AEte_de_r=C3=A9ception,Non_lus?=
Delivered-To: johndoe@gmail.com
Received: by 2002:a05:622a:68cd:b0:471:9721:748a with SMTP id ic13csp1107003qtb;
        Sat, 29 Mar 2025 07:35:01 -0700 (PDT)
From: "AI Innovators Club" <info@email.meetup.com>
To: johndoe@gmail.com
Subject: Test Email 2
Date: Sat, 29 Mar 2025 14:34:58 +0000

Test content 2
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.mbox")
	err := os.WriteFile(tmpFile, []byte(sampleMboxContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test detecting user email
	userEmail, err := DetectUserEmailFromMbox(tmpFile)
	if err != nil {
		t.Fatalf("Failed to detect user email: %v", err)
	}

	assert.Equal(t, "johndoe@gmail.com", userEmail, "Should detect johndoe@gmail.com as the user email")
}

func TestExtractEmailAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple email",
			input:    "user@example.com",
			expected: "user@example.com",
		},
		{
			name:     "Email with display name",
			input:    "John Doe <user@example.com>",
			expected: "user@example.com",
		},
		{
			name:     "Email with quotes",
			input:    `"John Doe" <user@example.com>`,
			expected: "user@example.com",
		},
		{
			name:     "Complex display name",
			input:    `"Meetup" <info@meetup.com>`,
			expected: "info@meetup.com",
		},
		{
			name:     "Case normalization",
			input:    "USER@EXAMPLE.COM",
			expected: "user@example.com",
		},
		{
			name:     "Invalid input",
			input:    "not an email",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEmailAddress(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMostFrequentEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]int
		expected string
	}{
		{
			name: "Single email",
			input: map[string]int{
				"user@example.com": 5,
			},
			expected: "user@example.com",
		},
		{
			name: "Multiple emails with clear winner",
			input: map[string]int{
				"user@example.com":    10,
				"another@example.com": 2,
				"third@example.com":   1,
			},
			expected: "user@example.com",
		},
		{
			name:     "Empty input",
			input:    map[string]int{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMostFrequentEmail(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSamplingStrategy(t *testing.T) {
	// Create a test mbox file with more emails than MaxSampleSize
	var mboxBuilder strings.Builder

	// Add 1200 emails (more than MaxSampleSize of 1000)
	for i := 0; i < 1200; i++ {
		mboxBuilder.WriteString(fmt.Sprintf(`From %d@xxx Mon Apr 07 14:31:02 +0000 2025
Delivered-To: johndoe@gmail.com
From: "Service %d" <service%d@example.com>
To: johndoe@gmail.com
Subject: Test Email %d
Date: Mon, 07 Apr 2025 14:31:02 +0000

Test content %d

`, i, i, i, i, i))
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "large_test.mbox")
	err := os.WriteFile(tmpFile, []byte(mboxBuilder.String()), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test user detection with sampling
	userEmail, err := DetectUserEmailFromMbox(tmpFile)
	if err != nil {
		t.Fatalf("Failed to detect user email: %v", err)
	}
	assert.Equal(t, "johndoe@gmail.com", userEmail, "Should detect johndoe@gmail.com as the user email")
}
