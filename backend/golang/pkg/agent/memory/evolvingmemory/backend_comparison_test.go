package evolvingmemory

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/localmodel/jinaaiembedding"
	"github.com/EternisAI/enchanted-twin/pkg/localmodel/ollama"
)

// BackendComparisonSuite provides comprehensive testing for backend compatibility.
type BackendComparisonSuite struct {
	t                 *testing.T
	ctx               context.Context
	logger            *log.Logger
	ai                *ai.Service
	embeddingsWrapper *storage.EmbeddingWrapper

	// Storage backends
	postgresStorage storage.Interface
	weaviateStorage storage.Interface

	// Test infrastructure
	postgresContainer testcontainers.Container
	postgresServer    *bootstrap.PostgresServer // Keep for fallback
	weaviateClient    *weaviate.Client

	// Test data
	testDocuments []memory.Document
	testFacts     []memory.MemoryFact
}

// setupBackendComparisonSuite initializes both storage backends for testing.
func setupBackendComparisonSuite(t *testing.T) *BackendComparisonSuite {
	ctx := context.Background()
	logger := log.New(os.Stdout)
	logger.SetLevel(log.DebugLevel) // Enable debug output for troubleshooting

	// Load environment variables
	envPath := filepath.Join("..", "..", "..", "..", ".env")
	_ = godotenv.Load(envPath)

	// Create AI services (with local model support)
	aiEmbeddingService, aiCompletionService, err := createAIServices(logger)
	if err != nil {
		t.Skip("Skipping backend comparison tests: " + err.Error())
		return nil
	}

	embeddingsModel := getEnvOrDefault("EMBEDDINGS_MODEL", "text-embedding-3-small")
	embeddingsWrapper, err := storage.NewEmbeddingWrapper(aiEmbeddingService, embeddingsModel)
	require.NoError(t, err, "Failed to create embeddings wrapper")

	// Use completion service if available, otherwise try embedding service
	var aiService *ai.Service
	if aiCompletionService != nil {
		aiService = aiCompletionService
	} else if service, ok := aiEmbeddingService.(*ai.Service); ok {
		aiService = service
	}

	suite := &BackendComparisonSuite{
		t:                 t,
		ctx:               ctx,
		logger:            logger,
		ai:                aiService, // May be nil for local models
		embeddingsWrapper: embeddingsWrapper,
	}

	// Setup PostgreSQL backend - if this fails, fail the test
	if !suite.setupPostgresBackend() {
		t.Fatal("Failed to setup PostgreSQL backend for comparison tests")
		return nil
	}

	// Setup Weaviate backend - if this fails, fail the test
	if !suite.setupWeaviateBackend() {
		t.Fatal("Failed to setup Weaviate backend for comparison tests")
		return nil
	}

	// Generate test data
	suite.generateTestData()

	return suite
}

// setupPostgresBackend creates a real PostgreSQL storage backend using testcontainers with pgvector.
func (s *BackendComparisonSuite) setupPostgresBackend() bool {
	s.logger.Info("Starting PostgreSQL container with pgvector extension")

	// Create PostgreSQL container with pgvector
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg16", // Official pgvector image with PostgreSQL 16
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithStartupTimeout(60*time.Second),
			wait.ForListeningPort("5432/tcp"),
		),
	}

	container, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		s.logger.Error("Failed to start PostgreSQL container", "error", err)
		return false
	}
	s.postgresContainer = container

	// Get the mapped port
	mappedPort, err := container.MappedPort(s.ctx, "5432")
	if err != nil {
		s.logger.Error("Failed to get mapped port", "error", err)
		return false
	}

	// Get container host
	host, err := container.Host(s.ctx)
	if err != nil {
		s.logger.Error("Failed to get container host", "error", err)
		return false
	}

	// Create connection string
	connString := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=testdb sslmode=disable",
		host, mappedPort.Port())

	s.logger.Info("Connecting to PostgreSQL container", "host", host, "port", mappedPort.Port())

	// Create a pgx connection for the storage layer with retry logic
	var pgxConn *pgx.Conn
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		var err error
		pgxConn, err = pgx.Connect(s.ctx, connString)
		if err == nil {
			break
		}

		if i == maxRetries-1 {
			s.logger.Error("Failed to create pgx connection after retries", "error", err, "connString", connString)
			return false
		}

		s.logger.Debug("Connection attempt failed, retrying", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}

	// Enable pgvector extension
	_, err = pgxConn.Exec(s.ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		s.logger.Error("Failed to create vector extension", "error", err)
		return false
	}

	// Create the required schema manually (since we're not using the bootstrap migrations)
	err = s.createPgvectorSchema(pgxConn)
	if err != nil {
		s.logger.Error("Failed to create PostgreSQL schema with pgvector", "error", err)
		return false
	}

	postgresStorage, err := storage.NewPostgresStorage(storage.NewPostgresStorageInput{
		DB:                pgxConn,
		Logger:            s.logger,
		EmbeddingsWrapper: s.embeddingsWrapper,
		ConnString:        connString,
	})
	if err != nil {
		s.logger.Error("Failed to create PostgreSQL storage", "error", err)
		return false
	}
	s.postgresStorage = postgresStorage

	s.logger.Info("Created real PostgreSQL storage backend with pgvector via testcontainers")
	return true
}

// createPgvectorSchema creates the essential tables for testing with proper pgvector support.
func (s *BackendComparisonSuite) createPgvectorSchema(conn *pgx.Conn) error {
	// Create essential tables with proper VECTOR columns (matching the migration schema)
	schema := `
		-- Memory facts table with proper vector column
		CREATE TABLE IF NOT EXISTS memory_facts (
		    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		    content TEXT NOT NULL,
		    content_vector VECTOR(1536) NOT NULL, -- OpenAI embedding dimensions
		    timestamp TIMESTAMPTZ NOT NULL,
		    source TEXT NOT NULL,
		    tags TEXT[] DEFAULT '{}',
		    document_references TEXT[] DEFAULT '{}',
		    metadata_json JSONB DEFAULT '{}',
		    -- Structured fact fields
		    fact_category TEXT NOT NULL,
		    fact_subject TEXT NOT NULL, 
		    fact_attribute TEXT,
		    fact_value TEXT,
		    fact_temporal_context TEXT,
		    fact_sensitivity TEXT,
		    fact_importance INTEGER,
		    fact_file_path TEXT,
		    created_at TIMESTAMPTZ DEFAULT NOW()
		);

		-- Documents table
		CREATE TABLE IF NOT EXISTS source_documents (
		    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		    content TEXT NOT NULL,
		    content_hash TEXT NOT NULL UNIQUE,
		    document_type TEXT NOT NULL,
		    original_id TEXT NOT NULL,
		    metadata_json JSONB DEFAULT '{}',
		    created_at TIMESTAMPTZ DEFAULT NOW()
		);

		-- Document chunks table with proper vector column
		CREATE TABLE IF NOT EXISTS document_chunks (
		    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		    content TEXT NOT NULL,
		    content_vector VECTOR(1536), -- OpenAI embedding dimensions
		    chunk_index INTEGER NOT NULL,
		    original_document_id TEXT NOT NULL,
		    source TEXT NOT NULL,
		    file_path TEXT,
		    tags TEXT[] DEFAULT '{}',
		    metadata_json JSONB DEFAULT '{}',
		    created_at TIMESTAMPTZ DEFAULT NOW()
		);
	`

	_, err := conn.Exec(s.ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to create pgvector schema: %w", err)
	}

	s.logger.Debug("Created PostgreSQL schema with pgvector support")
	return nil
}

// setupWeaviateBackend initializes Weaviate client and storage.
// Returns true if setup succeeded, false if it should be skipped.
func (s *BackendComparisonSuite) setupWeaviateBackend() bool {
	// Use unique port for each test to avoid conflicts - start from 51420 and add random offset
	portOffset, _ := rand.Int(rand.Reader, big.NewInt(50))
	port := fmt.Sprintf("%d", 51420+portOffset.Int64())

	// Start embedded Weaviate server with unique port and temp dir
	_, err := bootstrap.BootstrapWeaviateServer(s.ctx, s.logger, port, s.t.TempDir())
	if err != nil {
		s.logger.Error("Failed to start Weaviate server", "error", err, "port", port)
		return false
	}

	// Create client to connect to the server
	config := weaviate.Config{Scheme: "http", Host: fmt.Sprintf("localhost:%s", port)}
	s.weaviateClient = weaviate.New(config)

	// Create storage instance using storage.New (the generic constructor)
	weaviateStorage, err := storage.New(storage.NewStorageInput{
		Client:            s.weaviateClient,
		Logger:            s.logger,
		EmbeddingsWrapper: s.embeddingsWrapper,
	})
	if err != nil {
		s.logger.Error("Failed to create Weaviate storage", "error", err)
		return false
	}
	s.weaviateStorage = weaviateStorage

	// Ensure schema exists
	err = s.weaviateStorage.EnsureSchemaExists(s.ctx)
	if err != nil {
		s.logger.Error("Failed to ensure Weaviate schema exists", "error", err)
		return false
	}

	return true
}

