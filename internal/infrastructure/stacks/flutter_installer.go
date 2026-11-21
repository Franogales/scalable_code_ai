package stacks

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/Franogales/scalable_code_ai/internal/domain"
	"gopkg.in/yaml.v3"
)

const pubspecFileName = "pubspec.yaml"

var flutterRequiredDevDeps = []string{
	"custom_lint",
	"dart_code_linter",
	"lakos",
	"mutation_test",
}

// FlutterInstaller sets up scalable-code tooling in a Flutter project.
type FlutterInstaller struct {
	fs     domain.FileSystem
	runner domain.CommandRunner
}

// NewFlutterInstaller creates a FlutterInstaller.
func NewFlutterInstaller(fs domain.FileSystem, runner domain.CommandRunner) *FlutterInstaller {
	return &FlutterInstaller{
		fs:     fs,
		runner: runner,
	}
}

// Key returns the dart/flutter StackKey.
func (f *FlutterInstaller) Key() domain.StackKey {
	return domain.StackKey{
		Language:  domain.LanguageDart,
		Framework: domain.FrameworkFlutter,
	}
}

// ValidateProject ensures pubspec.yaml exists and belongs to a Flutter project.
func (f *FlutterInstaller) ValidateProject(_ context.Context, targetDir string) error {
	pubspecPath := filepath.Join(targetDir, pubspecFileName)
	if !f.fs.Exists(pubspecPath) {
		return fmt.Errorf("%s not found in %s", pubspecFileName, targetDir)
	}

	raw, err := f.fs.ReadFile(pubspecPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", pubspecFileName, err)
	}

	var parsed struct {
		Name         string         `yaml:"name"`
		Dependencies map[string]any `yaml:"dependencies"`
	}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("parse %s: %w", pubspecFileName, err)
	}
	if _, hasFlutter := parsed.Dependencies["flutter"]; !hasFlutter {
		return fmt.Errorf("%s does not declare a 'flutter' dependency", pubspecFileName)
	}
	return nil
}

// InstallDependencies adds custom_lint, dart_code_linter, lakos, and mutation_test to dev_dependencies.
func (f *FlutterInstaller) InstallDependencies(ctx context.Context, targetDir string) ([]string, []string, error) {
	pubspecPath := filepath.Join(targetDir, pubspecFileName)
	raw, err := f.fs.ReadFile(pubspecPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", pubspecFileName, err)
	}

	missing, err := findMissingPackages(raw, flutterRequiredDevDeps)
	if err != nil {
		return nil, nil, err
	}
	if len(missing) == 0 {
		return nil, nil, nil
	}

	if _, err := f.runner.LookPath("flutter"); err == nil {
		args := append([]string{"pub", "add", "--dev"}, missing...)
		if _, runErr := f.runner.Run(ctx, targetDir, "flutter", args...); runErr == nil {
			return missing, nil, nil
		} else if containsPackage(missing, "custom_lint") {
			// Fallback for modern analyzer >= 9 projects (e.g. riverpod_generator >= 4) where legacy custom_lint conflicts.
			coreMissing := filterOutPackage(missing, "custom_lint")
			var warnings []string
			if len(coreMissing) > 0 {
				coreArgs := append([]string{"pub", "add", "--dev"}, coreMissing...)
				if _, coreErr := f.runner.Run(ctx, targetDir, "flutter", coreArgs...); coreErr != nil {
					return nil, nil, fmt.Errorf("install flutter dev_dependencies: %w", coreErr)
				}
			}
			warnings = append(warnings,
				"Se omitió 'custom_lint' porque entra en conflicto con las versiones modernas de 'analyzer' en este proyecto (ej. riverpod_generator >= 4). Se instalaron dart_code_linter, lakos y mutation_test; para lints nativos usa plugins nativos en analysis_options.yaml (ej. riverpod_lint).",
			)
			return coreMissing, warnings, nil
		} else {
			return nil, nil, fmt.Errorf("install flutter dev_dependencies: %w", runErr)
		}
	}

	// Fallback when flutter CLI is unavailable in PATH: edit pubspec.yaml directly.
	updated, err := addDevDependenciesToPubspec(raw, missing)
	if err != nil {
		return nil, nil, fmt.Errorf("fallback update %s: %w", pubspecFileName, err)
	}
	if err := f.fs.WriteFile(pubspecPath, updated, 0o644); err != nil {
		return nil, nil, fmt.Errorf("write %s: %w", pubspecFileName, err)
	}

	warnings := []string{
		"'flutter' command was not found in PATH; added dev_dependencies directly to pubspec.yaml. Run 'flutter pub get' manually.",
	}
	return missing, warnings, nil
}

