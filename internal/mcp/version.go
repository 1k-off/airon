package mcp

// ClientVersion is the version string advertised to MCP servers.
// Overridden at link time from cmd/airon via -ldflags when building releases.
var ClientVersion = "dev"