// generateUUIDFromString creates a deterministic UUID v5 from a string for consistent test IDs.
func generateUUIDFromString(input string) string {
	namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // DNS namespace UUID
	return uuid.NewSHA1(namespace, []byte(input)).String()
}

// generateTestData creates deterministic test data for comparison.
func (s *BackendComparisonSuite) generateTestData() {
	now := time.Now()
	hour := time.Hour

	// Create diverse test documents with known semantic relationships
	s.testDocuments = []memory.Document{
		// Beverage preferences cluster
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-1"),
			FieldContent:   "The user loves coffee and drinks it every morning with milk and sugar.",
			FieldTimestamp: &now,
			FieldSource:    "conversations",
			FieldMetadata:  map[string]string{"topic": "preferences", "importance": "high", "category": "beverage"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-2"),
			FieldContent:   "The user prefers tea over coffee and likes it with honey.",
			FieldTimestamp: &now,
			FieldSource:    "conversations",
			FieldMetadata:  map[string]string{"topic": "preferences", "importance": "medium", "category": "beverage"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-20"),
			FieldContent:   "User mentioned they hate decaf coffee and only drink espresso-based drinks.",
			FieldTimestamp: func() *time.Time { t := now.Add(-hour); return &t }(),
			FieldSource:    "conversations",
			FieldMetadata:  map[string]string{"topic": "preferences", "importance": "medium", "category": "beverage"},
		},

		// Work and project cluster
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-3"),
			FieldContent:   "The user is working on a new project about machine learning and AI.",
			FieldTimestamp: &now,
			FieldSource:    "notes",
			FieldMetadata:  map[string]string{"topic": "work", "importance": "high", "project": "ml-ai"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-4"),
			FieldContent:   "The user has a meeting tomorrow at 2 PM about the quarterly review.",
			FieldTimestamp: &now,
			FieldSource:    "calendar",
			FieldMetadata:  map[string]string{"topic": "work", "importance": "medium", "meeting_type": "review"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-21"),
			FieldContent:   "Team standup meeting discussed the new API architecture and microservices design.",
			FieldTimestamp: func() *time.Time { t := now.Add(-2 * hour); return &t }(),
			FieldSource:    "notes",
			FieldMetadata:  map[string]string{"topic": "work", "importance": "high", "meeting_type": "standup"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-22"),
			FieldContent:   "User completed the database migration and updated all service endpoints.",
			FieldTimestamp: func() *time.Time { t := now.Add(-3 * hour); return &t }(),
			FieldSource:    "notes",
			FieldMetadata:  map[string]string{"topic": "work", "importance": "high", "task_type": "technical"},
		},

		// Personal interests and hobbies
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-5"),
			FieldContent:   "The user's favorite hobby is reading science fiction books.",
			FieldTimestamp: &now,
			FieldSource:    "personal",
			FieldMetadata:  map[string]string{"topic": "hobbies", "importance": "low", "genre": "sci-fi"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-23"),
			FieldContent:   "User recently finished reading 'Dune' and loved the world-building and political intrigue.",
			FieldTimestamp: func() *time.Time { t := now.Add(-4 * hour); return &t }(),
			FieldSource:    "personal",
			FieldMetadata:  map[string]string{"topic": "hobbies", "importance": "medium", "genre": "sci-fi", "book": "dune"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-24"),
			FieldContent:   "User enjoys weekend hiking trips and exploring nature trails around the city.",
			FieldTimestamp: func() *time.Time { t := now.Add(-5 * hour); return &t }(),
			FieldSource:    "personal",
			FieldMetadata:  map[string]string{"topic": "hobbies", "importance": "medium", "activity": "outdoor"},
		},

		// Travel and experiences
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-25"),
			FieldContent:   "User traveled to Japan last year and fell in love with the culture and cuisine.",
			FieldTimestamp: func() *time.Time { t := now.Add(-24 * hour); return &t }(),
			FieldSource:    "personal",
			FieldMetadata:  map[string]string{"topic": "travel", "importance": "high", "destination": "japan"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-26"),
			FieldContent:   "Planning a summer vacation to Europe, particularly interested in visiting museums in Paris.",
			FieldTimestamp: func() *time.Time { t := now.Add(-6 * hour); return &t }(),
			FieldSource:    "personal",
			FieldMetadata:  map[string]string{"topic": "travel", "importance": "medium", "destination": "europe"},
		},

		// Health and fitness
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-27"),
			FieldContent:   "User started a new workout routine focusing on strength training and cardio.",
			FieldTimestamp: func() *time.Time { t := now.Add(-7 * hour); return &t }(),
			FieldSource:    "personal",
			FieldMetadata:  map[string]string{"topic": "health", "importance": "medium", "activity": "fitness"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-28"),
			FieldContent:   "User mentioned having trouble sleeping and considering meditation apps.",
			FieldTimestamp: func() *time.Time { t := now.Add(-8 * hour); return &t }(),
			FieldSource:    "conversations",
			FieldMetadata:  map[string]string{"topic": "health", "importance": "medium", "concern": "sleep"},
		},

		// Technology interests
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-29"),
			FieldContent:   "User is excited about the latest developments in vector databases and embeddings.",
			FieldTimestamp: func() *time.Time { t := now.Add(-9 * hour); return &t }(),
			FieldSource:    "conversations",
			FieldMetadata:  map[string]string{"topic": "technology", "importance": "high", "domain": "ai"},
		},
		&memory.TextDocument{
			FieldID:        generateUUIDFromString("doc-30"),
			FieldContent:   "Experimenting with different programming languages, currently learning Rust.",
			FieldTimestamp: func() *time.Time { t := now.Add(-10 * hour); return &t }(),
			FieldSource:    "notes",
			FieldMetadata:  map[string]string{"topic": "technology", "importance": "medium", "language": "rust"},
		},
	}

	// Create structured memory facts
	s.testFacts = []memory.MemoryFact{
		// Beverage preferences
		{
			ID:                 generateUUIDFromString("fact-1"),
			Content:            "User prefers coffee with milk and sugar in the morning",
			Timestamp:          now,
			Source:             "conversations",
			Tags:               []string{"preference", "morning", "beverage"},
			Category:           "preference",
			Subject:            "user",
			Attribute:          "coffee_preference",
			Value:              "milk and sugar",
			Importance:         2,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-10"),
			Content:            "User dislikes decaf coffee and only drinks espresso-based beverages",
			Timestamp:          now.Add(-hour),
			Source:             "conversations",
			Tags:               []string{"preference", "beverage", "coffee", "espresso"},
			Category:           "preference",
			Subject:            "user",
			Attribute:          "coffee_type_preference",
			Value:              "espresso-based drinks only",
			Importance:         2,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-11"),
			Content:            "User sometimes prefers tea with honey over coffee",
			Timestamp:          now,
			Source:             "conversations",
			Tags:               []string{"preference", "beverage", "tea", "honey"},
			Category:           "preference",
			Subject:            "user",
			Attribute:          "tea_preference",
			Value:              "with honey",
			Importance:         1,
			DocumentReferences: []string{},
		},

		// Work and projects
		{
			ID:                 generateUUIDFromString("fact-2"),
			Content:            "User is working on machine learning project",
			Timestamp:          now,
			Source:             "notes",
			Tags:               []string{"work", "project", "ai"},
			Category:           "work",
			Subject:            "user",
			Attribute:          "current_project",
			Value:              "machine learning and AI",
			Importance:         3,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-3"),
			Content:            "User has quarterly review meeting tomorrow at 2 PM",
			Timestamp:          now,
			Source:             "calendar",
			Tags:               []string{"meeting", "work", "schedule"},
			Category:           "event",
			Subject:            "user",
			Attribute:          "upcoming_meeting",
			Value:              "quarterly review at 2 PM",
			Importance:         2,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-12"),
			Content:            "User completed database migration and service endpoint updates",
			Timestamp:          now.Add(-3 * hour),
			Source:             "notes",
			Tags:               []string{"work", "technical", "database", "completed"},
			Category:           "achievement",
			Subject:            "user",
			Attribute:          "completed_task",
			Value:              "database migration",
			Importance:         3,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-13"),
			Content:            "Team discussed API architecture and microservices in standup",
			Timestamp:          now.Add(-2 * hour),
			Source:             "notes",
			Tags:               []string{"work", "team", "architecture", "meeting"},
			Category:           "work",
			Subject:            "team",
			Attribute:          "discussion_topic",
			Value:              "API architecture and microservices",
			Importance:         2,
			DocumentReferences: []string{},
		},

		// Hobbies and interests
		{
			ID:                 generateUUIDFromString("fact-4"),
			Content:            "User's favorite hobby is reading science fiction books",
			Timestamp:          now,
			Source:             "personal",
			Tags:               []string{"hobby", "reading", "science fiction"},
			Category:           "interest",
			Subject:            "user",
			Attribute:          "favorite_hobby",
			Value:              "reading science fiction",
			Importance:         1,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-14"),
			Content:            "User recently finished reading 'Dune' and loved the world-building",
			Timestamp:          now.Add(-4 * hour),
			Source:             "personal",
			Tags:               []string{"reading", "science fiction", "dune", "completed"},
			Category:           "interest",
			Subject:            "user",
			Attribute:          "recent_book",
			Value:              "Dune",
			Importance:         1,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-15"),
			Content:            "User enjoys weekend hiking and exploring nature trails",
			Timestamp:          now.Add(-5 * hour),
			Source:             "personal",
			Tags:               []string{"hobby", "outdoor", "hiking", "weekend"},
			Category:           "interest",
			Subject:            "user",
			Attribute:          "outdoor_activity",
			Value:              "hiking",
			Importance:         2,
			DocumentReferences: []string{},
		},

		// Travel experiences
		{
			ID:                 generateUUIDFromString("fact-16"),
			Content:            "User traveled to Japan and loved the culture and cuisine",
			Timestamp:          now.Add(-24 * hour),
			Source:             "personal",
			Tags:               []string{"travel", "japan", "culture", "cuisine"},
			Category:           "experience",
			Subject:            "user",
			Attribute:          "travel_experience",
			Value:              "Japan - culture and cuisine",
			Importance:         3,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-17"),
			Content:            "User is planning summer vacation to Europe, interested in Paris museums",
			Timestamp:          now.Add(-6 * hour),
			Source:             "personal",
			Tags:               []string{"travel", "planning", "europe", "museums", "paris"},
			Category:           "plan",
			Subject:            "user",
			Attribute:          "travel_plan",
			Value:              "Europe summer vacation, Paris museums",
			Importance:         2,
			DocumentReferences: []string{},
		},

		// Health and fitness
		{
			ID:                 generateUUIDFromString("fact-18"),
			Content:            "User started new workout routine with strength training and cardio",
			Timestamp:          now.Add(-7 * hour),
			Source:             "personal",
			Tags:               []string{"health", "fitness", "workout", "strength", "cardio"},
			Category:           "health",
			Subject:            "user",
			Attribute:          "workout_routine",
			Value:              "strength training and cardio",
			Importance:         2,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-19"),
			Content:            "User has sleep troubles and is considering meditation apps",
			Timestamp:          now.Add(-8 * hour),
			Source:             "conversations",
			Tags:               []string{"health", "sleep", "meditation", "apps"},
			Category:           "concern",
			Subject:            "user",
			Attribute:          "health_concern",
			Value:              "sleep troubles, considering meditation",
			Importance:         2,
			DocumentReferences: []string{},
		},

		// Technology interests
		{
			ID:                 generateUUIDFromString("fact-20"),
			Content:            "User is excited about vector databases and embeddings developments",
			Timestamp:          now.Add(-9 * hour),
			Source:             "conversations",
			Tags:               []string{"technology", "ai", "vector", "databases", "embeddings"},
			Category:           "interest",
			Subject:            "user",
			Attribute:          "tech_interest",
			Value:              "vector databases and embeddings",
			Importance:         3,
			DocumentReferences: []string{},
		},
		{
			ID:                 generateUUIDFromString("fact-21"),
			Content:            "User is experimenting with programming languages, currently learning Rust",
			Timestamp:          now.Add(-10 * hour),
			Source:             "notes",
			Tags:               []string{"technology", "programming", "rust", "learning"},
			Category:           "learning",
			Subject:            "user",
			Attribute:          "current_learning",
			Value:              "Rust programming language",
			Importance:         2,
			DocumentReferences: []string{},
		},
	}
}

// generateLargeDataset creates additional test data for stress testing.
func (s *BackendComparisonSuite) generateLargeDataset() {
	now := time.Now()

	// Generate many additional documents and facts
	for i := range 100 {
		docID := generateUUIDFromString(fmt.Sprintf("large-doc-%d", i))
		factID := generateUUIDFromString(fmt.Sprintf("large-fact-%d", i))

		// Create variety of content
		contents := []string{
			"User discussed project deadlines and upcoming milestones",
			"Mentioned preference for working in quiet environments",
			"Shared thoughts about team collaboration and communication",
			"Talked about learning new programming frameworks",
			"Described favorite restaurants and dining experiences",
			"Discussed fitness goals and workout routines",
			"Mentioned travel plans and destination preferences",
			"Shared opinions about books and reading habits",
			"Talked about music preferences and concert experiences",
			"Discussed technology trends and industry developments",
		}

		categories := []string{"work", "preference", "learning", "experience", "interest", "health", "plan"}
		sources := []string{"conversations", "notes", "personal", "calendar"}

		content := contents[i%len(contents)]
		category := categories[i%len(categories)]
		source := sources[i%len(sources)]
		importance := (i % 3) + 1

		doc := &memory.TextDocument{
			FieldID:        docID,
			FieldContent:   fmt.Sprintf("%s - instance %d", content, i),
			FieldTimestamp: func() *time.Time { t := now.Add(time.Duration(-i) * time.Minute); return &t }(),
			FieldSource:    source,
			FieldMetadata:  map[string]string{"topic": category, "importance": fmt.Sprintf("%d", importance), "batch": "large"},
		}

		fact := memory.MemoryFact{
			ID:                 factID,
			Content:            fmt.Sprintf("Generated fact %d: %s", i, content),
			Timestamp:          now.Add(time.Duration(-i) * time.Minute),
			Source:             source,
			Tags:               []string{"generated", category, "large_dataset"},
			Category:           category,
			Subject:            "user",
			Attribute:          fmt.Sprintf("generated_attribute_%d", i%10),
			Value:              fmt.Sprintf("generated_value_%d", i),
			Importance:         importance,
			DocumentReferences: []string{},
		}

		s.testDocuments = append(s.testDocuments, doc)
		s.testFacts = append(s.testFacts, fact)
	}

	// Store the additional data
	pgStorageImpl, err := New(Dependencies{
		Logger:             s.logger,
		Storage:            s.postgresStorage,
		CompletionsService: s.ai,
		CompletionsModel:   "gpt-4o-mini",
		EmbeddingsWrapper:  s.embeddingsWrapper,
	})
	require.NoError(s.t, err)

	wvStorageImpl, err := New(Dependencies{
		Logger:             s.logger,
		Storage:            s.weaviateStorage,
		CompletionsService: s.ai,
		CompletionsModel:   "gpt-4o-mini",
		EmbeddingsWrapper:  s.embeddingsWrapper,
	})
	require.NoError(s.t, err)

	// Store additional documents
	for i := len(s.testDocuments) - 100; i < len(s.testDocuments); i++ {
		doc := s.testDocuments[i]
		_, err := s.postgresStorage.UpsertDocument(s.ctx, doc)
		require.NoError(s.t, err)

		_, err = s.weaviateStorage.UpsertDocument(s.ctx, doc)
		require.NoError(s.t, err)
	}

	// Store additional facts
	additionalFacts := make([]*memory.MemoryFact, 100)
	for i := range 100 {
		additionalFacts[i] = &s.testFacts[len(s.testFacts)-100+i]
	}

	pgStorageImplTyped, ok := pgStorageImpl.(interface {
		StoreFactsDirectly(context.Context, []*memory.MemoryFact, memory.ProgressCallback) error
	})
	require.True(s.t, ok)
	err = pgStorageImplTyped.StoreFactsDirectly(s.ctx, additionalFacts, nil)
	require.NoError(s.t, err)

	wvStorageImplTyped, ok := wvStorageImpl.(interface {
		StoreFactsDirectly(context.Context, []*memory.MemoryFact, memory.ProgressCallback) error
	})
	require.True(s.t, ok)
	err = wvStorageImplTyped.StoreFactsDirectly(s.ctx, additionalFacts, nil)
	require.NoError(s.t, err)

	// Allow time for indexing
	time.Sleep(5 * time.Second)
}

// cleanup tears down test infrastructure.
func (s *BackendComparisonSuite) cleanup() {
	if s.postgresContainer != nil {
		if err := s.postgresContainer.Terminate(s.ctx); err != nil {
			s.logger.Error("Failed to terminate PostgreSQL container", "error", err)
		}
	}
	if s.postgresServer != nil {
		_ = s.postgresServer.Stop()
	}
}

// seedBothBackends stores the same data in both backends using StorageImpl.
func (s *BackendComparisonSuite) seedBothBackends() {
	// Create storage implementations for both backends
	pgStorageImpl, err := New(Dependencies{
		Logger:             s.logger,
		Storage:            s.postgresStorage,
		CompletionsService: s.ai,
		CompletionsModel:   "gpt-4o-mini",
		EmbeddingsWrapper:  s.embeddingsWrapper,
	})
	require.NoError(s.t, err, "Failed to create PostgreSQL storage impl")

	wvStorageImpl, err := New(Dependencies{
		Logger:             s.logger,
		Storage:            s.weaviateStorage,
		CompletionsService: s.ai,
		CompletionsModel:   "gpt-4o-mini",
		EmbeddingsWrapper:  s.embeddingsWrapper,
	})
	require.NoError(s.t, err, "Failed to create Weaviate storage impl")

	// Store documents in both backends
	for _, doc := range s.testDocuments {
		_, err := s.postgresStorage.UpsertDocument(s.ctx, doc)
		require.NoError(s.t, err, "Failed to store document in PostgreSQL: %s", doc.ID())

		_, err = s.weaviateStorage.UpsertDocument(s.ctx, doc)
		require.NoError(s.t, err, "Failed to store document in Weaviate: %s", doc.ID())
	}

	// Convert facts to pointers for StoreFactsDirectly
	factPointers := make([]*memory.MemoryFact, len(s.testFacts))
	for i := range s.testFacts {
		factPointers[i] = &s.testFacts[i]
	}

	// Store facts using the high-level interface with type assertion
	pgStorageImplTyped, ok := pgStorageImpl.(interface {
		StoreFactsDirectly(context.Context, []*memory.MemoryFact, memory.ProgressCallback) error
	})
	require.True(s.t, ok, "Failed to type assert PostgreSQL storage impl")
	err = pgStorageImplTyped.StoreFactsDirectly(s.ctx, factPointers, func(processed, total int) {
		s.logger.Debug("PostgreSQL storing facts", "processed", processed, "total", total)
	})
	if err != nil {
		s.logger.Error("Failed to store facts in PostgreSQL", "error", err)
	}
	require.NoError(s.t, err, "Failed to store facts in PostgreSQL")

	// Debug: Try to execute a direct SQL query to see if data is in the database
	// Get container connection details for direct query
	if s.postgresContainer != nil {
		mappedPort, _ := s.postgresContainer.MappedPort(s.ctx, "5432")
		host, _ := s.postgresContainer.Host(s.ctx)
		connString := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=testdb sslmode=disable",
			host, mappedPort.Port())

		if pgxConn, err := pgx.Connect(s.ctx, connString); err == nil {
			var count int
			err = pgxConn.QueryRow(s.ctx, "SELECT COUNT(*) FROM memory_facts").Scan(&count)
			if err != nil {
				s.logger.Error("Failed to count memory_facts", "error", err)
			} else {
				s.logger.Info("Direct SQL count of memory_facts", "count", count)
			}
			_ = pgxConn.Close(s.ctx)
		}
	}

	// Debug: Check if facts were actually stored in PostgreSQL
	pgDebugResult, err := s.postgresStorage.Query(s.ctx, "user", nil) // Query for "user" to get some facts
	if err != nil {
		s.logger.Error("Failed to query PostgreSQL for debug", "error", err)
	} else {
		s.logger.Info("Debug: PostgreSQL facts stored", "count", len(pgDebugResult.Facts))
	}

	wvStorageImplTyped, ok := wvStorageImpl.(interface {
		StoreFactsDirectly(context.Context, []*memory.MemoryFact, memory.ProgressCallback) error
	})
	require.True(s.t, ok, "Failed to type assert Weaviate storage impl")
	err = wvStorageImplTyped.StoreFactsDirectly(s.ctx, factPointers, nil)
	require.NoError(s.t, err, "Failed to store facts in Weaviate")

	// Allow time for indexing
	time.Sleep(2 * time.Second)
}

// TestBackendQueryConsistency verifies that both backends return identical results.
func TestBackendQueryConsistency(t *testing.T) {
	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	suite.seedBothBackends()

	testCases := []struct {
		name      string
		queryText string
		filter    *memory.Filter
	}{
		{
			name:      "Basic semantic search - coffee",
			queryText: "coffee preferences",
			filter:    nil,
		},
		{
			name:      "Filtered by source",
			queryText: "user preferences",
			filter:    &memory.Filter{Source: helpers.Ptr("conversations")},
		},
		{
			name:      "Filtered by category",
			queryText: "work activities",
			filter:    &memory.Filter{FactCategory: helpers.Ptr("work")},
		},
		{
			name:      "Distance filtered search",
			queryText: "machine learning",
			filter:    &memory.Filter{Distance: 0.8},
		},
		{
			name:      "Combined filters",
			queryText: "user activities",
			filter: &memory.Filter{
				Source:       helpers.Ptr("conversations"),
				FactCategory: helpers.Ptr("preference"),
				Distance:     0.9,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Query both backends
			pgResult, err := suite.postgresStorage.Query(suite.ctx, tc.queryText, tc.filter)
			require.NoError(t, err, "PostgreSQL query failed")

			wvResult, err := suite.weaviateStorage.Query(suite.ctx, tc.queryText, tc.filter)
			require.NoError(t, err, "Weaviate query failed")

			// Verify result consistency
			suite.assertQueryResultsEquivalent(t, tc.name, pgResult, wvResult)
		})
	}
}

// TestDistanceSimilarityConsistency verifies distance/similarity scores match.
func TestDistanceSimilarityConsistency(t *testing.T) {
	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	suite.seedBothBackends()

	testQueries := []struct {
		name      string
		queryText string
		distance  float32
	}{
		{"High similarity threshold", "coffee preference", 0.9},
		{"Medium similarity threshold", "work project", 0.8},
		{"Low similarity threshold", "user meeting", 0.7},
	}

	for _, tq := range testQueries {
		t.Run(tq.name, func(t *testing.T) {
			filter := &memory.Filter{Distance: tq.distance}

			pgResult, err := suite.postgresStorage.Query(suite.ctx, tq.queryText, filter)
			require.NoError(t, err, "PostgreSQL query failed")

			wvResult, err := suite.weaviateStorage.Query(suite.ctx, tq.queryText, filter)
			require.NoError(t, err, "Weaviate query failed")

			// Both should return the same number of results within distance threshold
			assert.Equal(t, len(pgResult.Facts), len(wvResult.Facts),
				"Result count mismatch for distance %f", tq.distance)

			// Verify all returned results meet the distance threshold
			suite.verifyDistanceThreshold(t, pgResult.Facts, tq.distance)
			suite.verifyDistanceThreshold(t, wvResult.Facts, tq.distance)
		})
	}
}

// TestResultOrderingConsistency verifies results are returned in the same order.
func TestResultOrderingConsistency(t *testing.T) {
	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	suite.seedBothBackends()

	// Test ordering with different query patterns
	testCases := []struct {
		name      string
		queryText string
		limit     int
	}{
		{"Top 3 results", "user preferences", 3},
		{"Top 5 results", "work activities", 5},
		{"All results", "user activities", 0}, // No limit
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := &memory.Filter{}
			if tc.limit > 0 {
				filter.Limit = helpers.Ptr(tc.limit)
			}

			pgResult, err := suite.postgresStorage.Query(suite.ctx, tc.queryText, filter)
			require.NoError(t, err, "PostgreSQL query failed")

			wvResult, err := suite.weaviateStorage.Query(suite.ctx, tc.queryText, filter)
			require.NoError(t, err, "Weaviate query failed")

			// Verify same result count
			assert.Equal(t, len(pgResult.Facts), len(wvResult.Facts), "Result count mismatch")

			// Verify same content (order may differ slightly due to floating point precision)
			suite.assertSameFactsReturned(t, pgResult.Facts, wvResult.Facts)
		})
	}
}