// InstallScripts fetches and writes the Flutter validation script(s) into the target project.
func (f *FlutterInstaller) InstallScripts(ctx context.Context, targetDir string, scripts []domain.ScriptEntry, source domain.AssetSource) ([]string, error) {
	var installed []string
	for _, script := range scripts {
		content, err := source.FetchFile(ctx, script.Source)
		if err != nil {
			return installed, fmt.Errorf("fetch script %s: %w", script.Source, err)
		}
		destPath := filepath.Join(targetDir, filepath.FromSlash(script.Dest))
		if err := f.fs.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return installed, fmt.Errorf("create dir for %s: %w", destPath, err)
		}
		if err := f.fs.WriteFile(destPath, content, 0o644); err != nil {
			return installed, fmt.Errorf("write script %s: %w", destPath, err)
		}
		installed = append(installed, script.Dest)
	}
	return installed, nil
}

// GitIgnoreEntries returns report paths that should not be committed.
func (f *FlutterInstaller) GitIgnoreEntries() []string {
	return []string{
		".scalable-code/report.json",
		".scalable-code/report.md",
		".scalable-code/mutation-report/",
		".scalable-code/mutation-config.xml",
	}
}

// NextSteps returns human-readable instructions after installation completes.
func (f *FlutterInstaller) NextSteps() []string {
	return []string{
		"Abre este proyecto en Antigravity / Gemini y ejecuta el comando /scalable-code-setup para detectar tu arquitectura y generar analysis_options.yaml + .scalable-code/config.json.",
	}
}

func findMissingPackages(rawPubspec []byte, required []string) ([]string, error) {
	var parsed struct {
		Dependencies    map[string]any `yaml:"dependencies"`
		DevDependencies map[string]any `yaml:"dev_dependencies"`
	}
	if err := yaml.Unmarshal(rawPubspec, &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", pubspecFileName, err)
	}

	var missing []string
	for _, pkg := range required {
		_, inDeps := parsed.Dependencies[pkg]
		_, inDevDeps := parsed.DevDependencies[pkg]
		if !inDeps && !inDevDeps {
			missing = append(missing, pkg)
		}
	}
	return missing, nil
}

func addDevDependenciesToPubspec(rawPubspec []byte, packages []string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(rawPubspec, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s root must be a YAML mapping", pubspecFileName)
	}

	root := doc.Content[0]
	devDepsNode := findMappingValue(root, "dev_dependencies")
	if devDepsNode == nil {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "dev_dependencies"}
		devDepsNode = &yaml.Node{Kind: yaml.MappingNode}
		root.Content = append(root.Content, keyNode, devDepsNode)
	} else if devDepsNode.Kind != yaml.MappingNode {
		devDepsNode.Kind = yaml.MappingNode
		devDepsNode.Tag = "!!map"
		devDepsNode.Value = ""
		devDepsNode.Content = nil
	}

	for _, pkg := range packages {
		if findMappingValue(devDepsNode, pkg) != nil {
			continue
		}
		devDepsNode.Content = append(
			devDepsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: pkg},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "any"},
		)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func findMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func containsPackage(packages []string, target string) bool {
	for _, pkg := range packages {
		if pkg == target {
			return true
		}
	}
	return false
}

func filterOutPackage(packages []string, excluded string) []string {
	var filtered []string
	for _, pkg := range packages {
		if pkg != excluded {
			filtered = append(filtered, pkg)
		}
	}
	return filtered
}
