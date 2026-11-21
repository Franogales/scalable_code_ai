package fs

import (
	"os"
	"strings"
)

// OSFileSystem implements domain.FileSystem backed by the local operating system.
type OSFileSystem struct{}

// NewOSFileSystem creates a new OSFileSystem.
func NewOSFileSystem() *OSFileSystem {
	return &OSFileSystem{}
}

// ReadFile reads the entire file at path.
func (f *OSFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes data to path with the specified permissions.
func (f *OSFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// MkdirAll creates directory path and any necessary parents.
func (f *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Exists reports whether the path exists on disk.
func (f *OSFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// UpsertMarkedBlock inserts or replaces a section bounded by startMarker and endMarker.
// If the markers do not exist, the block is appended cleanly to the existing content.
func UpsertMarkedBlock(existing, startMarker, endMarker, body string) string {
	wrapped := startMarker + "\n" + strings.TrimSpace(body) + "\n" + endMarker

	normalized := strings.ReplaceAll(existing, "\r\n", "\n")
	startIdx := strings.Index(normalized, startMarker)
	endIdx := strings.Index(normalized, endMarker)

	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		before := strings.TrimRight(normalized[:startIdx], "\n")
		after := strings.Trim(normalized[endIdx+len(endMarker):], "\n")

		var parts []string
		if before != "" {
			parts = append(parts, before)
		}
		parts = append(parts, wrapped)
		if after != "" {
			parts = append(parts, after)
		}
		return strings.Join(parts, "\n\n") + "\n"
	}

	trimmed := strings.TrimRight(normalized, "\n")
	if trimmed == "" {
		return wrapped + "\n"
	}
	return trimmed + "\n\n" + wrapped + "\n"
}

// AppendMissingLines appends any entries not already present in existing content.
func AppendMissingLines(existing string, entries []string) (updated string, changed bool) {
	normalized := strings.ReplaceAll(existing, "\r\n", "\n")
	present := make(map[string]struct{})
	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			present[trimmed] = struct{}{}
		}
	}

	var missing []string
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if _, ok := present[trimmed]; !ok {
			present[trimmed] = struct{}{}
			missing = append(missing, trimmed)
		}
	}

	if len(missing) == 0 {
		return existing, false
	}

	base := strings.TrimRight(normalized, "\n")
	addition := strings.Join(missing, "\n")
	if base == "" {
		return addition + "\n", true
	}
	return base + "\n" + addition + "\n", true
}
