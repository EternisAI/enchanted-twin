package evolvingmemory

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate/entities/models"
)

// EnsureSchemaExistsInternal ensures the Weaviate schema exists for the Memory class.
func EnsureSchemaExistsInternal(client *weaviate.Client, logger *log.Logger) error {
	// First, check if schema already exists
	schema, err := client.Schema().Getter().Do(context.Background())
	if err != nil {
		return fmt.Errorf("getting schema: %w", err)
	}

	// Check if our class already exists
	for _, class := range schema.Classes {
		if class.Class == ClassName {
			// Schema exists, validate it matches our expectations
			logger.Infof("Schema for class %s already exists, validating...", ClassName)

			// Basic validation - check properties exist
			expectedProps := map[string]string{
				contentProperty:   "text",
				timestampProperty: "date",
				metadataProperty:  "text",
			}

			existingProps := make(map[string]string)
			for _, prop := range class.Properties {
				for _, dt := range prop.DataType {
					existingProps[prop.Name] = dt
				}
			}

			// Check if all expected properties exist
			for propName, propType := range expectedProps {
				if existingType, exists := existingProps[propName]; !exists {
					return fmt.Errorf("missing property %s in existing schema", propName)
				} else if existingType != propType {
					return fmt.Errorf("property %s has type %s, expected %s", propName, existingType, propType)
				}
			}

			logger.Info("Schema validation successful")
			return nil
		}
	}

	// Schema doesn't exist, create it
	logger.Infof("Creating schema for class %s", ClassName)

	classObj := &models.Class{
		Class:       ClassName,
		Description: "A memory entry in the evolving memory system",
		Properties: []*models.Property{
			{
				Name:        contentProperty,
				DataType:    []string{"text"},
				Description: "The content of the memory",
			},
			{
				Name:        timestampProperty,
				DataType:    []string{"date"},
				Description: "When this memory was created or last updated",
			},
			{
				Name:        metadataProperty,
				DataType:    []string{"text"},
				Description: "JSON-encoded metadata for the memory",
			},
		},
		Vectorizer: "none", // We provide our own vectors
	}

	err = client.Schema().ClassCreator().WithClass(classObj).Do(context.Background())
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	logger.Info("Schema created successfully")
	return nil
}
