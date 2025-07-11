package llama1b

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLlamaAnonymizer(t *testing.T) {
	appDataPath := os.Getenv("LLAMA_APP_DATA_PATH")
	if appDataPath == "" {
		t.Skip("LLAMA_APP_DATA_PATH not set")
	}

	sharedLibraryPath := os.Getenv("LLAMA_SHARED_LIB_PATH")
	if sharedLibraryPath == "" {
		t.Skip("LLAMA_SHARED_LIB_PATH not set")
	}

	anonymizer, err := NewLlamaAnonymizer(appDataPath, sharedLibraryPath)
	assert.NoError(t, err)
	defer func() { _ = anonymizer.Close() }()

	input := "I am John"
	start := time.Now()
	result, err := anonymizer.Anonymize(context.Background(), input)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	t.Logf("Input: %s", input)
	t.Logf("Anonymization Result: %v", result)
	t.Logf("Anonymization time: %v", elapsed)

	// Test second call to verify interactive session reuse
	input2 := "I am Emily"
	start = time.Now()
	result2, err := anonymizer.Anonymize(context.Background(), input2)
	elapsed = time.Since(start)

	assert.NoError(t, err)
	assert.NotNil(t, result2)

	t.Logf("Input2: %s", input2)
	t.Logf("Anonymization Result2: %v", result2)
	t.Logf("Second anonymization time: %v", elapsed)
}
