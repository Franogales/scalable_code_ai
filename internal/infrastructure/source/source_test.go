package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Franogales/scalable_code_ai/internal/infrastructure/fs"
)

func TestGitHubSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			_, _ = w.Write([]byte(`{"version":"1.0.0","stacks":{"dart/flutter":{"scripts":[{"source":"s.dart","dest":"tool/s.dart"}],"skills":[]}}}`))
		case "/s.dart":
			_, _ = w.Write([]byte("void main() {}"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	src := NewGitHubSourceWithBaseURL(srv.URL, srv.Client())
	ctx := context.Background()

	manifest, err := src.FetchManifest(ctx)
	if err != nil {
		t.Fatalf("FetchManifest failed: %v", err)
	}
	if manifest.Version != "1.0.0" {
		t.Fatalf("unexpected version %q", manifest.Version)
	}
	if _, ok := manifest.Stacks["dart/flutter"]; !ok {
		t.Fatalf("expected dart/flutter stack in manifest")
	}

	content, err := src.FetchFile(ctx, "s.dart")
	if err != nil {
		t.Fatalf("FetchFile failed: %v", err)
	}
	if string(content) != "void main() {}" {
		t.Fatalf("unexpected file content: %q", string(content))
	}

	if _, err := src.FetchFile(ctx, "../secret"); err == nil {
		t.Fatalf("expected error for path traversal")
	}
	if _, err := src.FetchFile(ctx, "missing.txt"); err == nil {
		t.Fatalf("expected error for 404")
	}
}

func TestLocalSource(t *testing.T) {
	dir := t.TempDir()
	manifestJSON := `{"version":"1.0.0","stacks":{"dart/flutter":{"scripts":[],"skills":[]}}}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	src := NewLocalSource(dir, fs.NewOSFileSystem())
	manifest, err := src.FetchManifest(context.Background())
	if err != nil {
		t.Fatalf("FetchManifest failed: %v", err)
	}
	if manifest.Version != "1.0.0" {
		t.Fatalf("unexpected version %q", manifest.Version)
	}
	if _, err := src.FetchFile(context.Background(), "../escape"); err == nil {
		t.Fatalf("expected error for path traversal")
	}
}
