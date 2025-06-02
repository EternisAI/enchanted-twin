package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

const (
	ContactClassName    = "Contact"
	contactNameProperty = "contact_name"
	tagsProperty        = "tags"
	insightProperty     = "insight"
	createdAtProperty   = "created_at"
	updatedAtProperty   = "updated_at"
)

type contactRecord struct {
	ID          *string    `json:"id,omitempty"`
	ContactName *string    `json:"contact_name,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Insight     *string    `json:"insight,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type Storage struct {
	client *weaviate.Client
	logger *log.Logger
}

func NewStorage(client *weaviate.Client, logger *log.Logger) *Storage {
	return &Storage{
		client: client,
		logger: logger,
	}
}

func EnsureContactSchemaExists(client *weaviate.Client, logger *log.Logger) error {
	ctx := context.Background()
	exists, err := client.Schema().ClassExistenceChecker().WithClassName(ContactClassName).Do(ctx)
	if err != nil {
		return fmt.Errorf("checking class existence for '%s': %w", ContactClassName, err)
	}
	if exists {
		logger.Debugf("Class '%s' already exists.", ContactClassName)
		return nil
	}

	logger.Infof("Class '%s' does not exist, creating it now.", ContactClassName)
	properties := []*models.Property{
		{
			Name:            contactNameProperty,
			DataType:        []string{"text"},
			IndexFilterable: helpers.Ptr(true),
			IndexSearchable: helpers.Ptr(true),
		},
		{
			Name:     tagsProperty,
			DataType: []string{"text[]"},
		},
		{
			Name:     insightProperty,
			DataType: []string{"text"},
		},
		{
			Name:     createdAtProperty,
			DataType: []string{"date"},
		},
		{
			Name:     updatedAtProperty,
			DataType: []string{"date"},
		},
	}

	classObj := &models.Class{
		Class:      ContactClassName,
		Properties: properties,
		Vectorizer: "none",
		VectorIndexConfig: map[string]interface{}{
			"distance": "cosine",
		},
		VectorIndexType: "hnsw",
	}

	err = client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
	if err != nil {
		existsAfterAttempt, checkErr := client.Schema().ClassExistenceChecker().WithClassName(ContactClassName).Do(ctx)
		if checkErr == nil && existsAfterAttempt {
			logger.Info("Class was created concurrently. Proceeding.", "class", ContactClassName)
			return nil
		}
		return fmt.Errorf("creating class '%s': %w", ContactClassName, err)
	}
	logger.Infof("Successfully created class '%s'", ContactClassName)
	return nil
}

func (s *Storage) AddContact(ctx context.Context, contact model.Contact) (string, error) {
	now := time.Now()
	newContact := contactRecord{
		ContactName: &contact.ContactName,
		Tags:        contact.Tags,
		Insight:     contact.Insight,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}

	creator := s.client.Data().Creator().
		WithClassName(ContactClassName).
		WithProperties(newContact)

	if contact.ID != "" {
		creator = creator.WithID(contact.ID)
	}

	res, err := creator.Do(ctx)
	if err != nil {
		return "", fmt.Errorf("creating contact: %w", err)
	}

	return res.Object.ID.String(), nil
}

func (s *Storage) UpdateInsight(ctx context.Context, contactID string, insight string) error {
	now := time.Now()

	// Create a map with only the fields we want to update
	updateProperties := map[string]interface{}{
		insightProperty:   insight,
		updatedAtProperty: now.Format(time.RFC3339),
	}

	err := s.client.Data().Updater().
		WithClassName(ContactClassName).
		WithID(contactID).
		WithProperties(updateProperties).
		WithMerge().
		Do(ctx)
	if err != nil {
		return fmt.Errorf("updating contact insight: %w", err)
	}

	return nil
}

func (s *Storage) GetContact(ctx context.Context, contactID string) (*model.Contact, error) {
	result, err := s.client.Data().ObjectsGetter().
		WithClassName(ContactClassName).
		WithID(contactID).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting contact by ID '%s': %w", contactID, err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("contact with ID '%s' not found", contactID)
	}

	obj := result[0]
	if obj.Properties == nil {
		return nil, fmt.Errorf("contact with ID '%s' has nil properties", contactID)
	}

	var rec contactRecord
	rawProps, _ := json.Marshal(obj.Properties)
	if err := json.Unmarshal(rawProps, &rec); err != nil {
		return nil, fmt.Errorf("decoding properties for %q: %w", contactID, err)
	}

	contact := &model.Contact{
		ID:          obj.ID.String(),
		ContactName: helpers.SafeDeref(rec.ContactName),
		Insight:     rec.Insight,
		Tags:        rec.Tags,
	}

	if rec.CreatedAt != nil {
		contact.CreatedAt = rec.CreatedAt.Format(time.RFC3339)
	}
	if rec.UpdatedAt != nil {
		contact.UpdatedAt = rec.UpdatedAt.Format(time.RFC3339)
	}

	return contact, nil
}

func (s *Storage) ListContacts(ctx context.Context) ([]*model.Contact, error) {
	result, err := s.client.Data().ObjectsGetter().
		WithClassName(ContactClassName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing contacts: %w", err)
	}

	contactsList := make([]*model.Contact, 0, len(result))
	for _, obj := range result {
		if obj.Properties == nil {
			continue
		}

		var rec contactRecord
		rawProps, _ := json.Marshal(obj.Properties)
		if err := json.Unmarshal(rawProps, &rec); err != nil {
			s.logger.Error("Failed to decode contact properties", "error", err, "contactID", obj.ID.String())
			continue
		}

		contact := &model.Contact{
			ID:          obj.ID.String(),
			ContactName: helpers.SafeDeref(rec.ContactName),
			Insight:     rec.Insight,
			Tags:        rec.Tags,
		}

		if rec.CreatedAt != nil {
			contact.CreatedAt = rec.CreatedAt.Format(time.RFC3339)
		}
		if rec.UpdatedAt != nil {
			contact.UpdatedAt = rec.UpdatedAt.Format(time.RFC3339)
		}

		contactsList = append(contactsList, contact)
	}

	return contactsList, nil
}
