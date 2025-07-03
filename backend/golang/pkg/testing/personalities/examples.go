//go:build test
// +build test

package personalities

// Example demonstrates how to use the personality testing framework
func ExamplePersonalityTesting() {
	// This function serves as documentation and makes the test code "reachable"

	// Create framework
	_ = NewPersonalityTestFramework(nil, nil, "testdata")

	// Create scenarios
	_ = NewChatMessageScenario("example", "description")
	_ = NewEmailScenario("example", "description")
	_ = NewSocialPostScenario("example", "description")

	// Use memory utilities
	_ = NewMemoryFactBuilder()
	_ = NewPersonalityMemoryBuilder("example_personality")

	// Create mock storage
	_ = NewMockMemoryStorage()
	_ = NewMockHolonRepository()
}
