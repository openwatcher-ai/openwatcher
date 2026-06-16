package installer

import (
	"testing"

	"openwatcher/desktop-app/internal/adb"
)

func TestParsePort(t *testing.T) {
	port, err := parsePort("40221")
	if err != nil || port != 40221 {
		t.Fatalf("parsePort = %d err=%v", port, err)
	}
	if _, err := parsePort("0"); err == nil {
		t.Fatalf("expected invalid port error")
	}
}

func TestResolveSelectedSerial(t *testing.T) {
	devices := []adb.Device{
		{Serial: "emulator-5554"},
		{Serial: "192.168.1.8:40221"},
	}

	if got, needsSelection := resolveSelectedSerial(devices, "192.168.1.8:40221"); got != "192.168.1.8:40221" || needsSelection {
		t.Fatalf("expected existing selection, got %q needsSelection=%v", got, needsSelection)
	}

	if got, needsSelection := resolveSelectedSerial(devices, "missing"); got != "" || !needsSelection {
		t.Fatalf("expected manual selection, got %q needsSelection=%v", got, needsSelection)
	}

	if got, needsSelection := resolveSelectedSerial(devices[:1], ""); got != "emulator-5554" || needsSelection {
		t.Fatalf("expected auto-selected single device, got %q needsSelection=%v", got, needsSelection)
	}
}

func TestInstallPolicyMessage(t *testing.T) {
	realDeviceStatus := Status{
		SelectedSerial: "192.168.1.8:40221",
		Devices: []adb.Device{
			{Serial: "192.168.1.8:40221", IsEmulator: false},
		},
		APK: APKInfo{Debug: true},
	}
	if got := installPolicyMessage(realDeviceStatus); got == "" {
		t.Fatalf("expected debug install block for real device")
	}

	emulatorStatus := Status{
		SelectedSerial: "emulator-5554",
		Devices: []adb.Device{
			{Serial: "emulator-5554", IsEmulator: true},
		},
		APK: APKInfo{Debug: true},
	}
	if got := installPolicyMessage(emulatorStatus); got != "" {
		t.Fatalf("unexpected emulator block: %q", got)
	}
}