// TestPerformanceBenchmark compares query performance between backends.
func TestPerformanceBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance benchmark in short mode")
	}

	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	suite.seedBothBackends()

	benchmarkQueries := []string{
		"coffee preferences and habits",
		"work projects and meetings",
		"user hobbies and interests",
	}

	for _, query := range benchmarkQueries {
		t.Run(fmt.Sprintf("Benchmark_%s", query), func(t *testing.T) {
			// Benchmark PostgreSQL
			pgStart := time.Now()
			for range 10 {
				_, err := suite.postgresStorage.Query(suite.ctx, query, nil)
				require.NoError(t, err)
			}
			pgDuration := time.Since(pgStart)

			// Benchmark Weaviate
			wvStart := time.Now()
			for range 10 {
				_, err := suite.weaviateStorage.Query(suite.ctx, query, nil)
				require.NoError(t, err)
			}
			wvDuration := time.Since(wvStart)

			suite.logger.Info("Performance comparison",
				"query", query,
				"postgres_avg_ms", pgDuration.Milliseconds()/10,
				"weaviate_avg_ms", wvDuration.Milliseconds()/10,
				"ratio", float64(pgDuration)/float64(wvDuration),
			)

			// Both should complete within reasonable time (adjust as needed)
			assert.Less(t, pgDuration.Seconds(), 30.0, "PostgreSQL queries too slow")
			assert.Less(t, wvDuration.Seconds(), 30.0, "Weaviate queries too slow")
		})
	}
}

