package ai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Franogales/scalable_code_ai/internal/domain"
	"github.com/Franogales/scalable_code_ai/internal/infrastructure/fs"
)

type fakeSource struct {
	files map[string][]byte
}

func (f *fakeSource) FetchManifest(_ context.Context) (domain.Manifest, error) {
	return domain.Manifest{}, nil
}

func (f *fakeSource) FetchFile(_ context.Context, rel string) ([]byte, error) {
	data, ok := f.files[rel]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

func TestGeminiInstaller_InstallSkills(t *testing.T) {
	dir := t.TempDir()
	inst := NewGeminiInstaller(fs.NewOSFileSystem())
	src := &fakeSource{
		files: map[string][]byte{
			"dart/flutter/skills/scalable-code-setup/SKILL.md": []byte("# Setup"),
			"dart/flutter/skills/scalable-code-check/SKILL.md": []byte("# Check"),
		},
	}

	skills := []domain.SkillEntry{
		{
			Name:  "scalable-code-setup",
			Files: []domain.SkillFile{{Source: "dart/flutter/skills/scalable-code-setup/SKILL.md", Dest: "SKILL.md"}},
		},
		{
			Name:  "scalable-code-check",
			Files: []domain.SkillFile{{Source: "dart/flutter/skills/scalable-code-check/SKILL.md", Dest: "SKILL.md"}},
		},
	}

	installed, err := inst.InstallSkills(context.Background(), dir, skills, src)
	if err != nil {
		t.Fatalf("InstallSkills failed: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("expected 2 skills installed, got %v", installed)
	}

	setupData, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "scalable-code-setup", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(setupData) != "# Setup" {
		t.Fatalf("unexpected setup skill content: %q", string(setupData))
	}
}

func TestGeminiInstaller_UpdateInstructions(t *testing.T) {
	t.Run("creates AGENTS.md when absent and stays idempotent", func(t *testing.T) {
		dir := t.TempDir()
		inst := NewGeminiInstaller(fs.NewOSFileSystem())

		updatedFile, err := inst.UpdateInstructions(context.Background(), dir, domain.LanguageDart, domain.FrameworkFlutter)
		if err != nil {
			t.Fatalf("UpdateInstructions failed: %v", err)
		}
		if updatedFile != "AGENTS.md" {
			t.Fatalf("got %q, want AGENTS.md", updatedFile)
		}

		first, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(first), "scalable-code-setup") || !strings.Contains(string(first), "scalable-code-check") {
			t.Fatalf("missing skill instructions in AGENTS.md:\n%s", string(first))
		}

		// Second call must produce identical content.
		if _, err := inst.UpdateInstructions(context.Background(), dir, domain.LanguageDart, domain.FrameworkFlutter); err != nil {
			t.Fatal(err)
		}
		second, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Fatalf("UpdateInstructions is not idempotent")
		}
	})

	t.Run("preserves existing AGENTS.md content", func(t *testing.T) {
		dir := t.TempDir()
		agentsPath := filepath.Join(dir, "AGENTS.md")
		if err := os.WriteFile(agentsPath, []byte("# Existing Guidelines\n\nFollow Dart style.\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		inst := NewGeminiInstaller(fs.NewOSFileSystem())
		if _, err := inst.UpdateInstructions(context.Background(), dir, domain.LanguageDart, domain.FrameworkFlutter); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(agentsPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "# Existing Guidelines") {
			t.Fatalf("existing content was lost:\n%s", string(got))
		}
	})
}
