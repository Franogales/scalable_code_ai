package stacks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Franogales/scalable_code_ai/internal/domain"
	"github.com/Franogales/scalable_code_ai/internal/infrastructure/fs"
	"gopkg.in/yaml.v3"
)

type fakeRunner struct {
	hasFlutter bool
	calls      [][]string
	runErr     error
	runFn      func(call []string) error
}

func (r *fakeRunner) LookPath(file string) (string, error) {
	if file == "flutter" && r.hasFlutter {
		return "/usr/bin/flutter", nil
	}
	return "", errors.New("not found")
}

func (r *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	if r.runFn != nil {
		return []byte("ok"), r.runFn(call)
	}
	return []byte("ok"), r.runErr
}

func TestFlutterInstaller_ValidateProject(t *testing.T) {
	osfs := fs.NewOSFileSystem()
	inst := NewFlutterInstaller(osfs, &fakeRunner{})

	t.Run("fails when pubspec.yaml is missing", func(t *testing.T) {
		dir := t.TempDir()
		if err := inst.ValidateProject(context.Background(), dir); err == nil {
			t.Fatalf("expected error for missing pubspec.yaml")
		}
	})

	t.Run("fails when pubspec.yaml has no flutter dependency", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: pure_dart\ndependencies:\n  path: ^1.9.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := inst.ValidateProject(context.Background(), dir); err == nil {
			t.Fatalf("expected error for non-flutter pubspec.yaml")
		}
	})

	t.Run("passes for valid flutter pubspec.yaml", func(t *testing.T) {
		dir := t.TempDir()
		content := "name: my_app\ndependencies:\n  flutter:\n    sdk: flutter\n"
		if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := inst.ValidateProject(context.Background(), dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestFlutterInstaller_InstallDependencies(t *testing.T) {
	osfs := fs.NewOSFileSystem()

	t.Run("runs flutter pub add --dev only for missing packages", func(t *testing.T) {
		dir := t.TempDir()
		pubspec := "name: my_app\ndependencies:\n  flutter:\n    sdk: flutter\ndev_dependencies:\n  lakos: ^2.0.7\n"
		if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte(pubspec), 0o644); err != nil {
			t.Fatal(err)
		}

		runner := &fakeRunner{hasFlutter: true}
		inst := NewFlutterInstaller(osfs, runner)

		installed, warnings, err := inst.InstallDependencies(context.Background(), dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(warnings) != 0 {
			t.Fatalf("unexpected warnings: %v", warnings)
		}
		wantInstalled := []string{"custom_lint", "dart_code_linter", "mutation_test"}
		if !reflect.DeepEqual(installed, wantInstalled) {
			t.Fatalf("got installed=%v, want %v", installed, wantInstalled)
		}
		wantCall := []string{"flutter", "pub", "add", "--dev", "custom_lint", "dart_code_linter", "mutation_test"}
		if len(runner.calls) != 1 || !reflect.DeepEqual(runner.calls[0], wantCall) {
			t.Fatalf("got runner calls=%v, want [%v]", runner.calls, wantCall)
		}
	})

	t.Run("does nothing when all dependencies already exist", func(t *testing.T) {
		dir := t.TempDir()
		pubspec := "name: my_app\ndependencies:\n  flutter:\n    sdk: flutter\ndev_dependencies:\n  custom_lint: ^0.7.0\n  dart_code_linter: ^2.0.0\n  lakos: ^2.0.7\n  mutation_test: ^1.6.0\n"
		if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte(pubspec), 0o644); err != nil {
			t.Fatal(err)
		}

		runner := &fakeRunner{hasFlutter: true}
		inst := NewFlutterInstaller(osfs, runner)
		installed, _, err := inst.InstallDependencies(context.Background(), dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(installed) != 0 || len(runner.calls) != 0 {
			t.Fatalf("expected no installs or calls, got installed=%v calls=%v", installed, runner.calls)
		}
	})

	t.Run("falls back to direct yaml update when flutter is missing from PATH", func(t *testing.T) {
		dir := t.TempDir()
		pubspec := "# App manifest\nname: my_app\ndependencies:\n  flutter:\n    sdk: flutter\n"
		if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte(pubspec), 0o644); err != nil {
			t.Fatal(err)
		}

		runner := &fakeRunner{hasFlutter: false}
		inst := NewFlutterInstaller(osfs, runner)

		installed, warnings, err := inst.InstallDependencies(context.Background(), dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(installed) != 4 {
			t.Fatalf("expected 4 installed packages, got %v", installed)
		}
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning about manual pub get, got %v", warnings)
		}

		updatedRaw, err := os.ReadFile(filepath.Join(dir, "pubspec.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			DevDependencies map[string]any `yaml:"dev_dependencies"`
		}
		if err := yaml.Unmarshal(updatedRaw, &parsed); err != nil {
			t.Fatalf("updated pubspec.yaml is invalid YAML: %v", err)
		}
		for _, pkg := range []string{"custom_lint", "dart_code_linter", "lakos", "mutation_test"} {
			if _, ok := parsed.DevDependencies[pkg]; !ok {
				t.Fatalf("missing %s in updated pubspec.yaml:\n%s", pkg, string(updatedRaw))
			}
		}
	})

	t.Run("retries without custom_lint when custom_lint causes a version conflict", func(t *testing.T) {
		dir := t.TempDir()
		pubspec := "name: my_app\ndependencies:\n  flutter:\n    sdk: flutter\n"
		if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte(pubspec), 0o644); err != nil {
			t.Fatal(err)
		}

		runner := &fakeRunner{
			hasFlutter: true,
			runFn: func(call []string) error {
				for _, arg := range call {
					if arg == "custom_lint" {
						return errors.New("version solving failed for custom_lint")
					}
				}
				return nil
			},
		}
		inst := NewFlutterInstaller(osfs, runner)

		installed, warnings, err := inst.InstallDependencies(context.Background(), dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantInstalled := []string{"dart_code_linter", "lakos", "mutation_test"}
		if !reflect.DeepEqual(installed, wantInstalled) {
			t.Fatalf("got installed=%v, want %v", installed, wantInstalled)
		}
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning about custom_lint skipped, got %v", warnings)
		}
		if len(runner.calls) != 2 {
			t.Fatalf("expected 2 flutter pub add attempts, got %d (%v)", len(runner.calls), runner.calls)
		}
	})
}

type fakeAssetSource struct {
	files map[string][]byte
}

func (f *fakeAssetSource) FetchManifest(_ context.Context) (domain.Manifest, error) {
	return domain.Manifest{}, nil
}

func (f *fakeAssetSource) FetchFile(_ context.Context, rel string) ([]byte, error) {
	data, ok := f.files[rel]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

func TestFlutterInstaller_InstallScripts(t *testing.T) {
	dir := t.TempDir()
	inst := NewFlutterInstaller(fs.NewOSFileSystem(), &fakeRunner{})
	src := &fakeAssetSource{
		files: map[string][]byte{
			"dart/flutter/scripts/scalable_code_check.dart": []byte("void main() {}"),
		},
	}

	scripts := []domain.ScriptEntry{
		{Source: "dart/flutter/scripts/scalable_code_check.dart", Dest: "tool/scalable_code_check.dart"},
	}

	installed, err := inst.InstallScripts(context.Background(), dir, scripts, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(installed) != 1 || installed[0] != "tool/scalable_code_check.dart" {
		t.Fatalf("unexpected installed scripts: %v", installed)
	}

	got, err := os.ReadFile(filepath.Join(dir, "tool", "scalable_code_check.dart"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "void main() {}" {
		t.Fatalf("unexpected script content: %q", string(got))
	}
}