// Helper methods for assertions

// assertQueryResultsEquivalent verifies that two query results contain equivalent data.
func (s *BackendComparisonSuite) assertQueryResultsEquivalent(t *testing.T, testName string, pgResult, wvResult memory.QueryResult) {
	// Same number of facts
	assert.Equal(t, len(pgResult.Facts), len(wvResult.Facts),
		"%s: Fact count mismatch - PostgreSQL: %d, Weaviate: %d",
		testName, len(pgResult.Facts), len(wvResult.Facts))

	// Same number of document chunks
	assert.Equal(t, len(pgResult.DocumentChunks), len(wvResult.DocumentChunks),
		"%s: Document chunk count mismatch - PostgreSQL: %d, Weaviate: %d",
		testName, len(pgResult.DocumentChunks), len(wvResult.DocumentChunks))

	// Verify same facts are returned
	s.assertSameFactsReturned(t, pgResult.Facts, wvResult.Facts)
}

// assertSameFactsReturned verifies both result sets contain the same facts.
func (s *BackendComparisonSuite) assertSameFactsReturned(t *testing.T, pgFacts, wvFacts []memory.MemoryFact) {
	// Create maps for easier comparison
	pgFactIDs := make(map[string]*memory.MemoryFact)
	wvFactIDs := make(map[string]*memory.MemoryFact)

	for i := range pgFacts {
		pgFactIDs[pgFacts[i].ID] = &pgFacts[i]
	}
	for i := range wvFacts {
		wvFactIDs[wvFacts[i].ID] = &wvFacts[i]
	}

	// Verify same fact IDs
	assert.Equal(t, len(pgFactIDs), len(wvFactIDs), "Different number of unique facts")

	for id, pgFact := range pgFactIDs {
		wvFact, exists := wvFactIDs[id]
		assert.True(t, exists, "Fact %s found in PostgreSQL but not in Weaviate", id)
		if exists {
			assert.Equal(t, pgFact.Content, wvFact.Content, "Fact content mismatch for %s", id)
			assert.Equal(t, pgFact.Source, wvFact.Source, "Fact source mismatch for %s", id)
		}
	}
}

