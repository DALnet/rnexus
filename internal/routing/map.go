package routing

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// skipPatterns matches lines to ignore when parsing the routing map
var skipPatterns = regexp.MustCompile(`(?i)^Tier|^Hub:|^Client:|^Special:|^LOA|===|DALnet Routing|^Temporary|^---|^\s*$`)

// Map represents the routing configuration
type Map struct {
	// Raw holds all lines from the rmap file
	Raw []string
	// Servers maps server name to its hub assignments
	Servers map[string][]string
	// ServerList is an ordered list of server names
	ServerList []string
}

// LoadMap reads and parses the routing map file
func LoadMap(dataDir string) (*Map, error) {
	path := filepath.Join(dataDir, "rmap.txt")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Map{
				Raw:        []string{},
				Servers:    make(map[string][]string),
				ServerList: []string{},
			}, nil
		}
		return nil, err
	}
	defer file.Close()

	m := &Map{
		Raw:     []string{},
		Servers: make(map[string][]string),
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Clean carriage returns
		line = strings.ReplaceAll(line, "\r", "")
		m.Raw = append(m.Raw, line)

		// Skip non-server lines
		if skipPatterns.MatchString(line) {
			continue
		}

		// Parse server: hub1 hub2 hub3 format
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			server := strings.TrimSpace(parts[0])
			if server == "" {
				continue
			}

			var hubs []string
			if len(parts) > 1 {
				hubPart := strings.TrimSpace(parts[1])
				for _, h := range strings.Fields(hubPart) {
					// Skip non-hub entries (comments, etc)
					if !strings.HasPrefix(h, "(") && !strings.HasPrefix(h, "=") {
						hubs = append(hubs, h)
					}
				}
			}
			m.Servers[server] = hubs
			m.ServerList = append(m.ServerList, server)
		}
	}

	return m, scanner.Err()
}

// GetUplinks returns the hub assignments for a server
func (m *Map) GetUplinks(server string) []string {
	// Try exact match first
	if hubs, ok := m.Servers[server]; ok {
		return hubs
	}
	// Try prefix match (server name without domain)
	server = strings.ToLower(server)
	for name, hubs := range m.Servers {
		if strings.HasPrefix(strings.ToLower(name), server) {
			return hubs
		}
	}
	return nil
}

// FindServer searches for servers matching a prefix
func (m *Map) FindServer(prefix string) []string {
	prefix = strings.ToLower(prefix)
	var matches []string
	for _, name := range m.ServerList {
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			matches = append(matches, name)
		}
	}
	return matches
}
