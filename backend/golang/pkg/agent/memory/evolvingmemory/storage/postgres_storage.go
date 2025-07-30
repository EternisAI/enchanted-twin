package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage/sqlc"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage/types"
)

// PostgresStorage implements the storage interface using PostgreSQL + pgvector.
type PostgresStorage struct {
	db                sqlc.DBTX
	queries           *sqlc.Queries
	logger            *log.Logger
	embeddingsWrapper *EmbeddingWrapper
	connString        string
}

// NewPostgresStorageInput contains the dependencies for PostgresStorage.
type NewPostgresStorageInput struct {
	DB                sqlc.DBTX
	Logger            *log.Logger
	EmbeddingsWrapper *EmbeddingWrapper
	ConnString        string
}

// NewPostgresStorage creates a new PostgresStorage instance.
func NewPostgresStorage(input NewPostgresStorageInput) (Interface, error) {
	if input.DB == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	if input.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if input.EmbeddingsWrapper == nil {
		return nil, fmt.Errorf("embeddingsWrapper cannot be nil")
	}

	storage := &PostgresStorage{
		db:                input.DB,
		queries:           sqlc.New(input.DB),
		logger:            input.Logger,
		embeddingsWrapper: input.EmbeddingsWrapper,
		connString:        input.ConnString,
	}

	return storage, nil
}

// ValidateSchema validates that the database schema is properly set up.
func (s *PostgresStorage) ValidateSchema(ctx context.Context) error {
	// Check if tables exist by trying to query them
	allowedTables := map[string]bool{
		"memory_facts":     true,
		"source_documents": true,
		"document_chunks":  true,
	}
	tables := []string{"memory_facts", "source_documents", "document_chunks"}

	for _, table := range tables {
		// Validate table name against whitelist
		if !allowedTables[table] {
			return fmt.Errorf("invalid table name: %s", table)
		}

		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s LIMIT 1", table)
		row := s.db.QueryRow(ctx, query)
		if err := row.Scan(&count); err != nil {
			return fmt.Errorf("table %s does not exist or is not accessible: %w", table, err)
		}
	}

	// Check if pgvector extension is available
	var extensionExists bool
	row := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')")
	if err := row.Scan(&extensionExists); err != nil {
		return fmt.Errorf("failed to check pgvector extension: %w", err)
	}
	if !extensionExists {
		return fmt.Errorf("pgvector extension is not installed")
	}

	s.logger.Debug("PostgreSQL schema validation successful")
	return nil
}

// GetByID retrieves a memory fact by its ID.
func (s *PostgresStorage) GetByID(ctx context.Context, id string) (*memory.MemoryFact, error) {
	factID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	pgFactID := pgtype.UUID{}
	if err := pgFactID.Scan(factID.String()); err != nil {
		return nil, fmt.Errorf("failed to convert UUID: %w", err)
	}

	fact, err := s.queries.GetMemoryFact(ctx, pgFactID)
	if err != nil {
		return nil, fmt.Errorf("getting memory fact: %w", err)
	}

	return s.convertSQLCToMemoryFact(fact)
}

