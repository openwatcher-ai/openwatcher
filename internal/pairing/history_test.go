package pairing

import (
	"path/filepath"
	"testing"
)

func TestRecordBindingStoresNewestRecordFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pairing-history.json")

	if err := RecordBinding(path, BindingRecord{
		TokenHash:  HashToken("token-a-0123456789abcdef0123456789"),
		DeviceName: "Watch A",
		PairedAt:   "2026-06-09T10:00:00Z",
		Source:     "pair-page",
	}); err != nil {
		t.Fatalf("RecordBinding() error = %v", err)
	}
	if err := RecordBinding(path, BindingRecord{
		TokenHash:  HashToken("token-b-0123456789abcdef0123456789"),
		DeviceName: "Watch B",
		PairedAt:   "2026-06-09T11:00:00Z",
		Source:     "desktop-bootstrap",
	}); err != nil {
		t.Fatalf("RecordBinding() second error = %v", err)
	}

	records, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("LoadHistory() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("history len = %d", len(records))
	}
	if records[0].DeviceName != "Watch B" || records[1].DeviceName != "Watch A" {
		t.Fatalf("history order = %#v", records)
	}
}

func TestAddAllowlistTokenHashDeduplicatesEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowlist.txt")
	hash := HashToken("token-a-0123456789abcdef0123456789")

	if err := AddAllowlistTokenHash(path, hash); err != nil {
		t.Fatalf("AddAllowlistTokenHash() error = %v", err)
	}
	if err := AddAllowlistTokenHash(path, hash); err != nil {
		t.Fatalf("AddAllowlistTokenHash() duplicate error = %v", err)
	}

	entries, err := LoadAllowlist(path)
	if err != nil {
		t.Fatalf("LoadAllowlist() error = %v", err)
	}
	if len(entries) != 1 || entries[0] != hash {
		t.Fatalf("allowlist entries = %#v", entries)
	}
}
