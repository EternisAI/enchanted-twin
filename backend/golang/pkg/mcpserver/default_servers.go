package mcpserver

import "github.com/EternisAI/enchanted-twin/graph/model"

// getDefaultMCPServers returns a map of default MCP server configurations, keyed by server type.
func getDefaultMCPServers() map[model.MCPServerType]*model.MCPServer {
	enabled := false
	servers := map[model.MCPServerType]*model.MCPServer{
		model.MCPServerTypeGoogle: {
			ID:      "google",
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
		model.MCPServerTypeEnchanted: {
			ID:      "enchanted",
			Name:    "Search & Image",
			Command: "npx",
			Args:    []string{},
			Enabled: enabled,
			Type:    model.MCPServerTypeEnchanted,
		},
		model.MCPServerTypeScreenpipe: {
			ID:      "screenpipe",
			Name:    "Screenpipe",
			Command: "npx",
			Args:    []string{},
			Enabled: enabled,
			Type:    model.MCPServerTypeScreenpipe,
		},
		model.MCPServerTypeSLACk: {
			ID:      "slack",
			Name:    "Slack",
			Command: "docker",
			Args: []string{
				"run",
				"-i",
				"--rm",
				"-e",
				"SLACK_BOT_TOKEN",
				"-e",
				"SLACK_TEAM_ID",
				"-e",
				"SLACK_CHANNEL_IDS",
				"mcp/slack",
			},
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
			ID:      "twitter",
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
		model.MCPServerTypeFreysa: {
			ID:      "freysa-video",
			Name:    "Freysa Video",
			Command: "url-http",
			Args:    []string{"https://freysa-video-mcp-production.up.railway.app/mcp"},
			Enabled: enabled,
			Type:    model.MCPServerTypeFreysa,
		},
	}
	return servers
}
