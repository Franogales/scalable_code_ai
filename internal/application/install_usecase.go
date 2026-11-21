package application

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Franogales/scalable_code_ai/internal/domain"
)

// StackResolver resolves a StackInstaller by StackKey.
type StackResolver interface {
	Get(key domain.StackKey) (domain.StackInstaller, error)
}

// AIResolver resolves an AIInstaller by AIProvider.
type AIResolver interface {
	Get(provider domain.AIProvider) (domain.AIInstaller, error)
}

// InstallUseCase orchestrates the interactive installation of tooling, scripts, skills, and rules.
type InstallUseCase struct {
	prompter domain.Prompter
	source   domain.AssetSource
	stacks   StackResolver
	ais      AIResolver
	fs       domain.FileSystem
}

// NewInstallUseCase constructs an InstallUseCase with its required ports.
func NewInstallUseCase(
	prompter domain.Prompter,
	source domain.AssetSource,
	stacks StackResolver,
	ais AIResolver,
	fs domain.FileSystem,
) *InstallUseCase {
	return &InstallUseCase{
		prompter: prompter,
		source:   source,
		stacks:   stacks,
		ais:      ais,
		fs:       fs,
	}
}

// GatherRequest runs the interactive wizard to build an InstallRequest.
func (uc *InstallUseCase) GatherRequest(targetDir string) (domain.InstallRequest, error) {
	lang, err := uc.prompter.SelectLanguage(domain.AvailableLanguages())
	if err != nil {
		return domain.InstallRequest{}, fmt.Errorf("select language: %w", err)
	}

	usesFramework, err := uc.prompter.ConfirmFramework()
	if err != nil {
		return domain.InstallRequest{}, fmt.Errorf("confirm framework: %w", err)
	}

	fw := domain.FrameworkNone
	if usesFramework {
		frameworks := domain.AvailableFrameworks(lang)
		if len(frameworks) == 0 {
			return domain.InstallRequest{}, fmt.Errorf("no frameworks available yet for %s", lang)
		}
		fw, err = uc.prompter.SelectFramework(frameworks)
		if err != nil {
			return domain.InstallRequest{}, fmt.Errorf("select framework: %w", err)
		}
	}

	aiProvider, err := uc.prompter.SelectAIProvider(domain.AvailableAIProviders())
	if err != nil {
		return domain.InstallRequest{}, fmt.Errorf("select AI provider: %w", err)
	}

	return domain.InstallRequest{
		TargetDir:     targetDir,
		Language:      lang,
		UsesFramework: usesFramework,
		Framework:     fw,
		AI:            aiProvider,
	}, nil
}

// Execute validates the target project and installs dependencies, scripts, skills, and agent instructions.
func (uc *InstallUseCase) Execute(ctx context.Context, req domain.InstallRequest) (domain.InstallReport, error) {
	stackKey := req.StackKey()
	stackInstaller, err := uc.stacks.Get(stackKey)
	if err != nil {
		return domain.InstallReport{}, err
	}

	aiInstaller, err := uc.ais.Get(req.AI)
	if err != nil {
		return domain.InstallReport{}, err
	}

	if err := stackInstaller.ValidateProject(ctx, req.TargetDir); err != nil {
		return domain.InstallReport{}, fmt.Errorf("validate project: %w", err)
	}

	manifest, err := uc.source.FetchManifest(ctx)
	if err != nil {
		return domain.InstallReport{}, fmt.Errorf("fetch registry manifest: %w", err)
	}

	assets, ok := manifest.Stacks[stackKey.String()]
	if !ok {
		return domain.InstallReport{}, fmt.Errorf("manifest does not contain assets for stack %q", stackKey.String())
	}

	deps, warnings, err := stackInstaller.InstallDependencies(ctx, req.TargetDir)
	if err != nil {
		return domain.InstallReport{}, fmt.Errorf("install dependencies: %w", err)
	}

	scripts, err := stackInstaller.InstallScripts(ctx, req.TargetDir, assets.Scripts, uc.source)
	if err != nil {
		return domain.InstallReport{}, fmt.Errorf("install scripts: %w", err)
	}

	skills, err := aiInstaller.InstallSkills(ctx, req.TargetDir, assets.Skills, uc.source)
	if err != nil {
		return domain.InstallReport{}, fmt.Errorf("install skills: %w", err)
	}

	var updatedFiles []string
	instructionsFile, err := aiInstaller.UpdateInstructions(ctx, req.TargetDir, req.Language, req.Framework)
	if err != nil {
		return domain.InstallReport{}, fmt.Errorf("update AI instructions: %w", err)
	}
	if instructionsFile != "" {
		updatedFiles = append(updatedFiles, instructionsFile)
	}

	gitIgnoreChanged, err := uc.updateGitIgnore(req.TargetDir, stackInstaller.GitIgnoreEntries())
	if err != nil {
		return domain.InstallReport{}, fmt.Errorf("update .gitignore: %w", err)
	}
	if gitIgnoreChanged {
		updatedFiles = append(updatedFiles, ".gitignore")
	}

	return domain.InstallReport{
		TargetDir:             req.TargetDir,
		Stack:                 stackKey,
		AI:                    req.AI,
		InstalledDependencies: deps,
		InstalledScripts:      scripts,
		InstalledSkills:       skills,
		UpdatedFiles:          updatedFiles,
		Warnings:              warnings,
		NextSteps:             stackInstaller.NextSteps(),
	}, nil
}

func (uc *InstallUseCase) updateGitIgnore(targetDir string, entries []string) (bool, error) {
	if len(entries) == 0 {
		return false, nil
	}

	gitIgnorePath := filepath.Join(targetDir, ".gitignore")
	var existing string
	if uc.fs.Exists(gitIgnorePath) {
		raw, err := uc.fs.ReadFile(gitIgnorePath)
		if err != nil {
			return false, err
		}
		existing = string(raw)
	}

	updated, changed := appendMissingLines(existing, entries)
	if !changed {
		return false, nil
	}
	if err := uc.fs.WriteFile(gitIgnorePath, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func appendMissingLines(existing string, entries []string) (string, bool) {
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
