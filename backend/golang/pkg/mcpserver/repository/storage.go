package repository

import (
	"context"
	"encoding/json"

	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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
INSERT INTO mcp_servers (id, name, command, envs, created_at)
VALUES ($1, $2, $3, $4, $5);
`

func (r *Repository) AddMCPServer(ctx context.Context, name string, command []string, envs []string) (*model.MCPServer, error) {
	newID := uuid.NewString()
	createdAt := time.Now().Format(time.RFC3339)

	commandJSON, err := json.Marshal(command)
	if err != nil {
		r.logger.Error("failed to marshal command for mcp server", "error", err, "name", name)
		return nil, err
	}

	envsJSON, err := json.Marshal(envs)
	if err != nil {
		r.logger.Error("failed to marshal envs for mcp server", "error", err, "name", name)
		return nil, err
	}

	_, err = r.db.ExecContext(ctx, insertMCPServerQuery, newID, name, string(commandJSON), string(envsJSON), createdAt)
	if err != nil {
		r.logger.Error("failed to insert mcp server", "error", err, "name", name)
		return nil, err
	}

	mcpServer := &model.MCPServer{
		ID:        newID,
		Name:      name,
		Command:   command,
		Envs:      envs,
		CreatedAt: createdAt,
	}

	return mcpServer, nil
}

const selectMCPServersQuery = `
SELECT id, name, command, envs, created_at FROM mcp_servers ORDER BY created_at DESC;
`

type dbMCPServer struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	Command   string `db:"command"` // JSON string from DB
	Envs      string `db:"envs"` // JSON string from DB
	CreatedAt string `db:"created_at"`
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
		var commandSlice []string
		if err := json.Unmarshal([]byte(dbServer.Command), &commandSlice); err != nil {
			r.logger.Error("failed to unmarshal command for mcp server", "error", err, "id", dbServer.ID)
			// Skip this server or return error? Skipping for now.
			continue
		}
		var envsSlice []string
		if dbServer.Envs != "" {
			if err := json.Unmarshal([]byte(dbServer.Envs), &envsSlice); err != nil {
				r.logger.Error("failed to unmarshal envs for mcp server", "error", err, "id", dbServer.ID)
				// Skip this server or return error? Skipping for now.
				continue
			}
		}
		servers = append(servers, &model.MCPServer{
			ID:        dbServer.ID,
			Name:      dbServer.Name,
			Command:   commandSlice,
			Envs:      envsSlice,
			CreatedAt: dbServer.CreatedAt,
		})
	}

	return servers, nil
}