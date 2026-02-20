package storage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxEntries = 500

// LoadLogs reads routing logs from file
// Returns logs in reverse chronological order (newest first)
func LoadLogs(dataDir string) ([]string, error) {
	path := filepath.Join(dataDir, "logs.txt")
	lines, err := readLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	// Reverse so newest is first (file stores oldest first)
	return reverse(lines), nil
}

// SaveLogs writes routing logs to file
// Expects logs in reverse chronological order (newest first)
func SaveLogs(dataDir string, logs []string) error {
	path := filepath.Join(dataDir, "logs.txt")
	// Reverse back to oldest-first for file storage
	return writeLines(path, reverse(logs))
}

// LoadStats reads command stats from file
func LoadStats(dataDir string) ([]string, error) {
	path := filepath.Join(dataDir, "stats.txt")
	lines, err := readLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	return lines, nil
}

// SaveStats writes command stats to file (max 500 entries)
func SaveStats(dataDir string, stats []string) error {
	path := filepath.Join(dataDir, "stats.txt")
	// Trim to max entries (keep newest at end)
	if len(stats) > maxEntries {
		stats = stats[len(stats)-maxEntries:]
	}
	return writeLines(path, stats)
}

// MOTD represents a message of the day with its setter
type MOTD struct {
	Setter  string
	Message string
}

// LoadMOTD reads the message of the day from file
func LoadMOTD(dataDir string) (*MOTD, error) {
	path := filepath.Join(dataDir, "motd.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &MOTD{Setter: "", Message: ""}, nil
		}
		return nil, err
	}
	line := strings.TrimSpace(string(data))
	parts := strings.SplitN(line, "%%", 2)
	if len(parts) != 2 {
		return &MOTD{Setter: "", Message: line}, nil
	}
	return &MOTD{Setter: parts[0], Message: parts[1]}, nil
}

// SaveMOTD writes the message of the day to file
func SaveMOTD(dataDir string, motd *MOTD) error {
	path := filepath.Join(dataDir, "motd.txt")
	content := fmt.Sprintf("%s%%%%%s\n", motd.Setter, motd.Message)
	return os.WriteFile(path, []byte(content), 0644)
}

// AddLog prepends a new log entry (keeping newest first in memory)
func AddLog(logs []string, entry string) []string {
	logs = append([]string{entry}, logs...)
	if len(logs) > maxEntries {
		logs = logs[:maxEntries]
	}
	return logs
}

// AddStat appends a new stat entry
func AddStat(stats []string, entry string) []string {
	stats = append(stats, entry)
	if len(stats) > maxEntries {
		stats = stats[1:]
	}
	return stats
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func writeLines(path string, lines []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, line := range lines {
		if _, err := fmt.Fprintln(file, line); err != nil {
			return err
		}
	}
	return nil
}

func reverse(s []string) []string {
	result := make([]string, len(s))
	for i, v := range s {
		result[len(s)-1-i] = v
	}
	return result
}
