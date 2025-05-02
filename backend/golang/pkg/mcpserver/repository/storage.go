package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

type Repository struct {
	logger *log.Logger
	db     *sqlx.DB
}

func NewRepository(logger *log.Logger, db *sqlx.DB) *Repository {
	return &Repository{
		logger: logger,
		db:     db,
	}
}

const insertMCPServerQuery = `
INSERT INTO mcp_servers (id, name, command, args, envs, type, created_at, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
`

func (r *Repository) AddMCPServer(ctx context.Context, input *model.ConnectMCPServerInput, enabled *bool) (*model.MCPServer, error) {
	newID := uuid.NewString()
	createdAt := time.Now().Format(time.RFC3339)

	argsJSON, err := json.Marshal(input.Args)
	if err != nil {
		r.logger.Error("failed to marshal command for mcp server", "error", err, "name", input.Name)
		return nil, err
	}

	envsJSON, err := json.Marshal(input.Envs)
	if err != nil {
		r.logger.Error("failed to marshal envs for mcp server", "error", err, "name", input.Name)
		return nil, err
	}

	enabledValue := false
	if enabled != nil {
		enabledValue = *enabled
	}

	_, err = r.db.ExecContext(ctx, insertMCPServerQuery, newID, input.Name, input.Command, string(argsJSON), string(envsJSON), input.Type.String(), createdAt, enabledValue)
	if err != nil {
		r.logger.Error("failed to insert mcp server", "error", err, "name", input.Name)
		return nil, err
	}

	envsModel := make([]*model.KeyValue, len(input.Envs))
	for i, env := range input.Envs {
		envsModel[i] = &model.KeyValue{
			Key:   env.Key,
			Value: env.Value,
		}
	}

	mcpServer := &model.MCPServer{
		ID:        newID,
		Name:      input.Name,
		Command:   input.Command,
		Args:      input.Args,
		Envs:      envsModel,
		CreatedAt: createdAt,
		Enabled:   enabledValue,
		Type:      input.Type,
	}

	return mcpServer, nil
}

const selectMCPServersQuery = `
SELECT id, name, command, args, envs, type, created_at, enabled FROM mcp_servers ORDER BY created_at DESC;
`

type dbMCPServer struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	Command   string `db:"command"` // JSON string from DB
	Args      string `db:"args"`    // JSON string from DB
	Envs      string `db:"envs"`    // JSON string from DB
	Type      string `db:"type"`
	CreatedAt string `db:"created_at"`
	Enabled   bool   `db:"enabled"`
}

func (r *Repository) GetMCPServers(ctx context.Context) ([]*model.MCPServer, error) {
	var dbServers []dbMCPServer
	err := r.db.SelectContext(ctx, &dbServers, selectMCPServersQuery)
	if err != nil {
		r.logger.Error("failed to select mcp servers", "error", err)
		return nil, err
	}

	servers := make([]*model.MCPServer, 0, len(dbServers))
	for _, dbServer := range dbServers {
		var argsSlice []string
		if dbServer.Args != "" {
			if err := json.Unmarshal([]byte(dbServer.Args), &argsSlice); err != nil {
				r.logger.Error("failed to unmarshal args for mcp server", "error", err, "id", dbServer.ID)
				// Skip this server or return error? Skipping for now.
				continue
			}
		}
		var envsSlice []*model.KeyValue
		if dbServer.Envs != "" {
			if err := json.Unmarshal([]byte(dbServer.Envs), &envsSlice); err != nil {
				r.logger.Error("failed to unmarshal envs for mcp server", "error", err, "id", dbServer.ID)
				// Skip this server or return error? Skipping for now.
				continue
			}
		}

		var mcpType model.MCPServerType
		if err := mcpType.UnmarshalGQL(dbServer.Type); err != nil {
			r.logger.Error("failed to unmarshal type for mcp server", "error", err, "id", dbServer.ID)
			// Skip this server or return error? Skipping for now.
			continue
		}

		servers = append(servers, &model.MCPServer{
			ID:        dbServer.ID,
			Name:      dbServer.Name,
			Command:   dbServer.Command,
			Args:      argsSlice,
			Envs:      envsSlice,
			CreatedAt: dbServer.CreatedAt,
			Enabled:   dbServer.Enabled,
			Type:      mcpType,
		})
	}

	return servers, nil
}
