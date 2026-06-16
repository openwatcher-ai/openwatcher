package runtime

import (
	"context"
)

func (m *Manager) fetchDesktopReleaseNotes(ctx context.Context, notesURL string) ([]ChangelogNoteItem, error) {
	payload, err := m.fetchJSON(ctx, notesURL, "changelog")
	if err != nil {
		return nil, err
	}
	if entry, err := ParseChangelogEntry(payload); err == nil {
		return entry.DesktopNotes(), nil
	}
	feed, err := ParseChangelogFeed(payload)
	if err != nil {
		return nil, err
	}
	notes := make([]ChangelogNoteItem, 0)
	for _, entry := range feed.FilterDesktopEntries() {
		notes = append(notes, entry.DesktopNotes()...)
	}
	return notes, nil
}