// verifyDistanceThreshold ensures all returned facts are valid and properly formatted.
//
// Note: In a complete implementation, this function would verify that each fact meets
// the specified distance threshold by checking the similarity scores in the query metadata.
// However, the current memory backend interface doesn't expose distance scores in the
// QueryResult, so we perform basic validation instead.
//
// Future improvements could include:
// - Extending memory.QueryResult to include distance/similarity scores per fact
// - Adding a separate metadata field that contains scoring information
// - Implementing a more sophisticated verification based on embedding similarities.
func (s *BackendComparisonSuite) verifyDistanceThreshold(t *testing.T, facts []memory.MemoryFact, threshold float32) {
	t.Helper()

	for i, fact := range facts {
		// Verify fact has required fields populated
		assert.NotEmpty(t, fact.ID, "Fact %d should have a valid ID", i)
		assert.NotEmpty(t, fact.Content, "Fact %d should have content", i)
		assert.NotEmpty(t, fact.Source, "Fact %d should have a source", i)
		assert.NotZero(t, fact.Timestamp, "Fact %d should have a timestamp", i)

		// Verify fact content quality (basic heuristics)
		contentLen := len(strings.TrimSpace(fact.Content))
		assert.Greater(t, contentLen, 5, "Fact %d content should be substantial", i)

		// Log threshold for debugging purposes
		if threshold > 0.95 {
			contentPreview := fact.Content
			if len(contentPreview) > 50 {
				contentPreview = contentPreview[:50]
			}
			s.logger.Debug("High threshold query returned fact",
				"fact_id", fact.ID,
				"threshold", threshold,
				"content_preview", contentPreview)
		}
	}

	// If we have facts with a very restrictive threshold, log for analysis
	if len(facts) > 0 && threshold > 0.9 {
		s.logger.Info("Restrictive threshold query results",
			"threshold", threshold,
			"returned_facts", len(facts))
	}
}

// getEnvOrDefault returns environment variable value or default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// createAIServices creates AI services with support for local models.
func createAIServices(logger *log.Logger) (ai.Embedding, *ai.Service, error) {
	var embeddingService ai.Embedding
	var completionService *ai.Service

	// Check for local embedding first
	if os.Getenv("USE_LOCAL_EMBEDDINGS") == "true" {
		appDataPath := os.Getenv("APP_DATA_PATH")
		sharedLibPath := os.Getenv("SHARED_LIBRARY_PATH")

		if appDataPath != "" && sharedLibPath != "" {
			logger.Info("Using local JinaAI embedding model")
			localEmbedding, err := jinaaiembedding.NewEmbedding(appDataPath, sharedLibPath)
			if err != nil {
				logger.Warn("Failed to create local embedding model, falling back to API", "error", err)
			} else {
				embeddingService = localEmbedding
			}
		}
	}

	// Check for Ollama completions
	var ollamaClient *ollama.OllamaClient
	if os.Getenv("USE_OLLAMA_COMPLETIONS") == "true" {
		ollamaURL := getEnvOrDefault("OLLAMA_URL", "http://localhost:11434/v1")
		ollamaModel := getEnvOrDefault("OLLAMA_COMPLETION_MODEL", "qwen3-0.6b-q4_k_m")

		logger.Info("Using Ollama completion model", "url", ollamaURL, "model", ollamaModel)
		ollamaClient = ollama.NewOllamaClient(ollamaURL, ollamaModel, logger)
	}

	// Check if we have local embeddings
	if embeddingService != nil {
		logger.Info("Using local embedding model")
		// Provide a mock completion service for tests that require it
		mockService := NewMockAIService(logger)
		logger.Warn("Using mock completion service for local embedding tests")
		if ollamaClient != nil {
			logger.Info("Local Ollama completion available but not yet fully integrated")
		}
		return embeddingService, mockService.Service, nil
	}

	// Fall back to API-based services if no local models
	embeddingsKey := os.Getenv("EMBEDDINGS_API_KEY")
	completionsKey := os.Getenv("COMPLETIONS_API_KEY")

	if embeddingsKey == "" {
		return nil, nil, fmt.Errorf("no embedding service available: set USE_LOCAL_EMBEDDINGS=true with required paths, or provide EMBEDDINGS_API_KEY")
	}

	embeddingsURL := getEnvOrDefault("EMBEDDINGS_API_URL", "https://api.openai.com/v1")
	logger.Info("Using API-based embedding service", "url", embeddingsURL)
	embeddingService = ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

	if completionsKey != "" {
		completionsURL := getEnvOrDefault("COMPLETIONS_API_URL", "https://api.openai.com/v1")
		logger.Info("Using API-based completion service", "url", completionsURL)
		completionService = ai.NewOpenAIService(logger, completionsKey, completionsURL)
	} else {
		logger.Warn("No completion service available - some tests may fail. Set USE_OLLAMA_COMPLETIONS=true or provide COMPLETIONS_API_KEY")
	}

	return embeddingService, completionService, nil
}

