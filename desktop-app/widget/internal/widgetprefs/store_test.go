package widgetprefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreRoundTripIsPrivateAndAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs", "position.json")
	s := NewFileStore(path)
	want := Position{MonitorID: "screen-a", Edge: "right", Normalized: .4}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	if got := s.Load(); got != want {
		t.Fatalf("%+v", got)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("%v %v", info, err)
	}
}
