package pairing

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type BindingRecord struct {
	TokenHash  string `json:"tokenHash"`
	DeviceName string `json:"deviceName,omitempty"`
	PairedAt   string `json:"pairedAt,omitempty"`
	Source     string `json:"source,omitempty"`
}

func HistoryPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "pairing-history.json")
}

func ResolveRelativeToConfig(configPath, path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(filepath.Dir(configPath), trimmed)
}

func LoadHistory(path string) ([]BindingRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var records []BindingRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	filtered := make([]BindingRecord, 0, len(records))
	for _, record := range records {
		hash := normalizeTokenHash(record.TokenHash)
		if hash == "" {
			continue
		}
		record.TokenHash = hash
		filtered = append(filtered, record)
	}
	sortRecords(filtered)
	return filtered, nil
}

func RecordBinding(path string, record BindingRecord) error {
	hash := normalizeTokenHash(record.TokenHash)
	if hash == "" {
		return errors.New("token hash 不能为空")
	}
	record.TokenHash = hash
	records, err := LoadHistory(path)
	if err != nil {
		return err
	}
	replaced := false
	for i := range records {
		if records[i].TokenHash == hash {
			if strings.TrimSpace(record.DeviceName) != "" {
				records[i].DeviceName = strings.TrimSpace(record.DeviceName)
			}
			if strings.TrimSpace(record.PairedAt) != "" {
				records[i].PairedAt = strings.TrimSpace(record.PairedAt)
			}
			if strings.TrimSpace(record.Source) != "" {
				records[i].Source = strings.TrimSpace(record.Source)
			}
			replaced = true
			break
		}
	}
	if !replaced {
		records = append(records, BindingRecord{
			TokenHash:  hash,
			DeviceName: strings.TrimSpace(record.DeviceName),
			PairedAt:   strings.TrimSpace(record.PairedAt),
			Source:     strings.TrimSpace(record.Source),
		})
	}
	sortRecords(records)
	return writeRecords(path, records)
}

func AddAllowlistTokenHash(path, tokenHash string) error {
	hash := normalizeTokenHash(tokenHash)
	if hash == "" {
		return errors.New("token hash 非法")
	}
	entries, err := loadAllowlist(path)
	if err != nil {
		return err
	}
	if !slices.Contains(entries, hash) {
		entries = append(entries, hash)
	}
	slices.Sort(entries)
	return writeAllowlist(path, entries)
}

func LoadAllowlist(path string) ([]string, error) {
	return loadAllowlist(path)
}

func loadAllowlist(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	entries := make([]string, 0, len(lines))
	seen := map[string]struct{}{}
	for _, line := range lines {
		hash := normalizeTokenHash(line)
		if hash == "" {
			continue
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		entries = append(entries, hash)
	}
	slices.Sort(entries)
	return entries, nil
}

func writeAllowlist(path string, entries []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	content := strings.Join(entries, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func writeRecords(path string, records []BindingRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func sortRecords(records []BindingRecord) {
	slices.SortStableFunc(records, func(a, b BindingRecord) int {
		at := parseTime(a.PairedAt)
		bt := parseTime(b.PairedAt)
		switch {
		case at.After(bt):
			return -1
		case at.Before(bt):
			return 1
		default:
			return strings.Compare(a.TokenHash, b.TokenHash)
		}
	})
}

func parseTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func normalizeTokenHash(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if len(value) != 64 {
		return ""
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return ""
		}
	}
	return value
}
