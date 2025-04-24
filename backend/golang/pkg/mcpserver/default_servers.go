package mcpserver

import "github.com/EternisAI/enchanted-twin/graph/model"

// getDefaultMCPServers returns a map of default MCP server configurations, keyed by server type.
func getDefaultMCPServers() map[model.MCPServerType]*model.MCPServer {
	enabled := false
	servers := map[model.MCPServerType]*model.MCPServer{
		model.MCPServerTypeGoogle: {
			Name:    "Google",
			Command: "npx",
			Args:    []string{"@shinzolabs/gmail-mcp"},
			Envs: []*model.KeyValue{
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
			Type:    model.MCPServerTypeGoogle,
		},
		model.MCPServerTypeSLACk: {
			Name:    "Slack",
			Command: "docker",
			Args:    []string{"run", "-i", "--rm", "-e", "SLACK_BOT_TOKEN", "-e", "SLACK_TEAM_ID", "-e", "SLACK_CHANNEL_IDS", "mcp/slack"},
			Envs: []*model.KeyValue{
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
			Type:    model.MCPServerTypeSLACk,
		},
		model.MCPServerTypeTwitter: {
			Name:    "Twitter",
			Command: "npx",
			Args:    []string{"-y", "@0xparashar/twitter-mcp"},
			Envs: []*model.KeyValue{
				{
					Key:   "REFRESH_TOKEN",
					Value: "1234567890",
				},
				{
					Key:   "CLIENT_ID",
					Value: "1234567890",
				},
				{
					Key:   "CLIENT_SECRET",
					Value: "1234567890",
				},
			},
			Enabled: enabled,
			Type:    model.MCPServerTypeTwitter,
		},
	}
	return servers
}
