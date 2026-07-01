package main

import (
	"os"
	"path/filepath"
	"sync"
)

const defaultAgentDBPath = "data/database/tdx-agent.sqlite"

var agentDBWriteMu sync.Mutex

func agentDBPath() string {
	if path := os.Getenv("AGENT_DB_PATH"); path != "" {
		return path
	}
	return filepath.FromSlash(defaultAgentDBPath)
}

func agentFeatureDBPath(featureEnv string) string {
	if path := os.Getenv(featureEnv); path != "" {
		return path
	}
	return agentDBPath()
}
