package routing

import (
	"strings"
	"testing"
)

func TestLinkTree(t *testing.T) {
	tree := NewLinkTree()

	// Add some servers (simulating IRC LINKS response)
	tree.Add("hub.dal.net", "hub.dal.net", 0, "DALnet Hub")
	tree.Add("server1.dal.net", "hub.dal.net", 1, "Server 1")
	tree.Add("server2.dal.net", "hub.dal.net", 1, "Server 2")
	tree.Add("leaf1.dal.net", "server1.dal.net", 2, "Leaf 1")

	// Build the tree
	lines := tree.Build()

	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}

	// First line should be the root
	if !strings.Contains(lines[0], "hub.dal.net") {
		t.Errorf("First line should contain hub: %s", lines[0])
	}
}

func TestGetLinkedServers(t *testing.T) {
	tree := NewLinkTree()
	tree.Add("hub.dal.net", "hub.dal.net", 0, "DALnet Hub")
	tree.Add("server1.dal.net", "hub.dal.net", 1, "Server 1")

	servers := tree.GetLinkedServers()

	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Should return short names
	hasHub := false
	hasServer1 := false
	for _, s := range servers {
		if s == "hub" {
			hasHub = true
		}
		if s == "server1" {
			hasServer1 = true
		}
	}

	if !hasHub || !hasServer1 {
		t.Errorf("Missing expected servers: %v", servers)
	}
}

func TestCompareToMap(t *testing.T) {
	tree := NewLinkTree()
	tree.Add("hub.dal.net", "hub.dal.net", 0, "Hub")
	tree.Add("server1.dal.net", "hub.dal.net", 1, "Server 1")

	rmap := &Map{
		ServerList: []string{"hub", "server1", "server2", "server3"},
		Servers: map[string][]string{
			"hub":     {},
			"server1": {"hub"},
			"server2": {"hub"},
			"server3": {"hub"},
		},
	}

	total, linked, missing := CompareToMap(tree, rmap)

	if total != 4 {
		t.Errorf("Expected total 4, got %d", total)
	}
	if linked != 2 {
		t.Errorf("Expected linked 2, got %d", linked)
	}
	if len(missing) != 2 {
		t.Errorf("Expected 2 missing, got %d: %v", len(missing), missing)
	}
}
