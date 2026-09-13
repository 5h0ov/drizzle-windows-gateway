package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type StoredSlot struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Dialect string `json:"dialect"`
}

func formatDialect(d string) string {
	switch strings.ToLower(d) {
	case "postgresql", "postgres", "pg":
		return "PostgreSQL"
	case "mysql":
		return "MySQL"
	case "sqlite", "sqlite3":
		return "SQLite"
	case "libsql", "turso":
		return "LibSQL"
	case "d1", "cloudflare-d1":
		return "Cloudflare D1"
	default:
		if len(d) > 0 {
			return strings.ToUpper(d[:1]) + strings.ToLower(d[1:])
		}
		return "Database"
	}
}

func getStoredConnections(storeDir string) []StoredSlot {
	filePath := filepath.Join(storeDir, "store.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var raw struct {
		Slots [][]json.RawMessage `json:"slots"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var list []StoredSlot
	for _, pair := range raw.Slots {
		if len(pair) >= 2 {
			var slot StoredSlot
			if err := json.Unmarshal(pair[1], &slot); err == nil && slot.Name != "" {
				list = append(list, slot)
			}
		}
	}
	return list
}