// Update updates an existing memory fact.
func (s *PostgresStorage) Update(ctx context.Context, id string, fact *memory.MemoryFact, vector []float32) error {
	factID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	pgFactID := pgtype.UUID{}
	if err := pgFactID.Scan(factID.String()); err != nil {
		return fmt.Errorf("failed to convert UUID: %w", err)
	}

	// Convert vector to pgvector format
	pgVector := pgvector.NewVector(vector)
	pgVectorPtr := &pgVector

	// Convert strings to pgtype.Text for nullable fields
	var factCategory, factAttribute, factValue, factTemporalContext, factSensitivity, factFilePath pgtype.Text
	var factSubject types.NullableSanitizedString
	var factImportance pgtype.Int4

	if fact.Category != "" {
		factCategory = pgtype.Text{String: fact.Category, Valid: true}
	}
	if fact.Subject != "" {
		factSubject = types.NullableSanitizedString{
			String: types.NewSanitizedString(fact.Subject),
			Valid:  true,
		}
	}
	if fact.Attribute != "" {
		factAttribute = pgtype.Text{String: fact.Attribute, Valid: true}
	}
	if fact.Value != "" {
		factValue = pgtype.Text{String: fact.Value, Valid: true}
	}
	if fact.TemporalContext != nil {
		factTemporalContext = pgtype.Text{String: *fact.TemporalContext, Valid: true}
	}
	if fact.Sensitivity != "" {
		factSensitivity = pgtype.Text{String: fact.Sensitivity, Valid: true}
	}
	if fact.FilePath != "" {
		factFilePath = pgtype.Text{String: fact.FilePath, Valid: true}
	}
	if fact.Importance > 0 {
		factImportance = pgtype.Int4{Int32: int32(fact.Importance), Valid: true}
	}

	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(fact.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	params := sqlc.UpdateMemoryFactParams{
		ID:                  pgFactID,
		Content:             fact.Content,
		ContentVector:       pgVectorPtr,
		Timestamp:           fact.Timestamp,
		Source:              fact.Source,
		Tags:                fact.Tags,
		DocumentReferences:  fact.DocumentReferences,
		MetadataJson:        metadataJSON,
		FactCategory:        factCategory,
		FactSubject:         factSubject,
		FactAttribute:       factAttribute,
		FactValue:           factValue,
		FactTemporalContext: factTemporalContext,
		FactSensitivity:     factSensitivity,
		FactImportance:      factImportance,
		FactFilePath:        factFilePath,
	}

	_, err = s.queries.UpdateMemoryFact(ctx, params)
	if err != nil {
		return fmt.Errorf("updating memory fact: %w", err)
	}

	s.logger.Infof("Successfully updated memory fact with ID %s", id)
	return nil
}

// Delete removes a memory fact by ID.
func (s *PostgresStorage) Delete(ctx context.Context, id string) error {
	factID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	pgFactID := pgtype.UUID{}
	if err := pgFactID.Scan(factID.String()); err != nil {
		return fmt.Errorf("failed to convert UUID: %w", err)
	}

	err = s.queries.DeleteMemoryFact(ctx, pgFactID)
	if err != nil {
		return fmt.Errorf("deleting memory fact: %w", err)
	}

	s.logger.Infof("Successfully deleted memory fact with ID %s", id)
	return nil
}

// StoreBatch stores multiple objects in a transaction.
func (s *PostgresStorage) StoreBatch(ctx context.Context, objects []*models.Object) error {
	if len(objects) == 0 {
		return nil
	}

	// Create a new connection for the transaction
	conn, err := pgx.Connect(ctx, s.connString)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			s.logger.Error("Failed to close database connection", "error", err)
		}
	}()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if err := tx.Rollback(ctx); err != nil {
				s.logger.Error("Failed to rollback transaction", "error", err)
			}
		}
	}()

	txQueries := s.queries.WithTx(tx)

	for _, obj := range objects {
		if err := s.storeSingleObject(ctx, txQueries, obj); err != nil {
			return fmt.Errorf("storing object: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	committed = true

	s.logger.Infof("Successfully stored batch of %d objects", len(objects))
	return nil
}

// Helper method to store a single object within a transaction.
func (s *PostgresStorage) storeSingleObject(ctx context.Context, txQueries *sqlc.Queries, obj *models.Object) error {
	// Convert object ID to UUID
	var objectID uuid.UUID
	var err error

	if obj.ID != "" {
		objectID, err = uuid.Parse(string(obj.ID))
		if err != nil {
			return fmt.Errorf("invalid object ID: %w", err)
		}
	} else {
		objectID = uuid.New()
	}

	pgObjectID := pgtype.UUID{}
	if err := pgObjectID.Scan(objectID.String()); err != nil {
		return fmt.Errorf("failed to convert UUID: %w", err)
	}

	// Handle different object classes
	switch obj.Class {
	case "MemoryFact":
		return s.storeMemoryFact(ctx, txQueries, pgObjectID, obj)
	case "SourceDocument":
		return s.storeSourceDocument(ctx, txQueries, pgObjectID, obj)
	case "DocumentChunk":
		return s.storeDocumentChunk(ctx, txQueries, pgObjectID, obj)
	default:
		return fmt.Errorf("unknown object class: %s", obj.Class)
	}
}

// Helper method to store a memory fact.
func (s *PostgresStorage) storeMemoryFact(ctx context.Context, txQueries *sqlc.Queries, id pgtype.UUID, obj *models.Object) error {
	props, ok := obj.Properties.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid properties type")
	}

	// Extract required fields
	content, _ := props["content"].(string)
	source, _ := props["source"].(string)
	timestampStr, _ := props["timestamp"].(string)

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		timestamp = time.Now()
	}

	// Extract arrays
	tags := s.extractStringArray(props["tags"])
	documentReferences := s.extractStringArray(props["documentReferences"])

	// Extract optional string fields and convert to pgtype.Text
	var factCategory, factAttribute, factValue, factTemporalContext, factSensitivity, factFilePath pgtype.Text
	var factSubject types.NullableSanitizedString
	var factImportance pgtype.Int4

	if val, ok := props["factCategory"].(string); ok && val != "" {
		factCategory = pgtype.Text{String: val, Valid: true}
	}
	if val, ok := props["factSubject"].(string); ok && val != "" {
		factSubject = types.NullableSanitizedString{
			String: types.NewSanitizedString(val),
			Valid:  true,
		}
	}
	if val, ok := props["factAttribute"].(string); ok && val != "" {
		factAttribute = pgtype.Text{String: val, Valid: true}
	}
	if val, ok := props["factValue"].(string); ok && val != "" {
		factValue = pgtype.Text{String: val, Valid: true}
	}
	if val, ok := props["factTemporalContext"].(string); ok && val != "" {
		factTemporalContext = pgtype.Text{String: val, Valid: true}
	}
	if val, ok := props["factSensitivity"].(string); ok && val != "" {
		factSensitivity = pgtype.Text{String: val, Valid: true}
	}
	if val, ok := props["factFilePath"].(string); ok && val != "" {
		factFilePath = pgtype.Text{String: val, Valid: true}
	}
	// Handle factImportance - can be int or float64 from JSON conversion
	if val, ok := props["factImportance"].(int); ok {
		factImportance = pgtype.Int4{Int32: int32(val), Valid: true}
	} else if val, ok := props["factImportance"].(float64); ok {
		factImportance = pgtype.Int4{Int32: int32(val), Valid: true}
	}

	// Convert metadata to JSON
	metadataJSON := []byte("{}")
	if metadataStr, ok := props["metadataJson"].(string); ok {
		metadataJSON = []byte(metadataStr)
	}

	// Convert vector
	pgVector := pgvector.NewVector(obj.Vector)

	params := sqlc.CreateMemoryFactParams{
		ID:                  id,
		Content:             content,
		ContentVector:       &pgVector,
		Timestamp:           timestamp,
		Source:              source,
		Tags:                tags,
		DocumentReferences:  documentReferences,
		MetadataJson:        metadataJSON,
		FactCategory:        factCategory,
		FactSubject:         factSubject,
		FactAttribute:       factAttribute,
		FactValue:           factValue,
		FactTemporalContext: factTemporalContext,
		FactSensitivity:     factSensitivity,
		FactImportance:      factImportance,
		FactFilePath:        factFilePath,
	}

	_, err = txQueries.CreateMemoryFact(ctx, params)
	return err
}

