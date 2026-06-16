package sessions

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"openwatcher/internal/config"

	_ "modernc.org/sqlite"
)

var ErrThreadNotFound = errors.New("thread not found")

func resolveStateDBPath(codexHome string) (string, error) {
	candidates, err := stateDBPathCandidates(codexHome)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("state sqlite not found under %s", codexHome)
	}

	for _, candidate := range candidates {
		if stateDBPathUsable(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("usable state sqlite not found under %s", codexHome)
}

func stateDBPathCandidates(codexHome string) ([]string, error) {
	preferredPaths := []string{
		filepath.Join(codexHome, "sqlite", "state_5.sqlite"),
		filepath.Join(codexHome, "state_5.sqlite"),
	}

	seen := make(map[string]bool, len(preferredPaths))
	var candidates []string
	for _, path := range preferredPaths {
		if appendStateDBCandidate(&candidates, seen, path) {
			continue
		}
		if _, err := os.Stat(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	globPatterns := []string{
		filepath.Join(codexHome, "sqlite", "state_*.sqlite"),
		filepath.Join(codexHome, "state_*.sqlite"),
	}
	var matches []string
	for _, pattern := range globPatterns {
		globMatches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		matches = append(matches, globMatches...)
	}
	sort.Slice(matches, func(i, j int) bool {
		leftInfo, leftErr := os.Stat(matches[i])
		rightInfo, rightErr := os.Stat(matches[j])
		switch {
		case leftErr != nil && rightErr != nil:
			return matches[i] > matches[j]
		case leftErr != nil:
			return false
		case rightErr != nil:
			return true
		case !leftInfo.ModTime().Equal(rightInfo.ModTime()):
			return leftInfo.ModTime().After(rightInfo.ModTime())
		default:
			return matches[i] > matches[j]
		}
	})
	for _, path := range matches {
		appendStateDBCandidate(&candidates, seen, path)
	}
	return candidates, nil
}

func appendStateDBCandidate(candidates *[]string, seen map[string]bool, path string) bool {
	cleanPath := filepath.Clean(path)
	if seen[cleanPath] {
		return false
	}
	info, err := os.Stat(cleanPath)
	if err != nil || info.IsDir() {
		return false
	}
	seen[cleanPath] = true
	*candidates = append(*candidates, cleanPath)
	return true
}

func stateDBPathUsable(path string) bool {
	db, err := openStateDB(path)
	if err != nil {
		return false
	}
	defer db.Close()

	hasThreads, err := hasTable(db, "threads")
	return err == nil && hasThreads
}

func (s *Scanner) RolloutPathForThread(threadID string) (string, error) {
	codexHome, err := config.ResolveCodexHome(s.CodexHome)
	if err != nil {
		return "", err
	}

	statePath, err := resolveStateDBPath(codexHome)
	if err != nil {
		return "", err
	}

	db, err := openStateDB(statePath)
	if err != nil {
		return "", err
	}
	defer db.Close()

	return loadRolloutPathForThread(db, threadID)
}

func openStateDB(path string) (*sql.DB, error) {
	dsn := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite", dsn.String()+"?mode=ro")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA query_only = 1;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func loadRolloutPathForThread(db *sql.DB, threadID string) (string, error) {
	var rolloutPath string
	err := db.QueryRow(`
SELECT rollout_path
FROM threads
WHERE id = ?
  AND archived = 0
  AND rollout_path != ''
LIMIT 1;
`, threadID).Scan(&rolloutPath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrThreadNotFound
	}
	if err != nil {
		return "", err
	}
	return filepath.Clean(rolloutPath), nil
}

func hasTable(db *sql.DB, name string) (bool, error) {
	var exists int
	err := db.QueryRow(`
SELECT 1
FROM sqlite_master
WHERE type = 'table'
  AND name = ?
LIMIT 1;
`, name).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func loadActiveThreads(db *sql.DB, limit int) ([]threadRow, error) {
	query := `
SELECT
    id,
    title,
    COALESCE(model, ''),
    COALESCE(reasoning_effort, ''),
    tokens_used,
    COALESCE(updated_at_ms, updated_at * 1000),
    rollout_path
FROM threads
WHERE archived = 0
  AND rollout_path != ''
ORDER BY COALESCE(updated_at_ms, updated_at * 1000) DESC
LIMIT ?;
`

	hasSpawnEdgesTable, err := hasTable(db, "thread_spawn_edges")
	if err != nil {
		return nil, err
	}
	if hasSpawnEdgesTable {
		query = `
SELECT
    id,
    title,
    COALESCE(model, ''),
    COALESCE(reasoning_effort, ''),
    tokens_used,
    COALESCE(updated_at_ms, updated_at * 1000),
    rollout_path
FROM threads
WHERE archived = 0
  AND rollout_path != ''
  AND NOT EXISTS (
      SELECT 1
      FROM thread_spawn_edges
      WHERE child_thread_id = threads.id
  )
ORDER BY COALESCE(updated_at_ms, updated_at * 1000) DESC
LIMIT ?;
`
	}

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []threadRow
	for rows.Next() {
		var (
			row         threadRow
			updatedAtMs int64
		)
		if err := rows.Scan(
			&row.ThreadID,
			&row.Title,
			&row.Model,
			&row.ReasoningEffort,
			&row.TokensUsedTotal,
			&updatedAtMs,
			&row.RolloutPath,
		); err != nil {
			return nil, err
		}
		row.UpdatedAt = time.UnixMilli(updatedAtMs)
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadRecentRollouts(db *sql.DB, cutoff time.Time) ([]rolloutEntry, error) {
	rows, err := db.Query(`
SELECT
    id,
    COALESCE(model, ''),
    rollout_path
FROM threads
WHERE archived = 0
  AND rollout_path != ''
  AND COALESCE(updated_at_ms, updated_at * 1000) >= ?
ORDER BY COALESCE(updated_at_ms, updated_at * 1000) DESC;
`, cutoff.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]bool{}
	var result []rolloutEntry
	for rows.Next() {
		var entry rolloutEntry
		if err := rows.Scan(&entry.ThreadID, &entry.Model, &entry.RolloutPath); err != nil {
			return nil, err
		}
		entry.RolloutPath = filepath.Clean(entry.RolloutPath)
		if entry.RolloutPath == "" || seen[entry.RolloutPath] {
			continue
		}
		seen[entry.RolloutPath] = true
		result = append(result, entry)
	}
	return result, rows.Err()
}