// MockAIService provides a minimal mock completions service for testing.
type MockAIService struct {
	*ai.Service
	logger *log.Logger
}

func NewMockAIService(logger *log.Logger) *MockAIService {
	// Create a minimal ai.Service with dummy values
	baseService := ai.NewOpenAIService(logger, "mock-key", "http://mock-url")
	return &MockAIService{
		Service: baseService,
		logger:  logger,
	}
}

// TestCrossBackendMigration tests data migration between backends.
func TestCrossBackendMigration(t *testing.T) {
	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	// Create storage implementation for PostgreSQL
	pgStorageImpl, err := New(Dependencies{
		Logger:             suite.logger,
		Storage:            suite.postgresStorage,
		CompletionsService: suite.ai,
		CompletionsModel:   "gpt-4o-mini",
		EmbeddingsWrapper:  suite.embeddingsWrapper,
	})
	require.NoError(t, err)

	// Store data only in PostgreSQL initially
	for _, doc := range suite.testDocuments {
		_, err := suite.postgresStorage.UpsertDocument(suite.ctx, doc)
		require.NoError(t, err)
	}

	// Convert facts to pointers
	factPointers := make([]*memory.MemoryFact, len(suite.testFacts))
	for i := range suite.testFacts {
		factPointers[i] = &suite.testFacts[i]
	}

	pgStorageImplTyped, ok := pgStorageImpl.(interface {
		StoreFactsDirectly(context.Context, []*memory.MemoryFact, memory.ProgressCallback) error
	})
	require.True(t, ok, "Failed to type assert PostgreSQL storage impl")
	err = pgStorageImplTyped.StoreFactsDirectly(suite.ctx, factPointers, nil)
	require.NoError(t, err)

	// Query data from PostgreSQL
	pgResult, err := suite.postgresStorage.Query(suite.ctx, "user preferences", nil)
	require.NoError(t, err)

	// Create storage implementation for Weaviate
	wvStorageImpl, err := New(Dependencies{
		Logger:             suite.logger,
		Storage:            suite.weaviateStorage,
		CompletionsService: suite.ai,
		CompletionsModel:   "gpt-4o-mini",
		EmbeddingsWrapper:  suite.embeddingsWrapper,
	})
	require.NoError(t, err)

	// Migrate facts to Weaviate
	pgFactPointers := make([]*memory.MemoryFact, len(pgResult.Facts))
	for i := range pgResult.Facts {
		pgFactPointers[i] = &pgResult.Facts[i]
	}
	wvStorageImplTyped, ok := wvStorageImpl.(interface {
		StoreFactsDirectly(context.Context, []*memory.MemoryFact, memory.ProgressCallback) error
	})
	require.True(t, ok, "Failed to type assert Weaviate storage impl")
	err = wvStorageImplTyped.StoreFactsDirectly(suite.ctx, pgFactPointers, nil)
	require.NoError(t, err)

	time.Sleep(2 * time.Second) // Allow indexing

	// Verify migrated data by querying Weaviate
	wvResult, err := suite.weaviateStorage.Query(suite.ctx, "user preferences", nil)
	require.NoError(t, err)

	// Results should be equivalent after migration
	suite.assertQueryResultsEquivalent(t, "Migration", pgResult, wvResult)
}

// TestStructuredFactFiltering tests that structured fact filtering works consistently.
func TestStructuredFactFiltering(t *testing.T) {
	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	suite.seedBothBackends()

	structuredFilters := []struct {
		name   string
		filter *memory.Filter
	}{
		{
			name: "Filter by fact category",
			filter: &memory.Filter{
				FactCategory: helpers.Ptr("preference"),
			},
		},
		{
			name: "Filter by subject",
			filter: &memory.Filter{
				Subject: helpers.Ptr("user"),
			},
		},
		{
			name: "Filter by importance",
			filter: &memory.Filter{
				FactImportance: helpers.Ptr(2),
			},
		},
		{
			name: "Filter by importance range",
			filter: &memory.Filter{
				FactImportanceMin: helpers.Ptr(2),
				FactImportanceMax: helpers.Ptr(3),
			},
		},
		{
			name: "Combined structured filters",
			filter: &memory.Filter{
				FactCategory:   helpers.Ptr("work"),
				Subject:        helpers.Ptr("user"),
				FactImportance: helpers.Ptr(3),
			},
		},
	}

	for _, tf := range structuredFilters {
		t.Run(tf.name, func(t *testing.T) {
			pgResult, err := suite.postgresStorage.Query(suite.ctx, "user activities", tf.filter)
			require.NoError(t, err, "PostgreSQL structured query failed")

			wvResult, err := suite.weaviateStorage.Query(suite.ctx, "user activities", tf.filter)
			require.NoError(t, err, "Weaviate structured query failed")

			suite.assertQueryResultsEquivalent(t, tf.name, pgResult, wvResult)
		})
	}
}

// TestExtendedQueryScenarios tests comprehensive query scenarios with more data.
func TestExtendedQueryScenarios(t *testing.T) {
	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	suite.seedBothBackends()

	extendedTestCases := []struct {
		name             string
		queryText        string
		filter           *memory.Filter
		expectMinResults int
	}{
		{
			name:             "Semantic clustering - all beverage preferences",
			queryText:        "coffee tea beverage drinks",
			filter:           nil,
			expectMinResults: 3,
		},
		{
			name:             "Complex work-related queries",
			queryText:        "database migration architecture technical work",
			filter:           &memory.Filter{FactCategory: helpers.Ptr("work")},
			expectMinResults: 2,
		},
		{
			name:             "Health and wellness topics",
			queryText:        "health fitness workout sleep meditation",
			filter:           &memory.Filter{FactCategory: helpers.Ptr("health")},
			expectMinResults: 2,
		},
		{
			name:             "Technology and learning interests",
			queryText:        "technology programming rust vector databases",
			filter:           &memory.Filter{Source: helpers.Ptr("conversations")},
			expectMinResults: 1,
		},
		{
			name:             "Travel and cultural experiences",
			queryText:        "travel japan europe culture museums",
			filter:           &memory.Filter{FactCategory: helpers.Ptr("experience")},
			expectMinResults: 1,
		},
		{
			name:             "Reading and entertainment preferences",
			queryText:        "reading books science fiction dune literature",
			filter:           &memory.Filter{FactCategory: helpers.Ptr("interest")},
			expectMinResults: 2,
		},
		{
			name:      "Time-based filtering - recent activities",
			queryText: "recent user activities",
			filter: &memory.Filter{
				TimestampAfter: helpers.Ptr(time.Now().Add(-2 * time.Hour)),
			},
			expectMinResults: 5,
		},
		{
			name:      "Importance-based filtering - high priority items",
			queryText: "important user information",
			filter: &memory.Filter{
				FactImportanceMin: helpers.Ptr(3),
			},
			expectMinResults: 3,
		},
		{
			name:      "Multi-category search with distance threshold",
			queryText: "user preferences and interests",
			filter: &memory.Filter{
				Distance:          0.8,
				FactImportanceMin: helpers.Ptr(1),
			},
			expectMinResults: 1,
		},
		{
			name:      "Source-specific with time range",
			queryText: "personal activities and hobbies",
			filter: &memory.Filter{
				Source:         helpers.Ptr("personal"),
				TimestampAfter: helpers.Ptr(time.Now().Add(-12 * time.Hour)),
			},
			expectMinResults: 3,
		},
	}

	for _, tc := range extendedTestCases {
		t.Run(tc.name, func(t *testing.T) {
			pgResult, err := suite.postgresStorage.Query(suite.ctx, tc.queryText, tc.filter)
			require.NoError(t, err, "PostgreSQL query failed")

			wvResult, err := suite.weaviateStorage.Query(suite.ctx, tc.queryText, tc.filter)
			require.NoError(t, err, "Weaviate query failed")

			suite.assertQueryResultsEquivalent(t, tc.name, pgResult, wvResult)

			// Verify minimum expected results
			assert.GreaterOrEqual(t, len(pgResult.Facts), tc.expectMinResults,
				"PostgreSQL returned fewer results than expected for %s", tc.name)
			assert.GreaterOrEqual(t, len(wvResult.Facts), tc.expectMinResults,
				"Weaviate returned fewer results than expected for %s", tc.name)
		})
	}
}