// Helper method to store a source document.
func (s *PostgresStorage) storeSourceDocument(ctx context.Context, txQueries *sqlc.Queries, id pgtype.UUID, obj *models.Object) error {
	props, ok := obj.Properties.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid properties type")
	}

	content, _ := props["content"].(string)
	contentHash, _ := props["contentHash"].(string)
	documentType, _ := props["documentType"].(string)
	originalID, _ := props["originalId"].(string)

	metadataJSON := []byte("{}")
	if metadataStr, ok := props["metadata"].(string); ok {
		metadataJSON = []byte(metadataStr)
	}

	params := sqlc.CreateSourceDocumentParams{
		ID:           id,
		Content:      content,
		ContentHash:  contentHash,
		DocumentType: documentType,
		OriginalID:   originalID,
		MetadataJson: metadataJSON,
	}

	_, err := txQueries.CreateSourceDocument(ctx, params)
	return err
}

// Helper method to store a document chunk.
func (s *PostgresStorage) storeDocumentChunk(ctx context.Context, txQueries *sqlc.Queries, id pgtype.UUID, obj *models.Object) error {
	props, ok := obj.Properties.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid properties type")
	}

	content, _ := props["content"].(string)
	source, _ := props["source"].(string)
	originalDocumentID, _ := props["originalDocumentId"].(string)

	chunkIndex := int32(0)
	if val, ok := props["chunkIndex"].(float64); ok {
		chunkIndex = int32(val)
	}

	tags := s.extractStringArray(props["tags"])

	var filePath pgtype.Text
	if val, ok := props["filePath"].(string); ok && val != "" {
		filePath = pgtype.Text{String: val, Valid: true}
	}

	metadataJSON := []byte("{}")
	if metadataStr, ok := props["metadata"].(string); ok {
		metadataJSON = []byte(metadataStr)
	}

	pgVector := pgvector.NewVector(obj.Vector)

	params := sqlc.CreateDocumentChunkParams{
		ID:                 id,
		Content:            content,
		ContentVector:      &pgVector,
		ChunkIndex:         chunkIndex,
		OriginalDocumentID: originalDocumentID,
		Source:             source,
		FilePath:           filePath,
		Tags:               tags,
		MetadataJson:       metadataJSON,
	}

	_, err := txQueries.CreateDocumentChunk(ctx, params)
	return err
}

// Helper method to extract string arrays from properties.
func (s *PostgresStorage) extractStringArray(val interface{}) []string {
	if arr, ok := val.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return []string{}
}

// Helper method to convert SQLC model to memory.MemoryFact.
func (s *PostgresStorage) convertSQLCToMemoryFact(fact sqlc.MemoryFact) (*memory.MemoryFact, error) {
	// Convert UUID from pgtype.UUID
	if !fact.ID.Valid {
		return nil, fmt.Errorf("invalid UUID in fact")
	}

	// Convert pgtype.UUID to uuid.UUID using proper method
	factUUID, err := uuid.FromBytes(fact.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert UUID from bytes: %w", err)
	}

	// Convert nullable fields
	var temporalContext *string
	if fact.FactTemporalContext.Valid {
		temporalContext = &fact.FactTemporalContext.String
	}

	var category, subject, attribute, value, sensitivity, filePath string
	var importance int

	if fact.FactCategory.Valid {
		category = fact.FactCategory.String
	}
	if fact.FactSubject.Valid {
		subject = fact.FactSubject.String.String()
	}
	if fact.FactAttribute.Valid {
		attribute = fact.FactAttribute.String
	}
	if fact.FactValue.Valid {
		value = fact.FactValue.String
	}
	if fact.FactSensitivity.Valid {
		sensitivity = fact.FactSensitivity.String
	}
	if fact.FactFilePath.Valid {
		filePath = fact.FactFilePath.String
	}
	if fact.FactImportance.Valid {
		importance = int(fact.FactImportance.Int32)
	}

	// Parse metadata
	metadata := make(map[string]string)
	if len(fact.MetadataJson) > 0 && string(fact.MetadataJson) != "{}" {
		if err := json.Unmarshal(fact.MetadataJson, &metadata); err != nil {
			s.logger.Warn("Failed to unmarshal metadata", "error", err)
		}
	}

	return &memory.MemoryFact{
		ID:                 factUUID.String(),
		Content:            fact.Content,
		Timestamp:          fact.Timestamp,
		Category:           category,
		Subject:            subject,
		Attribute:          attribute,
		Value:              value,
		TemporalContext:    temporalContext,
		Sensitivity:        sensitivity,
		Importance:         importance,
		Source:             fact.Source,
		DocumentReferences: fact.DocumentReferences,
		Tags:               fact.Tags,
		FilePath:           filePath,
		Metadata:           metadata,
	}, nil
}

