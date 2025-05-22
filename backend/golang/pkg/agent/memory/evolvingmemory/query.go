package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// Query retrieves memories relevant to the query text.
func (s *WeaviateStorage) Query(ctx context.Context, queryText string) (memory.QueryResult, error) {
	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to ensure schema before querying: %w", err)
	}

	// var filterBySpeakerID string
	// if opts, ok := options.(map[string]interface{}); ok {
	// 	if speakerToFilter, okS := opts["speakerID"].(string); okS && speakerToFilter != "" {
	// 		filterBySpeakerID = speakerToFilter
	// 		s.logger.Info("Query results will be filtered in Go by speakerID", "speakerID", filterBySpeakerID)
	// 	}
	// }

	s.logger.Info("Query method called", "query_text", queryText)

	vector, err := s.embeddingsService.Embedding(ctx, queryText, openAIEmbedModel)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to create embedding for query: %w", err)
	}
	queryVector32 := make([]float32, len(vector))
	for i, val := range vector {
		queryVector32[i] = float32(val)
	}

	nearVector := s.client.GraphQL().NearVectorArgBuilder().WithVector(queryVector32)

	contentField := graphql.Field{Name: contentProperty}
	timestampField := graphql.Field{Name: timestampProperty}
	metaField := graphql.Field{Name: metadataProperty}
	tagsField := graphql.Field{Name: tagsProperty}
	additionalFields := graphql.Field{
		Name: "_additional",
		Fields: []graphql.Field{
			{Name: "id"},
			{Name: "distance"},
		},
	}

	queryBuilder := s.client.GraphQL().Get().
		WithClassName(className).
		WithNearVector(nearVector).
		WithLimit(10).
		WithFields(contentField, timestampField, metaField, tagsField, additionalFields)

	resp, err := queryBuilder.Do(ctx)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to execute Weaviate query: %w", err)
	}

	if len(resp.Errors) > 0 {
		return memory.QueryResult{}, fmt.Errorf("GraphQL query errors: %v", resp.Errors)
	}

	finalResults := []memory.TextDocument{}
	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		s.logger.Warn("No 'Get' field in GraphQL response or not a map.")
		return memory.QueryResult{Documents: finalResults}, nil
	}

	classData, ok := data[className].([]interface{})
	if !ok {
		s.logger.Warn("No class data in GraphQL response or not a slice.", "class_name", className)
		return memory.QueryResult{Documents: finalResults}, nil
	}
	s.logger.Info("Retrieved documents from Weaviate (pre-filtering)", "count", len(classData))

	for _, item := range classData {
		obj, okMap := item.(map[string]interface{})
		if !okMap {
			s.logger.Warn("Retrieved item is not a map, skipping", "item", item)
			continue
		}

		content, _ := obj[contentProperty].(string)
		metadataJSON, _ := obj[metadataProperty].(string)

		var parsedTimestamp *time.Time
		if tsStr, tsOk := obj[timestampProperty].(string); tsOk {
			t, pErr := time.Parse(time.RFC3339, tsStr)
			if pErr == nil {
				parsedTimestamp = &t
			} else {
				s.logger.Warn("Failed to parse timestamp from Weaviate", "timestamp_str", tsStr, "error", pErr)
			}
		}

		additional, _ := obj["_additional"].(map[string]interface{})
		id, _ := additional["id"].(string)

		metaMap := make(map[string]string)
		if metadataJSON != "" {
			if errJson := json.Unmarshal([]byte(metadataJSON), &metaMap); errJson != nil {
				s.logger.Debug("Could not unmarshal metadataJson for retrieved doc, using empty map", "id", id, "error", errJson)
			}
		}

		// docSpeakerID := metaMap["speakerID"]

		// if filterBySpeakerID != "" && docSpeakerID != filterBySpeakerID {
		// 	s.logger.Debug("Document filtered out by speakerID mismatch", "doc_id", id, "doc_speaker_id", docSpeakerID, "filter_speaker_id", filterBySpeakerID)
		// 	continue
		// }

		var tags []string
		if tagsInterface, tagsOk := obj[tagsProperty].([]interface{}); tagsOk {
			for _, tagInterfaceItem := range tagsInterface {
				if tagStr, okTag := tagInterfaceItem.(string); okTag {
					tags = append(tags, tagStr)
				}
			}
		}

		finalResults = append(finalResults, memory.TextDocument{
			ID:        id,
			Content:   content,
			Timestamp: parsedTimestamp,
			Metadata:  metaMap,
			Tags:      tags,
		})
	}
	s.logger.Info("Query processed successfully.", "num_results_returned_after_filtering", len(finalResults))
	return memory.QueryResult{Documents: finalResults}, nil
}
