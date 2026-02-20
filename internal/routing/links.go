package routing

import (
	"fmt"
	"sort"
	"strings"
)

// LinkEntry represents a single server link from LINKS response
type LinkEntry struct {
	Server      string // Server name
	Hub         string // Connected to (upstream)
	Hops        int    // Hop count
	Description string // Server description
}

// LinkTree holds the collected LINKS data
type LinkTree struct {
	entries map[string]*LinkEntry
	order   []string // Order of insertion
}

// NewLinkTree creates a new link tree collector
func NewLinkTree() *LinkTree {
	return &LinkTree{
		entries: make(map[string]*LinkEntry),
		order:   []string{},
	}
}

// Add adds a link entry from IRC numeric 364
func (t *LinkTree) Add(server, hub string, hops int, description string) {
	t.entries[server] = &LinkEntry{
		Server:      server,
		Hub:         hub,
		Hops:        hops,
		Description: description,
	}
	t.order = append(t.order, server)
}

// GetLinkedServers returns a list of all linked server names (short form)
func (t *LinkTree) GetLinkedServers() []string {
	var servers []string
	for server := range t.entries {
		// Extract short name (before first dot)
		short := server
		if idx := strings.Index(server, "."); idx > 0 {
			short = server[:idx]
		}
		servers = append(servers, short)
	}
	return servers
}

// Build constructs the sorted tree and returns formatted lines
func (t *LinkTree) Build() []string {
	if len(t.entries) == 0 {
		return []string{}
	}

	// Find the root (hops == 0)
	var root string
	for server, entry := range t.entries {
		if entry.Hops == 0 {
			root = server
			break
		}
	}

	if root == "" {
		return []string{"Error: no root server found"}
	}

	// Sort entries hierarchically
	ordered := t.sortHierarchically(root)

	// Track which levels still have more nodes
	var lines []string
	for i, server := range ordered {
		entry := t.entries[server]
		line := t.formatLine(entry, ordered[i+1:])
		lines = append(lines, line)
	}

	return lines
}

// sortHierarchically returns servers in tree order (depth-first)
func (t *LinkTree) sortHierarchically(root string) []string {
	var result []string
	result = append(result, root)
	t.sortChildren(root, &result)
	return result
}

func (t *LinkTree) sortChildren(parent string, result *[]string) {
	// Find all servers linked to this parent
	var children []*LinkEntry
	for _, entry := range t.entries {
		if entry.Hub == parent && entry.Server != parent {
			children = append(children, entry)
		}
	}

	// Sort children alphabetically
	sort.Slice(children, func(i, j int) bool {
		return children[i].Server < children[j].Server
	})

	// Recursively process each child
	for _, child := range children {
		*result = append(*result, child.Server)
		t.sortChildren(child.Server, result)
	}
}

func (t *LinkTree) formatLine(entry *LinkEntry, remaining []string) string {
	if entry.Hops == 0 {
		return fmt.Sprintf("%s (%d) %s", entry.Server, entry.Hops, entry.Description)
	}

	// Build prefix based on depth
	var prefix strings.Builder
	for level := 1; level < entry.Hops; level++ {
		// Check if there are more nodes at this level after us
		if t.hasMoreAtLevel(level+1, remaining) {
			prefix.WriteString("   |")
		} else {
			prefix.WriteString("    ")
		}
	}
	prefix.WriteString("|_ ")

	return fmt.Sprintf("%s%s (%d) %s", prefix.String(), entry.Server, entry.Hops, entry.Description)
}

func (t *LinkTree) hasMoreAtLevel(level int, remaining []string) bool {
	for _, server := range remaining {
		if entry, ok := t.entries[server]; ok {
			if entry.Hops == level {
				return true
			}
		}
	}
	return false
}

// CompareToMap compares linked servers against the routing map
// Returns (total in map, linked count, missing servers)
func CompareToMap(tree *LinkTree, rmap *Map) (int, int, []string) {
	linked := tree.GetLinkedServers()
	linkedSet := make(map[string]bool)
	for _, s := range linked {
		linkedSet[strings.ToLower(s)] = true
	}

	var missing []string
	for _, server := range rmap.ServerList {
		short := server
		if idx := strings.Index(server, "."); idx > 0 {
			short = server[:idx]
		}
		if !linkedSet[strings.ToLower(short)] {
			missing = append(missing, short)
		}
	}

	return len(rmap.ServerList), len(linked), missing
}
