package source

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/Franogales/scalable_code_ai/internal/domain"
)

// LocalSource loads registry files from a directory on the local filesystem.
type LocalSource struct {
	rootDir string
	fs      domain.FileSystem
}

// NewLocalSource constructs an AssetSource backed by rootDir.
func NewLocalSource(rootDir string, fs domain.FileSystem) *LocalSource {
	return &LocalSource{
		rootDir: rootDir,
		fs:      fs,
	}
}

// FetchManifest reads and decodes manifest.json from rootDir.
func (s *LocalSource) FetchManifest(ctx context.Context) (domain.Manifest, error) {
	data, err := s.FetchFile(ctx, manifestRelPath)
	if err != nil {
		return domain.Manifest{}, fmt.Errorf("read local manifest: %w", err)
	}
	var manifest domain.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return domain.Manifest{}, fmt.Errorf("decode local manifest.json: %w", err)
	}
	return manifest, nil
}

// FetchFile reads a relative file from rootDir.
func (s *LocalSource) FetchFile(_ context.Context, relativePath string) ([]byte, error) {
	cleanPath, err := sanitizeRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(s.rootDir, filepath.FromSlash(cleanPath))
	data, err := s.fs.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read local asset %s: %w", fullPath, err)
	}
	return data, nil
}