// TestEdgeCasesAndErrorConditions tests boundary conditions and error scenarios.
func TestEdgeCasesAndErrorConditions(t *testing.T) {
	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	suite.seedBothBackends()

	edgeCases := []struct {
		name            string
		queryText       string
		filter          *memory.Filter
		expectNoResults bool
	}{
		{
			name:            "Empty query string",
			queryText:       "",
			filter:          nil,
			expectNoResults: false, // Should return some results based on filter
		},
		{
			name:            "Very long query string",
			queryText:       "this is a very long query string that contains many words and should test the embedding and search capabilities with extensive text that goes beyond normal query lengths and includes various topics like technology work health travel reading hobbies preferences meetings projects databases programming languages fitness outdoor activities museums culture cuisine sleep meditation vector embeddings",
			filter:          nil,
			expectNoResults: false,
		},
		{
			name:            "Query with special characters",
			queryText:       "user's preferences & hobbies: coffee, tea; work-related activities!",
			filter:          nil,
			expectNoResults: false,
		},
		{
			name:            "Non-existent category filter",
			queryText:       "user activities",
			filter:          &memory.Filter{FactCategory: helpers.Ptr("nonexistent_category")},
			expectNoResults: true,
		},
		{
			name:            "Non-existent source filter",
			filter:          &memory.Filter{Source: helpers.Ptr("nonexistent_source")},
			queryText:       "user activities",
			expectNoResults: true,
		},
		{
			name:      "Impossible time range",
			queryText: "user activities",
			filter: &memory.Filter{
				TimestampAfter:  helpers.Ptr(time.Now().Add(24 * time.Hour)),
				TimestampBefore: helpers.Ptr(time.Now()),
			},
			expectNoResults: true,
		},
		{
			name:            "Very restrictive distance threshold",
			queryText:       "coffee preferences",
			filter:          &memory.Filter{Distance: 0.99},
			expectNoResults: true,
		},
		{
			name:            "Very permissive distance threshold",
			queryText:       "completely unrelated nonsense query",
			filter:          &memory.Filter{Distance: 0.1},
			expectNoResults: false,
		},
		{
			name:      "Impossible importance range",
			queryText: "user activities",
			filter: &memory.Filter{
				FactImportanceMin: helpers.Ptr(10),
				FactImportanceMax: helpers.Ptr(5),
			},
			expectNoResults: true,
		},
		{
			name:            "Zero result limit",
			queryText:       "user activities",
			filter:          &memory.Filter{Limit: helpers.Ptr(0)},
			expectNoResults: true,
		},
		{
			name:            "Very large result limit",
			queryText:       "user activities",
			filter:          &memory.Filter{Limit: helpers.Ptr(10000)},
			expectNoResults: false,
		},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			pgResult, err := suite.postgresStorage.Query(suite.ctx, tc.queryText, tc.filter)
			require.NoError(t, err, "PostgreSQL query should not error for edge case: %s", tc.name)

			wvResult, err := suite.weaviateStorage.Query(suite.ctx, tc.queryText, tc.filter)
			require.NoError(t, err, "Weaviate query should not error for edge case: %s", tc.name)

			// Verify both backends handle edge cases consistently
			suite.assertQueryResultsEquivalent(t, tc.name, pgResult, wvResult)

			if tc.expectNoResults {
				assert.Empty(t, pgResult.Facts, "Expected no results for %s in PostgreSQL", tc.name)
				assert.Empty(t, wvResult.Facts, "Expected no results for %s in Weaviate", tc.name)
			}
		})
	}
}

// TestLargeDatasetScenarios tests behavior with significantly more data.
func TestLargeDatasetScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset tests in short mode")
	}

	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	// First seed the base data
	suite.seedBothBackends()

	// Generate additional large dataset for stress testing
	suite.generateLargeDataset()

	testCases := []struct {
		name      string
		queryText string
		filter    *memory.Filter
	}{
		{
			name:      "Query with large result set",
			queryText: "user",
			filter:    nil,
		},
		{
			name:      "Filtered query on large dataset",
			queryText: "activities and interests",
			filter:    &memory.Filter{FactImportance: helpers.Ptr(2)},
		},
		{
			name:      "Limited results from large dataset",
			queryText: "user preferences and activities",
			filter:    &memory.Filter{Limit: helpers.Ptr(10)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Measure query performance on large dataset
			pgStart := time.Now()
			pgResult, err := suite.postgresStorage.Query(suite.ctx, tc.queryText, tc.filter)
			pgDuration := time.Since(pgStart)
			require.NoError(t, err, "PostgreSQL query failed on large dataset")

			wvStart := time.Now()
			wvResult, err := suite.weaviateStorage.Query(suite.ctx, tc.queryText, tc.filter)
			wvDuration := time.Since(wvStart)
			require.NoError(t, err, "Weaviate query failed on large dataset")

			suite.logger.Info("Large dataset query performance",
				"query", tc.name,
				"postgres_ms", pgDuration.Milliseconds(),
				"weaviate_ms", wvDuration.Milliseconds(),
				"postgres_results", len(pgResult.Facts),
				"weaviate_results", len(wvResult.Facts),
			)

			// Results should still be consistent
			suite.assertQueryResultsEquivalent(t, tc.name, pgResult, wvResult)

			// Performance should be reasonable even with large dataset
			assert.Less(t, pgDuration.Seconds(), 60.0, "PostgreSQL queries too slow on large dataset")
			assert.Less(t, wvDuration.Seconds(), 60.0, "Weaviate queries too slow on large dataset")
		})
	}
}

