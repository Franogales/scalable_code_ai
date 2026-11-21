package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Franogales/scalable_code_ai/internal/domain"
)

type stubPrompter struct {
	lang       domain.Language
	usesFW     bool
	fw         domain.Framework
	aiProvider domain.AIProvider
}

func (p *stubPrompter) SelectLanguage(_ []domain.Option) (domain.Language, error) {
	return p.lang, nil
}
func (p *stubPrompter) ConfirmFramework() (bool, error) { return p.usesFW, nil }
func (p *stubPrompter) SelectFramework(_ []domain.Option) (domain.Framework, error) {
	return p.fw, nil
}
func (p *stubPrompter) SelectAIProvider(_ []domain.Option) (domain.AIProvider, error) {
	return p.aiProvider, nil
}

type stubSource struct {
	manifest domain.Manifest
	err      error
}

func (s *stubSource) FetchManifest(_ context.Context) (domain.Manifest, error) {
	return s.manifest, s.err
}
func (s *stubSource) FetchFile(_ context.Context, _ string) ([]byte, error) {
	return []byte("content"), nil
}

type memFS struct {
	files map[string][]byte
}

func newMemFS() *memFS { return &memFS{files: make(map[string][]byte)} }
func (m *memFS) ReadFile(path string) ([]byte, error) {
	d, ok := m.files[filepath.ToSlash(path)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return d, nil
}
func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[filepath.ToSlash(path)] = data
	return nil
}
func (m *memFS) MkdirAll(_ string, _ os.FileMode) error { return nil }
func (m *memFS) Exists(path string) bool {
	_, ok := m.files[filepath.ToSlash(path)]
	return ok
}

type stubStackInstaller struct {
	key         domain.StackKey
	validateErr error
	deps        []string
	scripts     []string
}

func (s *stubStackInstaller) Key() domain.StackKey { return s.key }
func (s *stubStackInstaller) ValidateProject(_ context.Context, _ string) error {
	return s.validateErr
}
func (s *stubStackInstaller) InstallDependencies(_ context.Context, _ string) ([]string, []string, error) {
	return s.deps, nil, nil
}
func (s *stubStackInstaller) InstallScripts(_ context.Context, _ string, _ []domain.ScriptEntry, _ domain.AssetSource) ([]string, error) {
	return s.scripts, nil
}
func (s *stubStackInstaller) GitIgnoreEntries() []string {
	return []string{".scalable-code/report.json", ".scalable-code/report.md"}
}
func (s *stubStackInstaller) NextSteps() []string {
	return []string{"Run /scalable-code-setup"}
}

type stubAIInstaller struct {
	provider domain.AIProvider
	skills   []string
}

func (a *stubAIInstaller) Provider() domain.AIProvider { return a.provider }
func (a *stubAIInstaller) InstallSkills(_ context.Context, _ string, _ []domain.SkillEntry, _ domain.AssetSource) ([]string, error) {
	return a.skills, nil
}
func (a *stubAIInstaller) UpdateInstructions(_ context.Context, _ string, _ domain.Language, _ domain.Framework) (string, error) {
	return "AGENTS.md", nil
}

type mapStackResolver map[domain.StackKey]domain.StackInstaller

func (r mapStackResolver) Get(k domain.StackKey) (domain.StackInstaller, error) {
	v, ok := r[k]
	if !ok {
		return nil, errors.New("stack not supported")
	}
	return v, nil
}

type mapAIResolver map[domain.AIProvider]domain.AIInstaller

func (r mapAIResolver) Get(p domain.AIProvider) (domain.AIInstaller, error) {
	v, ok := r[p]
	if !ok {
		return nil, errors.New("ai not supported")
	}
	return v, nil
}

func TestInstallUseCase_Execute(t *testing.T) {
	key := domain.StackKey{Language: domain.LanguageDart, Framework: domain.FrameworkFlutter}
	fs := newMemFS()
	uc := NewInstallUseCase(
		&stubPrompter{
			lang:       domain.LanguageDart,
			usesFW:     true,
			fw:         domain.FrameworkFlutter,
			aiProvider: domain.AIProviderGemini,
		},
		&stubSource{
			manifest: domain.Manifest{
				Version: "1.0.0",
				Stacks: map[string]domain.StackAssets{
					"dart/flutter": {
						Scripts: []domain.ScriptEntry{{Source: "s.dart", Dest: "tool/scalable_code_check.dart"}},
						Skills:  []domain.SkillEntry{{Name: "scalable-code-setup"}},
					},
				},
			},
		},
		mapStackResolver{
			key: &stubStackInstaller{
				key:     key,
				deps:    []string{"custom_lint", "dart_code_linter", "lakos"},
				scripts: []string{"tool/scalable_code_check.dart"},
			},
		},
		mapAIResolver{
			domain.AIProviderGemini: &stubAIInstaller{
				provider: domain.AIProviderGemini,
				skills:   []string{".agents/skills/scalable-code-setup"},
			},
		},
		fs,
	)

	req, err := uc.GatherRequest("/proj")
	if err != nil {
		t.Fatalf("GatherRequest failed: %v", err)
	}

	report, err := uc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(report.InstalledDependencies) != 3 {
		t.Fatalf("expected 3 dependencies, got %v", report.InstalledDependencies)
	}

	gi, err := fs.ReadFile("/proj/.gitignore")
	if err != nil {
		t.Fatalf("expected .gitignore to be created: %v", err)
	}
	if !strings.Contains(string(gi), ".scalable-code/report.json") {
		t.Fatalf("expected .gitignore to contain report entry, got %q", string(gi))
	}

	// Second execution should not duplicate .gitignore entries.
	report2, err := uc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}
	for _, f := range report2.UpdatedFiles {
		if f == ".gitignore" {
			t.Fatalf("expected .gitignore not to be marked updated on second run")
		}
	}
}

func TestInstallUseCase_Errors(t *testing.T) {
	key := domain.StackKey{Language: domain.LanguageDart, Framework: domain.FrameworkFlutter}

	t.Run("returns error when framework is skipped and pure dart has no installer", func(t *testing.T) {
		uc := NewInstallUseCase(
			&stubPrompter{lang: domain.LanguageDart, usesFW: false, aiProvider: domain.AIProviderGemini},
			&stubSource{},
			mapStackResolver{key: &stubStackInstaller{key: key}},
			mapAIResolver{domain.AIProviderGemini: &stubAIInstaller{provider: domain.AIProviderGemini}},
			newMemFS(),
		)
		req, err := uc.GatherRequest("/proj")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := uc.Execute(context.Background(), req); err == nil {
			t.Fatalf("expected error for unsupported dart/none stack")
		}
	})

	t.Run("returns error when project validation fails", func(t *testing.T) {
		uc := NewInstallUseCase(
			&stubPrompter{},
			&stubSource{},
			mapStackResolver{key: &stubStackInstaller{key: key, validateErr: errors.New("missing pubspec.yaml")}},
			mapAIResolver{domain.AIProviderGemini: &stubAIInstaller{provider: domain.AIProviderGemini}},
			newMemFS(),
		)
		req := domain.InstallRequest{
			TargetDir:     "/proj",
			Language:      domain.LanguageDart,
			UsesFramework: true,
			Framework:     domain.FrameworkFlutter,
			AI:            domain.AIProviderGemini,
		}
		if _, err := uc.Execute(context.Background(), req); err == nil {
			t.Fatalf("expected validation error")
		}
	})
}
