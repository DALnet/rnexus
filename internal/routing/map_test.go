package routing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMap(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rnexus-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test rmap.txt
	rmapContent := `DALnet Routing Team Map
===========================
Tier 1 Hubs

Hub: services

server1: hub1 hub2 hub3
server2: hub1
server3: hub2 hub3

===========================
Special Servers
LOA servers go here

Temporary assignments
`
	err = os.WriteFile(filepath.Join(tmpDir, "rmap.txt"), []byte(rmapContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Load the map
	m, err := LoadMap(tmpDir)
	if err != nil {
		t.Fatalf("LoadMap failed: %v", err)
	}

	// Verify servers were parsed
	if len(m.Servers) != 3 {
		t.Errorf("Expected 3 servers, got %d", len(m.Servers))
	}

	// Verify server1 hubs
	hubs := m.Servers["server1"]
	if len(hubs) != 3 {
		t.Errorf("Expected 3 hubs for server1, got %d", len(hubs))
	}
	if hubs[0] != "hub1" || hubs[1] != "hub2" || hubs[2] != "hub3" {
		t.Errorf("Unexpected hubs for server1: %v", hubs)
	}

	// Verify server2 has one hub
	hubs = m.Servers["server2"]
	if len(hubs) != 1 || hubs[0] != "hub1" {
		t.Errorf("Unexpected hubs for server2: %v", hubs)
	}
}

func TestLoadMapMissing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rnexus-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Load from empty directory (no rmap.txt)
	m, err := LoadMap(tmpDir)
	if err != nil {
		t.Fatalf("LoadMap should not fail for missing file: %v", err)
	}

	if len(m.Servers) != 0 {
		t.Errorf("Expected empty server map, got %d entries", len(m.Servers))
	}
}

func TestGetUplinks(t *testing.T) {
	m := &Map{
		Servers: map[string][]string{
			"testserver": {"hub1", "hub2"},
			"other":      {"hub3"},
		},
	}

	// Exact match
	hubs := m.GetUplinks("testserver")
	if len(hubs) != 2 {
		t.Errorf("Expected 2 hubs, got %d", len(hubs))
	}

	// Prefix match (case insensitive)
	hubs = m.GetUplinks("TEST")
	if len(hubs) != 2 {
		t.Errorf("Expected 2 hubs for prefix match, got %d", len(hubs))
	}

	// No match
	hubs = m.GetUplinks("nonexistent")
	if hubs != nil {
		t.Errorf("Expected nil for nonexistent server, got %v", hubs)
	}
}
