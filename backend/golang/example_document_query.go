// Example usage of the new QueryDocuments functionality
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

func main() {
	// This is a demonstration of how to use the new QueryDocuments functionality
	// Note: This is example code and would need proper initialization of storage
	
	ctx := context.Background()
	
	// Example 1: Query all conversation documents from Slack
	filter1 := memory.NewDocumentQuery().
		WithDocumentTypes("conversation").
		WithSource("slack").
		WithLimit(50).
		Build()
	
	fmt.Printf("Filter 1 - Slack conversations:\n%+v\n\n", filter1)
	
	// Example 2: Query all chunks of a specific document
	filter2 := memory.NewDocumentQuery().
		ChunksOnly().
		WithOriginalID("meeting-notes-2024-01-15").
		Build()
	
	fmt.Printf("Filter 2 - Document chunks:\n%+v\n\n", filter2)
	
	// Example 3: Query documents with metadata filters
	filter3 := memory.NewDocumentQuery().
		WithMetadata("department", "engineering").
		WithMetadata("project", "ai-assistant").
		WithLimit(20).
		Build()
	
	fmt.Printf("Filter 3 - Metadata filtered:\n%+v\n\n", filter3)
	
	// Example 4: Query original documents only (no chunks)
	filter4 := memory.NewDocumentQuery().
		OriginalsOnly().
		WithDocumentTypes("text", "conversation").
		Build()
	
	fmt.Printf("Filter 4 - Original documents only:\n%+v\n\n", filter4)
	
	// Example 5: Query documents by content hash (deduplication)
	filter5 := memory.NewDocumentQuery().
		WithContentHash("abc123...").
		Build()
	
	fmt.Printf("Filter 5 - By content hash:\n%+v\n\n", filter5)
	
	// Usage with actual storage (commented out as it requires proper setup):
	// var storage memory.Storage
	// result, err := storage.QueryDocuments(ctx, filter1)
	// if err != nil {
	//     log.Fatalf("Query failed: %v", err)
	// }
	// 
	// fmt.Printf("Found %d documents (total: %d, hasMore: %t)\n", 
	//     len(result.Documents), result.Total, result.HasMore)
	// 
	// for _, doc := range result.Documents {
	//     fmt.Printf("Document: %s (type: %s, original: %s)\n", 
	//         doc.ID, doc.DocumentType, doc.OriginalID)
	//     
	//     if doc.IsChunk {
	//         fmt.Printf("  -> Chunk %d of %s\n", *doc.ChunkNumber, *doc.OriginalDocID)
	//     }
	// }
	
	log.Println("Document query examples completed!")
}