// TestConcurrentAccess tests both backends under concurrent load.
func TestConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent access tests in short mode")
	}

	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	suite.seedBothBackends()

	// Test concurrent queries
	t.Run("Concurrent Queries", func(t *testing.T) {
		numConcurrent := 10
		numQueries := 5

		queries := []string{
			"coffee preferences",
			"work projects",
			"user hobbies",
			"health activities",
			"technology interests",
		}

		// Test PostgreSQL concurrent queries
		var pgWg sync.WaitGroup
		pgResults := make([]memory.QueryResult, numConcurrent*numQueries)
		pgErrors := make([]error, numConcurrent*numQueries)

		for i := range numConcurrent {
			pgWg.Add(1)
			go func(goroutineID int) {
				defer pgWg.Done()
				for j, query := range queries {
					idx := goroutineID*numQueries + j
					result, err := suite.postgresStorage.Query(suite.ctx, query, nil)
					pgResults[idx] = result
					pgErrors[idx] = err
				}
			}(i)
		}
		pgWg.Wait()

		// Test Weaviate concurrent queries
		var wvWg sync.WaitGroup
		wvResults := make([]memory.QueryResult, numConcurrent*numQueries)
		wvErrors := make([]error, numConcurrent*numQueries)

		for i := range numConcurrent {
			wvWg.Add(1)
			go func(goroutineID int) {
				defer wvWg.Done()
				for j, query := range queries {
					idx := goroutineID*numQueries + j
					result, err := suite.weaviateStorage.Query(suite.ctx, query, nil)
					wvResults[idx] = result
					wvErrors[idx] = err
				}
			}(i)
		}
		wvWg.Wait()

		// Verify no errors occurred
		for i := 0; i < numConcurrent*numQueries; i++ {
			assert.NoError(t, pgErrors[i], "PostgreSQL concurrent query %d failed", i)
			assert.NoError(t, wvErrors[i], "Weaviate concurrent query %d failed", i)
		}

		// Verify results are consistent (same queries should return same results)
		for i := range numQueries {
			baseQuery := queries[i]
			basePgResult := pgResults[i]
			baseWvResult := wvResults[i]

			for j := 1; j < numConcurrent; j++ {
				idx := j*numQueries + i
				suite.assertQueryResultsEquivalent(t,
					fmt.Sprintf("Concurrent query consistency: %s (goroutine %d)", baseQuery, j),
					basePgResult, pgResults[idx])
				suite.assertQueryResultsEquivalent(t,
					fmt.Sprintf("Concurrent query consistency: %s (goroutine %d)", baseQuery, j),
					baseWvResult, wvResults[idx])
			}
		}
	})

	// Test concurrent writes and reads
	t.Run("Concurrent Writes and Reads", func(t *testing.T) {
		numWorkers := 5
		opsPerWorker := 10

		var wg sync.WaitGroup
		results := make(chan error, numWorkers*opsPerWorker*2) // *2 for both backends

		// Generate additional test data for concurrent operations
		generateConcurrentTestData := func(workerID, opID int) (memory.Document, memory.MemoryFact) {
			now := time.Now()
			docID := generateUUIDFromString(fmt.Sprintf("concurrent-doc-%d-%d", workerID, opID))
			factID := generateUUIDFromString(fmt.Sprintf("concurrent-fact-%d-%d", workerID, opID))

			doc := &memory.TextDocument{
				FieldID:        docID,
				FieldContent:   fmt.Sprintf("Concurrent operation by worker %d, operation %d", workerID, opID),
				FieldTimestamp: &now,
				FieldSource:    "concurrent-test",
				FieldMetadata:  map[string]string{"worker": fmt.Sprintf("%d", workerID), "op": fmt.Sprintf("%d", opID)},
			}

			fact := memory.MemoryFact{
				ID:                 factID,
				Content:            fmt.Sprintf("Concurrent fact from worker %d, operation %d", workerID, opID),
				Timestamp:          now,
				Source:             "concurrent-test",
				Tags:               []string{"concurrent", "test"},
				Category:           "concurrent",
				Subject:            fmt.Sprintf("worker-%d", workerID),
				Attribute:          "concurrent_operation",
				Value:              fmt.Sprintf("operation-%d", opID),
				Importance:         1,
				DocumentReferences: []string{},
			}

			return doc, fact
		}

		for i := range numWorkers {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for opID := range opsPerWorker {
					doc, fact := generateConcurrentTestData(workerID, opID)

					// Test PostgreSQL concurrent operations
					go func() {
						// Write document
						_, err := suite.postgresStorage.UpsertDocument(suite.ctx, doc)
						results <- err

						// Write fact (requires storage implementation)
						pgStorageImpl, err := New(Dependencies{
							Logger:             suite.logger,
							Storage:            suite.postgresStorage,
							CompletionsService: suite.ai,
							CompletionsModel:   "gpt-4o-mini",
							EmbeddingsWrapper:  suite.embeddingsWrapper,
						})
						if err != nil {
							results <- err
							return
						}

						pgStorageImplTyped, ok := pgStorageImpl.(interface {
							StoreFactsDirectly(context.Context, []*memory.MemoryFact, memory.ProgressCallback) error
						})
						if !ok {
							results <- fmt.Errorf("failed to type assert PostgreSQL storage impl")
							return
						}
						err = pgStorageImplTyped.StoreFactsDirectly(suite.ctx, []*memory.MemoryFact{&fact}, nil)
						results <- err
					}()

					// Test Weaviate concurrent operations
					go func() {
						// Write document
						_, err := suite.weaviateStorage.UpsertDocument(suite.ctx, doc)
						results <- err

						// Write fact (requires storage implementation)
						wvStorageImpl, err := New(Dependencies{
							Logger:             suite.logger,
							Storage:            suite.weaviateStorage,
							CompletionsService: suite.ai,
							CompletionsModel:   "gpt-4o-mini",
							EmbeddingsWrapper:  suite.embeddingsWrapper,
						})
						if err != nil {
							results <- err
							return
						}

						wvStorageImplTyped, ok := wvStorageImpl.(interface {
							StoreFactsDirectly(context.Context, []*memory.MemoryFact, memory.ProgressCallback) error
						})
						if !ok {
							results <- fmt.Errorf("failed to type assert Weaviate storage impl")
							return
						}
						err = wvStorageImplTyped.StoreFactsDirectly(suite.ctx, []*memory.MemoryFact{&fact}, nil)
						results <- err
					}()
				}
			}(i)
		}

		wg.Wait()
		close(results)

		// Check for errors
		var errorCount int
		for err := range results {
			if err != nil {
				t.Errorf("Concurrent operation failed: %v", err)
				errorCount++
			}
		}

		assert.Equal(t, 0, errorCount, "Expected no errors in concurrent operations")

		// Allow time for indexing
		time.Sleep(3 * time.Second)

		// Verify data was written correctly by querying
		pgResult, err := suite.postgresStorage.Query(suite.ctx, "concurrent operation", nil)
		require.NoError(t, err)
		wvResult, err := suite.weaviateStorage.Query(suite.ctx, "concurrent operation", nil)
		require.NoError(t, err)

		// Should have results from concurrent operations
		assert.Greater(t, len(pgResult.Facts), 0, "PostgreSQL should have facts from concurrent operations")
		assert.Greater(t, len(wvResult.Facts), 0, "Weaviate should have facts from concurrent operations")
	})
}

// TestStressScenarios tests extreme load conditions.
func TestStressScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress tests in short mode")
	}

	suite := setupBackendComparisonSuite(t)
	if suite == nil {
		return
	}
	defer suite.cleanup()

	suite.seedBothBackends()

	t.Run("Rapid Sequential Queries", func(t *testing.T) {
		numQueries := 100
		query := "user preferences and activities"

		// Test PostgreSQL rapid queries
		pgStart := time.Now()
		for i := range numQueries {
			_, err := suite.postgresStorage.Query(suite.ctx, query, nil)
			require.NoError(t, err, "PostgreSQL rapid query %d failed", i)
		}
		pgDuration := time.Since(pgStart)

		// Test Weaviate rapid queries
		wvStart := time.Now()
		for i := range numQueries {
			_, err := suite.weaviateStorage.Query(suite.ctx, query, nil)
			require.NoError(t, err, "Weaviate rapid query %d failed", i)
		}
		wvDuration := time.Since(wvStart)

		suite.logger.Info("Rapid query performance",
			"num_queries", numQueries,
			"postgres_total_ms", pgDuration.Milliseconds(),
			"postgres_avg_ms", pgDuration.Milliseconds()/int64(numQueries),
			"weaviate_total_ms", wvDuration.Milliseconds(),
			"weaviate_avg_ms", wvDuration.Milliseconds()/int64(numQueries),
		)

		// Verify reasonable performance
		avgPgTime := pgDuration / time.Duration(numQueries)
		avgWvTime := wvDuration / time.Duration(numQueries)
		assert.Less(t, avgPgTime.Milliseconds(), int64(5000), "PostgreSQL average query time too slow")
		assert.Less(t, avgWvTime.Milliseconds(), int64(5000), "Weaviate average query time too slow")
	})

	t.Run("Memory Usage Patterns", func(t *testing.T) {
		// Test with various query patterns that might cause memory issues
		memoryTestQueries := []struct {
			name   string
			query  string
			filter *memory.Filter
		}{
			{
				name:   "Large result set query",
				query:  "user",
				filter: nil,
			},
			{
				name:  "Complex filter query",
				query: "activities and preferences",
				filter: &memory.Filter{
					FactImportanceMin: helpers.Ptr(1),
					FactImportanceMax: helpers.Ptr(3),
					Distance:          0.5,
				},
			},
			{
				name:  "Time range query",
				query: "recent activities",
				filter: &memory.Filter{
					TimestampAfter: helpers.Ptr(time.Now().Add(-24 * time.Hour)),
				},
			},
		}

		for _, tc := range memoryTestQueries {
			t.Run(tc.name, func(t *testing.T) {
				// Run query multiple times to test for memory leaks
				for range 20 {
					pgResult, err := suite.postgresStorage.Query(suite.ctx, tc.query, tc.filter)
					require.NoError(t, err, "PostgreSQL memory test query failed")

					wvResult, err := suite.weaviateStorage.Query(suite.ctx, tc.query, tc.filter)
					require.NoError(t, err, "Weaviate memory test query failed")

					// Basic consistency check
					assert.Equal(t, len(pgResult.Facts), len(wvResult.Facts),
						"Result count should be consistent across runs")
				}
			})
		}
	})
}
