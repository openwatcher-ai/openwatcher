package runtime

import (
	"encoding/json"
	"fmt"
	"strings"
)

const ChangelogSchemaVersion = 1

type ChangelogFeed struct {
	SchemaVersion int              `json:"schemaVersion"`
	Channel       string           `json:"channel"`
	UpdatedAt     string           `json:"updatedAt"`
	Entries       []ChangelogEntry `json:"entries"`
}

type ChangelogEntry struct {
	ID          string              `json:"id"`
	PublishedAt string              `json:"publishedAt"`
	Scope       []string            `json:"scope"`
	Components  ChangelogComponents `json:"components"`
	Notes       ChangelogNotes      `json:"notes"`
}

type ChangelogComponents struct {
	Desktop       ChangelogComponentState `json:"desktop"`
	Watch         ChangelogComponentState `json:"watch"`
	Runtime       ChangelogComponentState `json:"runtime"`
	Compatibility ChangelogComponentState `json:"compatibility"`
	Docs          ChangelogComponentState `json:"docs"`
}

type ChangelogComponentState struct {
	Status string `json:"status"`
}

type ChangelogNotes struct {
	Features      []ChangelogNoteItem `json:"features"`
	Improvements  []ChangelogNoteItem `json:"improvements"`
	Fixes         []ChangelogNoteItem `json:"fixes"`
	Compatibility []ChangelogNoteItem `json:"compatibility"`
}

type ChangelogNoteItem struct {
	Component string `json:"component"`
	Text      string `json:"text"`
}

func ParseChangelogFeed(payload []byte) (ChangelogFeed, error) {
	var feed ChangelogFeed
	if err := json.Unmarshal(payload, &feed); err != nil {
		return ChangelogFeed{}, fmt.Errorf("解析 changelog 失败：%w", err)
	}
	if err := feed.Validate(); err != nil {
		return ChangelogFeed{}, err
	}
	return feed, nil
}

func ParseChangelogEntry(payload []byte) (ChangelogEntry, error) {
	var entry ChangelogEntry
	if err := json.Unmarshal(payload, &entry); err != nil {
		return ChangelogEntry{}, fmt.Errorf("解析 changelog entry 失败：%w", err)
	}
	if err := entry.Validate(); err != nil {
		return ChangelogEntry{}, err
	}
	return entry, nil
}

func (f ChangelogFeed) Validate() error {
	if f.SchemaVersion != ChangelogSchemaVersion {
		return fmt.Errorf("不支持的 changelog schemaVersion：%d", f.SchemaVersion)
	}
	if strings.TrimSpace(f.Channel) == "" {
		return fmt.Errorf("changelog 缺少 channel")
	}
	for _, entry := range f.Entries {
		if err := entry.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (e ChangelogEntry) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("changelog entry 缺少 id")
	}
	if strings.TrimSpace(e.PublishedAt) == "" {
		return fmt.Errorf("changelog entry %s 缺少 publishedAt", e.ID)
	}
	return nil
}

func (f ChangelogFeed) FilterDesktopEntries() []ChangelogEntry {
	filtered := make([]ChangelogEntry, 0, len(f.Entries))
	for _, entry := range f.Entries {
		if entry.ForDesktop() {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (e ChangelogEntry) ForDesktop() bool {
	return e.Components.Desktop.Status == "updated" ||
		e.Components.Runtime.Status == "updated" ||
		e.Components.Compatibility.Status == "updated" ||
		e.Components.Docs.Status == "updated"
}

func (e ChangelogEntry) Summary() string {
	for _, group := range [][]ChangelogNoteItem{
		e.Notes.Fixes,
		e.Notes.Improvements,
		e.Notes.Features,
		e.Notes.Compatibility,
	} {
		for _, item := range group {
			if text := strings.TrimSpace(item.Text); text != "" {
				return text
			}
		}
	}
	return ""
}

func (e ChangelogEntry) DesktopNotes() []ChangelogNoteItem {
	notes := make([]ChangelogNoteItem, 0)
	for _, group := range [][]ChangelogNoteItem{
		e.Notes.Features,
		e.Notes.Improvements,
		e.Notes.Fixes,
		e.Notes.Compatibility,
	} {
		for _, item := range group {
			if desktopNoteItem(item) {
				notes = append(notes, item)
			}
		}
	}
	return notes
}

func desktopNoteItem(item ChangelogNoteItem) bool {
	component := strings.TrimSpace(item.Component)
	return component == "桌面应用" ||
		component == "运行时依赖" ||
		component == "兼容性" ||
		component == "文档"
}
