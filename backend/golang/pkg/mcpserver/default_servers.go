package mcpserver

import "github.com/EternisAI/enchanted-twin/graph/model"

// getDefaultMCPServers returns a slice of default MCP server configurations.
func getDefaultMCPServers() []*model.MCPServer {
	enabled := false
	return []*model.MCPServer{
		{
			Name:    "GMail",
			Command: "npx",
			Args:    []string{"@shinzolabs/gmail-mcp"},
			Envs:    []*model.KeyValue{
				{
					Key:   "CLIENT_ID",
					Value: "1234567890",
				},
				{
					Key:   "CLIENT_SECRET",
					Value: "1234567890",
				},
				{
					Key:   "REFRESH_TOKEN",
					Value: "1234567890",
				},
			},
			Enabled: enabled,
		},
		{
			Name:    "Google Drive",
			Command: "npx",
			Args:    []string{"-y","@isaacphi/mcp-gdrive"},
			Envs:    []*model.KeyValue{
				{
					Key:   "CLIENT_ID",
					Value: "1234567890",
				},
				{
					Key:   "CLIENT_SECRET",
					Value: "1234567890",	
				},
				{
					Key:   "GDRIVE_CREDS_DIR",
					Value: "/tmp/gdrive_creds",
				},
			},
			Enabled: enabled,
		},
		{
			Name:    "Slack",
			Command: "docker",
			Args:    []string{"run", "-i", "--rm", "-e", "SLACK_BOT_TOKEN", "-e", "SLACK_TEAM_ID", "-e", "SLACK_CHANNEL_IDS", "mcp/slack"},
			Envs:    []*model.KeyValue{
				{
					Key:   "SLACK_BOT_TOKEN",
					Value: "xoxb-1234567890",
				},
				{
					Key:   "SLACK_TEAM_ID",
					Value: "T00000000",
				},
			},
			Enabled: enabled,
		},
	}
}
