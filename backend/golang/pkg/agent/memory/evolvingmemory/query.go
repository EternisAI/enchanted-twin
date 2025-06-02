package evolvingmemory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/weaviate/weaviate-go-client/v5/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// Query retrieves memories relevant to the query text.
func (s *WeaviateStorage) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	// var filterBySpeakerID string
	// if opts, ok := options.(map[string]interface{}); ok {
	// 	if speakerToFilter, okS := opts["speakerID"].(string); okS && speakerToFilter != "" {
	// 		filterBySpeakerID = speakerToFilter
	// 		s.logger.Info("Query results will be filtered in Go by speakerID", "speakerID", filterBySpeakerID)
	// 	}
	// }

	s.logger.Info("Query method called", "query_text", queryText, "filter", filter)

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
	metaField := graphql.Field{
		Name: metadataProperty,
		Fields: []graphql.Field{
			{Name: sourceProperty},
			{Name: contactNameProperty},
			{Name: "speakerID"},
		},
	}
	tagsField := graphql.Field{Name: tagsProperty}
	additionalFields := graphql.Field{
		Name: "_additional",
		Fields: []graphql.Field{
			{Name: "id"},
			{Name: "distance"},
		},
	}

	limit := 10
	if filter != nil && filter.Limit != nil {
		limit = *filter.Limit
	}

	if filter != nil && filter.Distance > 0 {
		nearVector = nearVector.WithDistance(filter.Distance)
		s.logger.Debug("Added distance filter", "distance", filter.Distance)
	} else {
		nearVector = nearVector.WithDistance(1.0)
	}

	queryBuilder := s.client.GraphQL().Get().
		WithClassName(ClassName).
		WithNearVector(nearVector).
		WithLimit(limit).
		WithFields(contentField, timestampField, metaField, tagsField, additionalFields)

	// Add WHERE filtering if filter is provided
	if filter != nil {
		var whereFilters []*filters.WhereBuilder

		if filter.Source != nil {
			sourceFilter := filters.Where().
				WithPath([]string{sourceProperty}).
				WithOperator(filters.Equal).
				WithValueText(*filter.Source)
			whereFilters = append(whereFilters, sourceFilter)
			s.logger.Debug("Added source filter", "source", *filter.Source)
		}

		if filter.ContactName != nil {
			contactFilter := filters.Where().
				WithPath([]string{contactNameProperty}).
				WithOperator(filters.Equal).
				WithValueText(*filter.ContactName)
			whereFilters = append(whereFilters, contactFilter)
			s.logger.Debug("Added contact name filter", "contactName", *filter.ContactName)
		}

		if len(whereFilters) > 0 {
			combinedFilter := filters.Where().
				WithOperator(filters.And).
				WithOperands(whereFilters)
			queryBuilder = queryBuilder.WithWhere(combinedFilter)
			s.logger.Debug("Applied combined WHERE filters", "filter_count", len(whereFilters))
		}
	}

	resp, err := queryBuilder.Do(ctx)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to execute Weaviate query: %w", err)
	}

	if len(resp.Errors) > 0 {
		var errorMessages []string
		for _, err := range resp.Errors {
			if err != nil && err.Message != "" {
				errorMessages = append(errorMessages, err.Message)
			}
		}
		return memory.QueryResult{}, fmt.Errorf("GraphQL query errors: %s", strings.Join(errorMessages, "; "))
	}

	finalResults := []memory.TextDocument{}
	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		s.logger.Warn("No 'Get' field in GraphQL response or not a map.")
		return memory.QueryResult{Facts: []memory.MemoryFact{}, Documents: finalResults}, nil
	}

	classData, ok := data[ClassName].([]interface{})
	if !ok {
		s.logger.Warn("No class data in GraphQL response or not a slice.", "class_name", ClassName)
		return memory.QueryResult{Facts: []memory.MemoryFact{}, Documents: finalResults}, nil
	}
	s.logger.Info("Retrieved documents from Weaviate (pre-filtering)", "count", len(classData))

	for _, item := range classData {
		obj, okMap := item.(map[string]interface{})
		if !okMap {
			s.logger.Warn("Retrieved item is not a map, skipping", "item", item)
			continue
		}

		content, _ := obj[contentProperty].(string)

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
		if metadataObj, metaOk := obj[metadataProperty].(map[string]interface{}); metaOk {
			for key, value := range metadataObj {
				if strValue, ok := value.(string); ok {
					metaMap[key] = strValue
				}
			}
		}

		var tags []string
		if tagsInterface, tagsOk := obj[tagsProperty].([]interface{}); tagsOk {
			for _, tagInterfaceItem := range tagsInterface {
				if tagStr, okTag := tagInterfaceItem.(string); okTag {
					tags = append(tags, tagStr)
				}
			}
		}

		finalResults = append(finalResults, memory.TextDocument{
			FieldID:        id,
			FieldContent:   content,
			FieldTimestamp: parsedTimestamp,
			FieldMetadata:  metaMap,
			FieldTags:      tags,
		})
	}
	s.logger.Info("Query processed successfully.", "num_results_returned_after_filtering", len(finalResults))
	return memory.QueryResult{Facts: []memory.MemoryFact{}, Documents: finalResults}, nil
}

