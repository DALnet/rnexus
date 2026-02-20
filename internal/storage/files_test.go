package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogsRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rnexus-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some logs (newest first in memory)
	logs := []string{
		"[Thu Feb 20, 2025 12:00:00 GMT] [server1]: Connected",
		"[Thu Feb 20, 2025 11:00:00 GMT] [server2]: Disconnected",
	}

	// Save logs
	err = SaveLogs(tmpDir, logs)
	if err != nil {
		t.Fatalf("SaveLogs failed: %v", err)
	}

	// Load logs back
	loaded, err := LoadLogs(tmpDir)
	if err != nil {
		t.Fatalf("LoadLogs failed: %v", err)
	}

	if len(loaded) != len(logs) {
		t.Errorf("Expected %d logs, got %d", len(logs), len(loaded))
	}

	// Should be in same order (newest first)
	for i := range logs {
		if loaded[i] != logs[i] {
			t.Errorf("Log %d mismatch: expected %q, got %q", i, logs[i], loaded[i])
		}
	}
}

func TestAddLog(t *testing.T) {
	logs := []string{"old1", "old2"}
	logs = AddLog(logs, "new")

	if len(logs) != 3 {
		t.Errorf("Expected 3 logs, got %d", len(logs))
	}

	if logs[0] != "new" {
		t.Errorf("New log should be first, got %q", logs[0])
	}
}

func TestAddLogMaxEntries(t *testing.T) {
	// Create logs at max capacity
	logs := make([]string, 500)
	for i := range logs {
		logs[i] = "entry"
	}

	// Add one more
	logs = AddLog(logs, "new")

	if len(logs) != 500 {
		t.Errorf("Expected 500 logs (max), got %d", len(logs))
	}

	if logs[0] != "new" {
		t.Errorf("New log should be first")
	}
}

func TestMOTDRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rnexus-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	motd := &MOTD{
		Setter:  "testuser on Thu Feb 20, 2025",
		Message: "This is the message of the day",
	}

	// Save MOTD
	err = SaveMOTD(tmpDir, motd)
	if err != nil {
		t.Fatalf("SaveMOTD failed: %v", err)
	}

	// Verify file format
	data, _ := os.ReadFile(filepath.Join(tmpDir, "motd.txt"))
	expected := "testuser on Thu Feb 20, 2025%%This is the message of the day\n"
	if string(data) != expected {
		t.Errorf("MOTD file format wrong: got %q", string(data))
	}

	// Load MOTD back
	loaded, err := LoadMOTD(tmpDir)
	if err != nil {
		t.Fatalf("LoadMOTD failed: %v", err)
	}

	if loaded.Setter != motd.Setter {
		t.Errorf("Setter mismatch: expected %q, got %q", motd.Setter, loaded.Setter)
	}
	if loaded.Message != motd.Message {
		t.Errorf("Message mismatch: expected %q, got %q", motd.Message, loaded.Message)
	}
}

func TestLoadMOTDMissing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rnexus-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Load from empty directory
	motd, err := LoadMOTD(tmpDir)
	if err != nil {
		t.Fatalf("LoadMOTD should not fail for missing file: %v", err)
	}

	if motd.Setter != "" || motd.Message != "" {
		t.Errorf("Expected empty MOTD, got setter=%q message=%q", motd.Setter, motd.Message)
	}
}