// Query performs vector similarity search with optional filters.
func (s *PostgresStorage) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	if queryText == "" {
		return memory.QueryResult{}, fmt.Errorf("query text cannot be empty")
	}

	// Generate embedding for the query
	embedding, err := s.embeddingsWrapper.Embedding(ctx, queryText)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("generating embedding for query: %w", err)
	}

	pgVector := pgvector.NewVector(embedding)

	// Set default limit if not specified (matches Weaviate's default)
	limit := int32(100)
	if filter != nil && filter.Limit != nil && *filter.Limit > 0 {
		limit = int32(*filter.Limit)
	}

	// When distance filtering is applied, increase limit significantly to match Weaviate behavior
	// Weaviate returns ALL results that pass distance threshold, not limited by the original limit
	actualLimit := limit
	if filter != nil && filter.Distance > 0 {
		// Use a much higher limit to get enough candidates for distance filtering
		// This matches Weaviate's behavior where distance filtering determines result count
		actualLimit = 1000 // Get up to 1000 candidates, then distance filter determines final count
	}

	// Build query parameters based on filter
	var source, category, subject, filePath *string
	var importance, minImportance, maxImportance *int32
	var startTime, endTime *time.Time
	var tags, documentRefs []string

	if filter != nil {
		if filter.Source != nil && *filter.Source != "" {
			source = filter.Source
		}
		if filter.FactCategory != nil && *filter.FactCategory != "" {
			category = filter.FactCategory
		}
		if filter.Subject != nil && *filter.Subject != "" {
			subject = filter.Subject
		}
		if filter.FactFilePath != nil && *filter.FactFilePath != "" {
			filePath = filter.FactFilePath
		}
		if filter.FactImportance != nil && *filter.FactImportance > 0 {
			imp := int32(*filter.FactImportance)
			importance = &imp
		}
		if filter.FactImportanceMin != nil && *filter.FactImportanceMin > 0 {
			minImp := int32(*filter.FactImportanceMin)
			minImportance = &minImp
		}
		if filter.FactImportanceMax != nil && *filter.FactImportanceMax > 0 {
			maxImp := int32(*filter.FactImportanceMax)
			maxImportance = &maxImp
		}
		if filter.TimestampAfter != nil && !filter.TimestampAfter.IsZero() {
			startTime = filter.TimestampAfter
		}
		if filter.TimestampBefore != nil && !filter.TimestampBefore.IsZero() {
			endTime = filter.TimestampBefore
		}
		if filter.Tags != nil && len(filter.Tags.All) > 0 {
			tags = filter.Tags.All
		} else if filter.Tags != nil && len(filter.Tags.Any) > 0 {
			tags = filter.Tags.Any
		}
		if len(filter.DocumentReferences) > 0 {
			documentRefs = filter.DocumentReferences
		}
	}

	// Convert nullable parameters to pgtype
	var pgSource, pgCategory, pgFilePath pgtype.Text
	var pgSubject types.NullableSanitizedString
	var pgImportance, pgMinImportance, pgMaxImportance pgtype.Int4
	var pgStartTime, pgEndTime pgtype.Timestamptz
	var pgTags, pgDocumentRefs []string

	if source != nil {
		pgSource = pgtype.Text{String: *source, Valid: true}
	}
	if category != nil {
		pgCategory = pgtype.Text{String: *category, Valid: true}
	}
	if subject != nil {
		pgSubject = types.NullableSanitizedString{
			String: types.NewSanitizedString(*subject),
			Valid:  true,
		}
	}
	if filePath != nil {
		pgFilePath = pgtype.Text{String: *filePath, Valid: true}
	}
	if importance != nil {
		pgImportance = pgtype.Int4{Int32: *importance, Valid: true}
	}
	if minImportance != nil {
		pgMinImportance = pgtype.Int4{Int32: *minImportance, Valid: true}
	}
	if maxImportance != nil {
		pgMaxImportance = pgtype.Int4{Int32: *maxImportance, Valid: true}
	}
	if startTime != nil {
		pgStartTime = pgtype.Timestamptz{Time: *startTime, Valid: true}
	}
	if endTime != nil {
		pgEndTime = pgtype.Timestamptz{Time: *endTime, Valid: true}
	}
	if tags != nil {
		pgTags = tags
	}
	if documentRefs != nil {
		pgDocumentRefs = documentRefs
	}

	// Convert nullable parameters to the correct format for SQLC
	var sourceParam, categoryParam, subjectParam, filePathParam string
	var importanceParam, minImportanceParam, maxImportanceParam int32

	if pgSource.Valid {
		sourceParam = pgSource.String
	}
	if pgCategory.Valid {
		categoryParam = pgCategory.String
	}
	if pgSubject.Valid {
		subjectParam = pgSubject.String.String()
	}
	if pgFilePath.Valid {
		filePathParam = pgFilePath.String
	}
	if pgImportance.Valid {
		importanceParam = pgImportance.Int32
	}
	if pgMinImportance.Valid {
		minImportanceParam = pgMinImportance.Int32
	}
	if pgMaxImportance.Valid {
		maxImportanceParam = pgMaxImportance.Int32
	}

	// Set distance parameter
	var distanceParam float64
	if filter != nil && filter.Distance > 0 {
		distanceParam = float64(filter.Distance)
	}

	// Execute vector similarity query
	params := sqlc.QueryMemoryFactsByVectorParams{
		ContentVector: &pgVector,
		Column2:       sourceParam,        // source
		Column3:       categoryParam,      // fact_category
		Column4:       subjectParam,       // fact_subject
		Column5:       importanceParam,    // fact_importance
		Column6:       minImportanceParam, // fact_importance >= (min)
		Column7:       maxImportanceParam, // fact_importance <= (max)
		Column8:       pgStartTime,        // timestamp >= (after)
		Column9:       pgEndTime,          // timestamp <= (before)
		Column10:      filePathParam,      // fact_file_path
		Column11:      pgTags,             // tags
		Column12:      pgDocumentRefs,     // document_references
		Limit:         actualLimit,
		Column14:      distanceParam, // distance threshold
	}

	s.logger.Debug("Vector query parameters",
		"source", sourceParam,
		"category", categoryParam,
		"subject", subjectParam,
		"importance", importanceParam,
		"min_importance", minImportanceParam,
		"max_importance", maxImportanceParam,
		"distance_threshold", distanceParam,
		"requested_limit", limit,
		"actual_limit", actualLimit)

	rows, err := s.queries.QueryMemoryFactsByVector(ctx, params)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("executing vector query: %w", err)
	}

	s.logger.Debug("Vector query raw results", "row_count", len(rows))

	// Convert results to memory facts
	facts := make([]memory.MemoryFact, 0, len(rows))

	for _, row := range rows {
		fact, err := s.convertSQLCRowToMemoryFact(row)
		if err != nil {
			s.logger.Warn("Failed to convert row to memory fact", "error", err)
			continue
		}

		facts = append(facts, *fact)
	}

	s.logger.Debug("Vector query completed",
		"query", queryText,
		"results", len(facts),
		"requested_limit", limit,
		"actual_limit", actualLimit,
		"distance_threshold", distanceParam)

	return memory.QueryResult{
		Facts: facts,
	}, nil
}

