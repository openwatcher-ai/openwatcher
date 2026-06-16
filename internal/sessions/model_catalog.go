package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultModelCatalogFilename = "model_catalog.local.json"

type modelCompactThresholds map[string]int64

type modelCatalogFile struct {
	Models json.RawMessage `json:"models"`
}

type modelCatalogEntry struct {
	ID                         string `json:"id"`
	Model                      string `json:"model"`
	Name                       string `json:"name"`
	Slug                       string `json:"slug"`
	AutoCompactTokenLimit      *int64 `json:"auto_compact_token_limit"`
	ModelAutoCompactTokenLimit *int64 `json:"model_auto_compact_token_limit"`
}

func loadModelCompactThresholds(codexHome string) modelCompactThresholds {
	path := resolveModelCatalogPath(codexHome)
	data, err := os.ReadFile(path)
	if err != nil {
		return modelCompactThresholds{}
	}

	entries := parseModelCatalogEntries(data)
	thresholds := modelCompactThresholds{}
	for _, entry := range entries {
		threshold := entry.compactThresholdTokens()
		if threshold <= 0 {
			continue
		}
		for _, name := range entry.identifiers() {
			thresholds[strings.ToLower(name)] = threshold
		}
	}
	return thresholds
}

func resolveModelCatalogPath(codexHome string) string {
	configPath := filepath.Join(codexHome, "config.toml")
	if data, err := os.ReadFile(configPath); err == nil {
		if configured, ok := parseModelCatalogPath(data); ok {
			return expandHomePath(configured)
		}
	}
	return filepath.Join(codexHome, defaultModelCatalogFilename)
}

func parseModelCatalogPath(data []byte) (string, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, "model_catalog_json") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok || strings.TrimSpace(key) != "model_catalog_json" {
			continue
		}
		parsed, ok := parseTomlString(strings.TrimSpace(stripInlineComment(value)))
		if ok && parsed != "" {
			return parsed, true
		}
	}
	return "", false
}

func stripInlineComment(value string) string {
	inQuote := rune(0)
	escaped := false
	for index, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inQuote == '"' {
			escaped = true
			continue
		}
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			inQuote = r
			continue
		}
		if r == '#' {
			return strings.TrimSpace(value[:index])
		}
	}
	return strings.TrimSpace(value)
}

func parseTomlString(value string) (string, bool) {
	if strings.HasPrefix(value, "\"") {
		parsed, err := strconv.Unquote(value)
		return parsed, err == nil
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
		return value[1 : len(value)-1], true
	}
	return "", false
}

func expandHomePath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func parseModelCatalogEntries(data []byte) []modelCatalogEntry {
	var catalog modelCatalogFile
	if err := json.Unmarshal(data, &catalog); err == nil && len(catalog.Models) > 0 {
		if entries := parseModelEntriesRaw(catalog.Models); len(entries) > 0 {
			return entries
		}
	}
	return parseModelEntriesRaw(data)
}

func parseModelEntriesRaw(data []byte) []modelCatalogEntry {
	var entries []modelCatalogEntry
	if err := json.Unmarshal(data, &entries); err == nil {
		return entries
	}

	var keyed map[string]modelCatalogEntry
	if err := json.Unmarshal(data, &keyed); err != nil {
		return nil
	}
	entries = make([]modelCatalogEntry, 0, len(keyed))
	for id, entry := range keyed {
		if entry.ID == "" {
			entry.ID = id
		}
		entries = append(entries, entry)
	}
	return entries
}

func (e modelCatalogEntry) compactThresholdTokens() int64 {
	if e.AutoCompactTokenLimit != nil && *e.AutoCompactTokenLimit > 0 {
		return *e.AutoCompactTokenLimit
	}
	if e.ModelAutoCompactTokenLimit != nil && *e.ModelAutoCompactTokenLimit > 0 {
		return *e.ModelAutoCompactTokenLimit
	}
	return 0
}

func (e modelCatalogEntry) identifiers() []string {
	values := []string{e.ID, e.Model, e.Name, e.Slug}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func (m modelCompactThresholds) thresholdForModel(model string) int64 {
	return m[strings.ToLower(strings.TrimSpace(model))]
}
