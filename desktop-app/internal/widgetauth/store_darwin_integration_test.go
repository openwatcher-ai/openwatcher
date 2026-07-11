//go:build darwin && cgo

package widgetauth

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestDarwinSystemStoreRoundTripWhenEnabled(t *testing.T) {
	if os.Getenv("OPENWATCHER_TEST_SYSTEM_CREDENTIAL") != "1" {
		t.Skip("set OPENWATCHER_TEST_SYSTEM_CREDENTIAL=1 to exercise a temporary Keychain item")
	}
	service := fmt.Sprintf("ai.openwatcher.widget.test.%d", time.Now().UnixNano())
	store := newSystemStore(service, "roundtrip")
	t.Cleanup(func() {
		if err := store.Delete(); err != nil {
			t.Errorf("cleanup Keychain test credential: %v", err)
		}
	})
	token, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Write(token); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read()
	if err != nil || got != token {
		t.Fatalf("Keychain roundtrip = %q, %v", got, err)
	}
	if err := store.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read after delete err = %v", err)
	}
}