// DeleteAll removes all memory facts (used for testing).
func (s *PostgresStorage) DeleteAll(ctx context.Context) error {
	// Create a new connection for the transaction
	conn, err := pgx.Connect(ctx, s.connString)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			s.logger.Error("Failed to close database connection", "error", err)
		}
	}()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			s.logger.Error("Failed to rollback transaction", "error", err)
		}
	}()

	// Delete all records from all tables
	_, err = tx.Exec(ctx, "DELETE FROM document_chunks")
	if err != nil {
		return fmt.Errorf("deleting document chunks: %w", err)
	}

	_, err = tx.Exec(ctx, "DELETE FROM source_documents")
	if err != nil {
		return fmt.Errorf("deleting source documents: %w", err)
	}

	_, err = tx.Exec(ctx, "DELETE FROM memory_facts")
	if err != nil {
		return fmt.Errorf("deleting memory facts: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	s.logger.Debug("All memory facts deleted from PostgreSQL storage")
	return nil
}

func (s *PostgresStorage) EnsureSchemaExists(ctx context.Context) error {
	return s.ValidateSchema(ctx)
}


func (s *PostgresStorage) GetDocumentReferences(ctx context.Context, memoryID string) ([]*DocumentReference, error) {
	// Get the memory fact to retrieve its document references
	memoryFact, err := s.GetByID(ctx, memoryID)
	if err != nil {
		return nil, fmt.Errorf("getting memory fact: %w", err)
	}

	if len(memoryFact.DocumentReferences) == 0 {
		return []*DocumentReference{}, nil
	}

	// Convert document reference IDs to UUIDs for batch query
	documentUUIDs := make([]uuid.UUID, 0, len(memoryFact.DocumentReferences))
	for _, ref := range memoryFact.DocumentReferences {
		docUUID, err := uuid.Parse(ref)
		if err != nil {
			s.logger.Warn("Invalid document reference UUID", "ref", ref, "error", err)
			continue
		}
		documentUUIDs = append(documentUUIDs, docUUID)
	}

	if len(documentUUIDs) == 0 {
		return []*DocumentReference{}, nil
	}

	// Convert UUIDs to pgtype.UUID for the query
	pgDocumentUUIDs := make([]pgtype.UUID, 0, len(documentUUIDs))
	for _, docUUID := range documentUUIDs {
		pgDocID := pgtype.UUID{}
		if err := pgDocID.Scan(docUUID.String()); err != nil {
			s.logger.Warn("Failed to convert document UUID", "uuid", docUUID, "error", err)
			continue
		}
		pgDocumentUUIDs = append(pgDocumentUUIDs, pgDocID)
	}

	// Query source documents
	docs, err := s.queries.GetSourceDocumentsBatch(ctx, pgDocumentUUIDs)
	if err != nil {
		return nil, fmt.Errorf("getting source documents: %w", err)
	}

	// Convert to DocumentReference format
	references := make([]*DocumentReference, 0, len(docs))
	for _, doc := range docs {
		docUUID, err := uuid.FromBytes(doc.ID.Bytes[:])
		if err != nil {
			s.logger.Warn("Failed to convert document UUID from bytes", "error", err)
			continue
		}

		references = append(references, &DocumentReference{
			ID:      docUUID.String(),
			Content: doc.Content,
			Type:    doc.DocumentType,
		})
	}

	return references, nil
}

func (s *PostgresStorage) UpsertDocument(ctx context.Context, doc memory.Document) (string, error) {
	// Determine document type based on concrete type
	var docType string
	switch doc.(type) {
	case *memory.ConversationDocument:
		docType = "conversation"
	case *memory.TextDocument:
		docType = "text"
	case *memory.FileDocument:
		docType = "file"
	default:
		docType = "unknown"
	}

	// Generate content hash for deduplication
	contentHash := sha256hex(doc.Content())

	// Check if document already exists by hash
	existingDoc, err := s.queries.GetSourceDocumentByHash(ctx, contentHash)
	if err == nil {
		// Document exists, return its ID
		docUUID, err := uuid.FromBytes(existingDoc.ID.Bytes[:])
		if err != nil {
			return "", fmt.Errorf("failed to convert existing document UUID: %w", err)
		}
		return docUUID.String(), nil
	}

	// Document doesn't exist, create new one
	docID := uuid.New()
	pgDocID := pgtype.UUID{}
	if err := pgDocID.Scan(docID.String()); err != nil {
		return "", fmt.Errorf("failed to convert UUID: %w", err)
	}

	// Convert metadata to JSON
	metadataJSON := []byte("{}")
	if doc.Metadata() != nil {
		if jsonData, err := json.Marshal(doc.Metadata()); err == nil {
			metadataJSON = jsonData
		}
	}

	params := sqlc.CreateSourceDocumentParams{
		ID:           pgDocID,
		Content:      doc.Content(),
		ContentHash:  contentHash,
		DocumentType: docType,
		OriginalID:   doc.ID(),
		MetadataJson: metadataJSON,
	}

	_, err = s.queries.CreateSourceDocument(ctx, params)
	if err != nil {
		return "", fmt.Errorf("creating source document: %w", err)
	}

	s.logger.Debug("Successfully upserted document", "id", docID.String(), "type", docType)
	return docID.String(), nil
}

func (s *PostgresStorage) GetStoredDocument(ctx context.Context, documentID string) (*StoredDocument, error) {
	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}

	pgDocID := pgtype.UUID{}
	if err := pgDocID.Scan(docUUID.String()); err != nil {
		return nil, fmt.Errorf("failed to convert UUID: %w", err)
	}

	doc, err := s.queries.GetSourceDocument(ctx, pgDocID)
	if err != nil {
		return nil, fmt.Errorf("getting source document: %w", err)
	}

	// Parse metadata
	metadata := make(map[string]interface{})
	if len(doc.MetadataJson) > 0 && string(doc.MetadataJson) != "{}" {
		if err := json.Unmarshal(doc.MetadataJson, &metadata); err != nil {
			s.logger.Warn("Failed to unmarshal document metadata", "error", err)
		}
	}

	storedDocUUID, err := uuid.FromBytes(doc.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert document UUID: %w", err)
	}

	// Convert metadata from map[string]interface{} to map[string]string
	metadataStr := make(map[string]string)
	for k, v := range metadata {
		if str, ok := v.(string); ok {
			metadataStr[k] = str
		} else {
			metadataStr[k] = fmt.Sprintf("%v", v)
		}
	}

	return &StoredDocument{
		ID:          storedDocUUID.String(),
		Content:     doc.Content,
		ContentHash: doc.ContentHash,
		Type:        doc.DocumentType,
		OriginalID:  doc.OriginalID,
		Metadata:    metadataStr,
		CreatedAt:   doc.CreatedAt,
	}, nil
}

