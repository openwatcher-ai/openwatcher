package releasehistory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	SummaryTypeLegacy = "legacy"
	SummaryTypeUser   = "user"
)

type Entry struct {
	BuiltAtUTC  time.Time
	VersionName string
	VersionCode int
	Commit      string
	Branch      string
	Artifact    string
	SHA256      string
	Summary     string
	SummaryType string
}

type ChangelogEntry struct {
	VersionName        string `json:"versionName"`
	VersionCode        int    `json:"versionCode"`
	PublishedAtUTC     string `json:"publishedAtUtc"`
	PublishedAtBeijing string `json:"publishedAtBeijing"`
	Summary            string `json:"summary"`
	SummaryType        string `json:"summaryType"`
}

func ParseMarkdown(r io.Reader) ([]Entry, error) {
	scanner := bufio.NewScanner(r)
	var entries []Entry
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := parseCells(line)
		if len(cells) == 0 {
			continue
		}
		if isHeaderRow(cells) || isDividerRow(cells) {
			continue
		}
		if len(cells) != 8 && len(cells) != 9 {
			return nil, fmt.Errorf("line %d: expected 8 or 9 columns, got %d", lineNo, len(cells))
		}
		builtAt, err := time.Parse(time.RFC3339, cells[0])
		if err != nil {
			return nil, fmt.Errorf("line %d: parse builtAt: %w", lineNo, err)
		}
		versionCode, err := strconv.Atoi(cells[2])
		if err != nil {
			return nil, fmt.Errorf("line %d: parse versionCode: %w", lineNo, err)
		}
		summaryType := SummaryTypeLegacy
		if len(cells) == 9 {
			summaryType = normalizeSummaryType(cells[7])
		}
		entries = append(entries, Entry{
			BuiltAtUTC:  builtAt.UTC(),
			VersionName: strings.TrimSpace(cells[1]),
			VersionCode: versionCode,
			Commit:      strings.TrimSpace(cells[3]),
			Branch:      strings.TrimSpace(cells[4]),
			Artifact:    strings.TrimSpace(cells[5]),
			SHA256:      strings.TrimSpace(cells[6]),
			Summary:     strings.TrimSpace(cells[len(cells)-1]),
			SummaryType: summaryType,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func BuildUserChangelog(entries []Entry) []ChangelogEntry {
	latestByVersion := make(map[int]Entry)
	for _, entry := range entries {
		if normalizeSummaryType(entry.SummaryType) != SummaryTypeUser {
			continue
		}
		if strings.TrimSpace(entry.Summary) == "" {
			continue
		}
		entry.SummaryType = SummaryTypeUser
		latestByVersion[entry.VersionCode] = entry
	}
	versionCodes := make([]int, 0, len(latestByVersion))
	for versionCode := range latestByVersion {
		versionCodes = append(versionCodes, versionCode)
	}
	slices.Sort(versionCodes)
	shanghai, _ := time.LoadLocation("Asia/Shanghai")
	items := make([]ChangelogEntry, 0, len(versionCodes))
	for _, versionCode := range versionCodes {
		entry := latestByVersion[versionCode]
		items = append(items, ChangelogEntry{
			VersionName:        entry.VersionName,
			VersionCode:        entry.VersionCode,
			PublishedAtUTC:     entry.BuiltAtUTC.Format(time.RFC3339),
			PublishedAtBeijing: entry.BuiltAtUTC.In(shanghai).Format("2006-01-02 15:04"),
			Summary:            entry.Summary,
			SummaryType:        SummaryTypeUser,
		})
	}
	return items
}

func EncodeUserChangelogJSON(entries []Entry) ([]byte, error) {
	payload := BuildUserChangelog(entries)
	return json.MarshalIndent(payload, "", "  ")
}

func normalizeSummaryType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SummaryTypeUser:
		return SummaryTypeUser
	default:
		return SummaryTypeLegacy
	}
}

func parseCells(line string) []string {
	parts := strings.Split(line, "|")
	if len(parts) < 3 {
		return nil
	}
	cells := make([]string, 0, len(parts)-2)
	for _, part := range parts[1 : len(parts)-1] {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func isHeaderRow(cells []string) bool {
	return len(cells) > 0 && cells[0] == "构建时间 UTC"
}

func isDividerRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cleaned := strings.ReplaceAll(strings.TrimSpace(cell), "-", "")
		if cleaned != "" {
			return false
		}
	}
	return true
}
