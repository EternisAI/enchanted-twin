package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcweaviate "github.com/testcontainers/testcontainers-go/modules/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

func TestWeaviateStorage(t *testing.T) {
	ctx := context.Background()

	weaviateContainer, err := tcweaviate.Run(ctx, "semitechnologies/weaviate:1.30.6")
	defer func() {
		if err := testcontainers.TerminateContainer(weaviateContainer); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}()
	require.NoError(t, err, "failed to start weaviate container")

	host, err := weaviateContainer.Host(ctx)
	require.NoError(t, err)

	port, err := weaviateContainer.MappedPort(ctx, "8080")
	require.NoError(t, err)

	client, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("%s:%s", host, port.Port()),
		Scheme: "http",
	})
	require.NoError(t, err, "failed to create weaviate client")

	logger := log.Default()

	err = EnsureContactSchemaExists(client, logger)
	require.NoError(t, err, "failed to ensure schema exists")

	storage := NewStorage(client, logger)

	t.Run("AddContact", func(t *testing.T) {
		insight := "Loves discussing technology and innovation"
		contact := model.Contact{
			ContactName: "John Doe",
			Tags:        []string{"friend", "work"},
			Insight:     &insight,
		}

		contactID, err := storage.AddContact(ctx, contact)
		assert.NoError(t, err, "failed to add contact")
		assert.NotEmpty(t, contactID, "contact ID should not be empty")
		t.Logf("Created contact with ID: %s", contactID)
	})

	t.Run("ListContacts", func(t *testing.T) {
		insight := "Enjoys outdoor activities and hiking"
		contact := model.Contact{
			ContactName: "Jane Smith",
			Tags:        []string{"family", "personal"},
			Insight:     &insight,
		}

		contactID, err := storage.AddContact(ctx, contact)
		require.NoError(t, err, "failed to add contact for list test")
		require.NotEmpty(t, contactID, "contact ID should not be empty")

		contactsList, err := storage.ListContacts(ctx)
		require.NoError(t, err, "failed to list contacts")
		assert.Len(t, contactsList, 2, "expected 2 contacts")

		for _, c := range contactsList {
			t.Logf("contact: %+v", c)
		}

		contactNames := make([]string, len(contactsList))
		for i, c := range contactsList {
			contactNames[i] = c.ContactName
		}
		assert.Contains(t, contactNames, "John Doe")
		assert.Contains(t, contactNames, "Jane Smith")
	})

	t.Run("GetContact", func(t *testing.T) {
		specificID := uuid.New().String()
		insight := "Expert in machine learning and AI"
		contact := model.Contact{
			ID:          specificID,
			ContactName: "Alice Johnson",
			Tags:        []string{"colleague", "tech"},
			Insight:     &insight,
		}

		returnedID, err := storage.AddContact(ctx, contact)
		require.NoError(t, err, "failed to add contact with specific ID")
		assert.Equal(t, specificID, returnedID, "returned ID should match specified ID")

		retrievedContact, err := storage.GetContact(ctx, specificID)
		require.NoError(t, err, "failed to get contact")
		assert.NotNil(t, retrievedContact)
		assert.Equal(t, specificID, retrievedContact.ID)
		assert.Equal(t, "Alice Johnson", retrievedContact.ContactName)
		require.NotNil(t, retrievedContact.Insight)
		assert.Equal(t, "Expert in machine learning and AI", *retrievedContact.Insight)
		assert.Equal(t, []string{"colleague", "tech"}, retrievedContact.Tags)

		t.Logf("Retrieved contact: %+v", retrievedContact)
	})

	t.Run("UpdateInsight", func(t *testing.T) {
		contactsList, err := storage.ListContacts(ctx)
		require.NoError(t, err, "failed to list contacts")
		require.NotEmpty(t, contactsList, "need at least one contact for update test")

		existingContact := contactsList[0]
		originalInsight := existingContact.Insight

		t.Logf("Contact before update: %+v", existingContact)

		newInsight := "Updated insight: Very creative and innovative thinker"
		err = storage.UpdateInsight(ctx, existingContact.ID, newInsight)
		require.NoError(t, err, "failed to update insight")

		updatedContact, err := storage.GetContact(ctx, existingContact.ID)
		require.NoError(t, err, "failed to get updated contact")
		require.NotNil(t, updatedContact.Insight)
		assert.Equal(t, newInsight, *updatedContact.Insight)
		assert.Equal(t, existingContact.ContactName, updatedContact.ContactName)
		assert.Equal(t, existingContact.Tags, updatedContact.Tags)
		assert.NotEqual(t, originalInsight, updatedContact.Insight)

		t.Logf("Updated contact: %+v", updatedContact)
	})

	t.Run("GetContact_NotFound", func(t *testing.T) {
		nonExistentID := uuid.New().String()
		contact, err := storage.GetContact(ctx, nonExistentID)
		assert.Error(t, err, "expected error for non-existent contact")
		assert.Nil(t, contact)

		t.Logf("Expected error for non-existent ID %s: %v", nonExistentID, err)
	})
}