func (s *PostgresStorage) GetStoredDocumentsBatch(ctx context.Context, documentIDs []string) ([]*StoredDocument, error) {
	if len(documentIDs) == 0 {
		return []*StoredDocument{}, nil
	}

	// Convert IDs to UUIDs
	docUUIDs := make([]uuid.UUID, 0, len(documentIDs))
	for _, id := range documentIDs {
		docUUID, err := uuid.Parse(id)
		if err != nil {
			s.logger.Warn("Invalid document ID in batch", "id", id, "error", err)
			continue
		}
		docUUIDs = append(docUUIDs, docUUID)
	}

	if len(docUUIDs) == 0 {
		return []*StoredDocument{}, nil
	}

	// Convert UUIDs to pgtype.UUID for the query
	pgDocumentUUIDs := make([]pgtype.UUID, 0, len(docUUIDs))
	for _, docUUID := range docUUIDs {
		pgDocID := pgtype.UUID{}
		if err := pgDocID.Scan(docUUID.String()); err != nil {
			s.logger.Warn("Failed to convert document UUID", "uuid", docUUID, "error", err)
			continue
		}
		pgDocumentUUIDs = append(pgDocumentUUIDs, pgDocID)
	}

	docs, err := s.queries.GetSourceDocumentsBatch(ctx, pgDocumentUUIDs)
	if err != nil {
		return nil, fmt.Errorf("getting source documents batch: %w", err)
	}

	// Convert to StoredDocument format
	storedDocs := make([]*StoredDocument, 0, len(docs))
	for _, doc := range docs {
		// Parse metadata
		metadata := make(map[string]interface{})
		if len(doc.MetadataJson) > 0 && string(doc.MetadataJson) != "{}" {
			if err := json.Unmarshal(doc.MetadataJson, &metadata); err != nil {
				s.logger.Warn("Failed to unmarshal document metadata", "error", err)
			}
		}

		docUUID, err := uuid.FromBytes(doc.ID.Bytes[:])
		if err != nil {
			s.logger.Warn("Failed to convert document UUID", "error", err)
			continue
		}

		// Convert metadata from map[string]interface{} to map[string]string
		metadataStr := make(map[string]string)
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				metadataStr[k] = str
			} else {
				metadataStr[k] = fmt.Sprintf("%v", v)
			}
		}

		storedDocs = append(storedDocs, &StoredDocument{
			ID:          docUUID.String(),
			Content:     doc.Content,
			ContentHash: doc.ContentHash,
			Type:        doc.DocumentType,
			OriginalID:  doc.OriginalID,
			Metadata:    metadataStr,
			CreatedAt:   doc.CreatedAt,
		})
	}

	return storedDocs, nil
}