// QueryWithDistance retrieves memories relevant to the query text with similarity distances, with optional metadata filtering.
func (s *WeaviateStorage) QueryWithDistance(ctx context.Context, queryText string, metadataFilters ...map[string]string) (memory.QueryWithDistanceResult, error) {
	s.logger.Info("QueryWithDistance method called", "query_text", queryText, "filters", metadataFilters)

	vector, err := s.embeddingsService.Embedding(ctx, queryText, openAIEmbedModel)
	if err != nil {
		return memory.QueryWithDistanceResult{}, fmt.Errorf("failed to create embedding for query: %w", err)
	}
	queryVector32 := make([]float32, len(vector))
	for i, val := range vector {
		queryVector32[i] = float32(val)
	}

	nearVector := s.client.GraphQL().NearVectorArgBuilder().WithVector(queryVector32)

	contentField := graphql.Field{Name: contentProperty}
	timestampField := graphql.Field{Name: timestampProperty}
	metaField := graphql.Field{
		Name: metadataProperty,
		Fields: []graphql.Field{
			{Name: sourceProperty},
			{Name: contactNameProperty},
			{Name: "speakerID"},
		},
	}
	tagsField := graphql.Field{Name: tagsProperty}
	additionalFields := graphql.Field{
		Name: "_additional",
		Fields: []graphql.Field{
			{Name: "id"},
			{Name: "distance"},
		},
	}

	queryBuilder := s.client.GraphQL().Get().
		WithClassName(ClassName).
		WithNearVector(nearVector).
		WithLimit(10).
		WithFields(contentField, timestampField, metaField, tagsField, additionalFields)

	// Add WHERE filtering if metadata filters are provided
	if len(metadataFilters) > 0 {
		filterMap := metadataFilters[0] // Use first filter map if provided
		for key, value := range filterMap {
			if key == "type" {
				// Filter by nested property in metadata object
				whereFilter := filters.Where().
					WithPath([]string{metadataProperty, key}).
					WithOperator(filters.Equal).
					WithValueText(value)

				queryBuilder = queryBuilder.WithWhere(whereFilter)
				s.logger.Debug("Added WHERE filter for metadata object", "key", key, "value", value)
			}
		}
	}

	resp, err := queryBuilder.Do(ctx)
	if err != nil {
		return memory.QueryWithDistanceResult{}, fmt.Errorf("failed to execute Weaviate query: %w", err)
	}

	if len(resp.Errors) > 0 {
		return memory.QueryWithDistanceResult{}, fmt.Errorf("GraphQL query errors: %v", resp.Errors)
	}

	finalResults := []memory.DocumentWithDistance{}
	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		s.logger.Warn("No 'Get' field in GraphQL response or not a map.")
		return memory.QueryWithDistanceResult{Documents: finalResults}, nil
	}

	classData, ok := data[ClassName].([]interface{})
	if !ok {
		s.logger.Warn("No class data in GraphQL response or not a slice.", "class_name", ClassName)
		return memory.QueryWithDistanceResult{Documents: finalResults}, nil
	}
	s.logger.Info("Retrieved documents from Weaviate with distances", "count", len(classData), "with_filters", len(metadataFilters) > 0)

	for _, item := range classData {
		obj, okMap := item.(map[string]interface{})
		if !okMap {
			s.logger.Warn("Retrieved item is not a map, skipping", "item", item)
			continue
		}

		content, _ := obj[contentProperty].(string)

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
		distance, _ := additional["distance"].(float64)

		// Handle metadata as object with nested properties
		metaMap := make(map[string]string)
		if metadataObj, metaOk := obj[metadataProperty].(map[string]interface{}); metaOk {
			// Extract all metadata fields
			for key, value := range metadataObj {
				if strValue, ok := value.(string); ok {
					metaMap[key] = strValue
				}
			}
		}

		var tags []string
		if tagsInterface, tagsOk := obj[tagsProperty].([]interface{}); tagsOk {
			for _, tagInterfaceItem := range tagsInterface {
				if tagStr, okTag := tagInterfaceItem.(string); okTag {
					tags = append(tags, tagStr)
				}
			}
		}

		finalResults = append(finalResults, memory.DocumentWithDistance{
			Document: memory.TextDocument{
				FieldID:        id,
				FieldContent:   content,
				FieldTimestamp: parsedTimestamp,
				FieldMetadata:  metaMap,
				FieldTags:      tags,
			},
			Distance: float32(distance),
		})
	}
	s.logger.Info("QueryWithDistance processed successfully.", "num_results_returned", len(finalResults))
	return memory.QueryWithDistanceResult{Documents: finalResults}, nil
}