func (s *PostgresStorage) StoreDocumentChunksBatch(ctx context.Context, chunks []*DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Create a new connection for the transaction
	conn, err := pgx.Connect(ctx, s.connString)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			s.logger.Error("Failed to close database connection", "error", err)
		}
	}()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.Rollback(ctx); err != nil {
				s.logger.Error("Failed to rollback transaction", "error", err)
			}
		}
	}()

	txQueries := s.queries.WithTx(tx)

	for _, chunk := range chunks {
		chunkID := uuid.New()
		pgChunkID := pgtype.UUID{}
		if err := pgChunkID.Scan(chunkID.String()); err != nil {
			return fmt.Errorf("failed to convert chunk UUID: %w", err)
		}

		// Generate embedding for chunk content
		embedding, err := s.embeddingsWrapper.Embedding(ctx, chunk.Content)
		if err != nil {
			return fmt.Errorf("generating embedding for chunk: %w", err)
		}

		pgVector := pgvector.NewVector(embedding)

		var filePath pgtype.Text
		if chunk.FilePath != "" {
			filePath = pgtype.Text{String: chunk.FilePath, Valid: true}
		}

		// Convert metadata to JSON
		metadataJSON := []byte("{}")
		if chunk.Metadata != nil {
			if jsonData, err := json.Marshal(chunk.Metadata); err == nil {
				metadataJSON = jsonData
			}
		}

		params := sqlc.CreateDocumentChunkParams{
			ID:                 pgChunkID,
			Content:            chunk.Content,
			ContentVector:      &pgVector,
			ChunkIndex:         int32(chunk.ChunkIndex),
			OriginalDocumentID: chunk.OriginalDocumentID,
			Source:             chunk.Source,
			FilePath:           filePath,
			Tags:               chunk.Tags,
			MetadataJson:       metadataJSON,
		}

		_, err = txQueries.CreateDocumentChunk(ctx, params)
		if err != nil {
			return fmt.Errorf("creating document chunk: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	committed = true

	s.logger.Debug("Successfully stored document chunks batch", "count", len(chunks))
	return nil
}

func (s *PostgresStorage) QueryDocumentChunks(ctx context.Context, queryText string, filter *memory.Filter) ([]*DocumentChunk, error) {
	if queryText == "" {
		return []*DocumentChunk{}, fmt.Errorf("query text cannot be empty")
	}

	// Generate embedding for the query
	embedding, err := s.embeddingsWrapper.Embedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("generating embedding for query: %w", err)
	}

	pgVector := pgvector.NewVector(embedding)

	// Set default limit if not specified (matches Weaviate's default)
	limit := int32(100)
	if filter != nil && filter.Limit != nil && *filter.Limit > 0 {
		limit = int32(*filter.Limit)
	}

	// When distance filtering is applied, increase limit significantly to match Weaviate behavior
	actualLimit := limit
	if filter != nil && filter.Distance > 0 {
		actualLimit = 1000 // Get up to 1000 candidates, then distance filter determines final count
	}

	// Build query parameters based on filter
	var source, filePath *string
	var tags []string

	if filter != nil {
		if filter.Source != nil && *filter.Source != "" {
			source = filter.Source
		}
		if filter.FactFilePath != nil && *filter.FactFilePath != "" {
			filePath = filter.FactFilePath
		}
		if filter.Tags != nil && len(filter.Tags.All) > 0 {
			tags = filter.Tags.All
		} else if filter.Tags != nil && len(filter.Tags.Any) > 0 {
			tags = filter.Tags.Any
		}
	}

	// Convert nullable parameters to pgtype
	var pgSource, pgFilePath pgtype.Text
	var pgTags []string

	if source != nil {
		pgSource = pgtype.Text{String: *source, Valid: true}
	}
	if filePath != nil {
		pgFilePath = pgtype.Text{String: *filePath, Valid: true}
	}
	if tags != nil {
		pgTags = tags
	}

	var sourceParam, filePathParam string
	if pgSource.Valid {
		sourceParam = pgSource.String
	}
	if pgFilePath.Valid {
		filePathParam = pgFilePath.String
	}

	// Set distance parameter
	var distanceParam float64
	if filter != nil && filter.Distance > 0 {
		distanceParam = float64(filter.Distance)
	}

	params := sqlc.QueryDocumentChunksByVectorParams{
		ContentVector: &pgVector,
		Column2:       sourceParam,   // source parameter
		Column3:       filePathParam, // file_path parameter
		Column4:       pgTags,        // tags parameter
		Limit:         actualLimit,
		Column6:       distanceParam, // distance threshold
	}

	rows, err := s.queries.QueryDocumentChunksByVector(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("executing document chunk vector query: %w", err)
	}

	// Convert results to DocumentChunk format
	chunks := make([]*DocumentChunk, 0, len(rows))
	for _, row := range rows {
		// Parse metadata
		metadata := make(map[string]interface{})
		if len(row.MetadataJson) > 0 && string(row.MetadataJson) != "{}" {
			if err := json.Unmarshal(row.MetadataJson, &metadata); err != nil {
				s.logger.Warn("Failed to unmarshal chunk metadata", "error", err)
			}
		}

		chunkUUID, err := uuid.FromBytes(row.ID.Bytes[:])
		if err != nil {
			s.logger.Warn("Failed to convert chunk UUID", "error", err)
			continue
		}

		var filePath string
		if row.FilePath.Valid {
			filePath = row.FilePath.String
		}

		chunks = append(chunks, &DocumentChunk{
			ID:                 chunkUUID.String(),
			Content:            row.Content,
			ChunkIndex:         int(row.ChunkIndex),
			OriginalDocumentID: row.OriginalDocumentID,
			Source:             row.Source,
			FilePath:           filePath,
			Tags:               row.Tags,
			Metadata:           make(map[string]string),
			CreatedAt:          row.CreatedAt,
		})
	}

	s.logger.Debug("Document chunk vector query completed",
		"query", queryText,
		"results", len(chunks),
		"requested_limit", limit,
		"actual_limit", actualLimit)

	return chunks, nil
}

func (s *PostgresStorage) GetFactsByIDs(ctx context.Context, factIDs []string) ([]*memory.MemoryFact, error) {
	if len(factIDs) == 0 {
		return []*memory.MemoryFact{}, nil
	}

	// Convert IDs to UUIDs
	factUUIDs := make([]uuid.UUID, 0, len(factIDs))
	for _, id := range factIDs {
		factUUID, err := uuid.Parse(id)
		if err != nil {
			s.logger.Warn("Invalid fact ID in batch", "id", id, "error", err)
			continue
		}
		factUUIDs = append(factUUIDs, factUUID)
	}

	if len(factUUIDs) == 0 {
		return []*memory.MemoryFact{}, nil
	}

	// Convert UUIDs to pgtype.UUID for the query
	pgFactUUIDs := make([]pgtype.UUID, 0, len(factUUIDs))
	for _, factUUID := range factUUIDs {
		pgFactID := pgtype.UUID{}
		if err := pgFactID.Scan(factUUID.String()); err != nil {
			s.logger.Warn("Failed to convert fact UUID", "uuid", factUUID, "error", err)
			continue
		}
		pgFactUUIDs = append(pgFactUUIDs, pgFactID)
	}

	facts, err := s.queries.GetMemoryFactsByIDs(ctx, pgFactUUIDs)
	if err != nil {
		return nil, fmt.Errorf("getting memory facts by IDs: %w", err)
	}

	// Convert to memory.MemoryFact format
	memoryFacts := make([]*memory.MemoryFact, 0, len(facts))
	for _, fact := range facts {
		memoryFact, err := s.convertSQLCToMemoryFact(fact)
		if err != nil {
			s.logger.Warn("Failed to convert fact to memory fact", "error", err)
			continue
		}
		memoryFacts = append(memoryFacts, memoryFact)
	}

	return memoryFacts, nil
}

// Helper method to convert SQLC query row to memory.MemoryFact.
func (s *PostgresStorage) convertSQLCRowToMemoryFact(row sqlc.QueryMemoryFactsByVectorRow) (*memory.MemoryFact, error) {
	// Convert the embedded MemoryFact from the row
	fact := sqlc.MemoryFact{
		ID:                  row.ID,
		Content:             row.Content,
		ContentVector:       row.ContentVector,
		Timestamp:           row.Timestamp,
		Source:              row.Source,
		Tags:                row.Tags,
		DocumentReferences:  row.DocumentReferences,
		MetadataJson:        row.MetadataJson,
		FactCategory:        row.FactCategory,
		FactSubject:         row.FactSubject,
		FactAttribute:       row.FactAttribute,
		FactValue:           row.FactValue,
		FactTemporalContext: row.FactTemporalContext,
		FactSensitivity:     row.FactSensitivity,
		FactImportance:      row.FactImportance,
		FactFilePath:        row.FactFilePath,
		CreatedAt:           row.CreatedAt,
	}

	return s.convertSQLCToMemoryFact(fact)
}
