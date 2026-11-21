package domain

import (
	"context"
	"os"
)

// Prompter gathers interactive user selections for the installation wizard.
type Prompter interface {
	SelectLanguage(options []Option) (Language, error)
	ConfirmFramework() (bool, error)
	SelectFramework(options []Option) (Framework, error)
	SelectAIProvider(options []Option) (AIProvider, error)
}

// AssetSource retrieves the manifest and raw asset files from a registry (GitHub or local).
type AssetSource interface {
	FetchManifest(ctx context.Context) (Manifest, error)
	FetchFile(ctx context.Context, relativePath string) ([]byte, error)
}

// StackInstaller encapsulates language/framework-specific setup logic.
type StackInstaller interface {
	Key() StackKey
	ValidateProject(ctx context.Context, targetDir string) error
	InstallDependencies(ctx context.Context, targetDir string) (installed []string, warnings []string, err error)
	InstallScripts(ctx context.Context, targetDir string, scripts []ScriptEntry, source AssetSource) ([]string, error)
	GitIgnoreEntries() []string
	NextSteps() []string
}

// AIInstaller encapsulates AI-platform-specific skill and rule installation.
type AIInstaller interface {
	Provider() AIProvider
	InstallSkills(ctx context.Context, targetDir string, skills []SkillEntry, source AssetSource) ([]string, error)
	UpdateInstructions(ctx context.Context, targetDir string, lang Language, fw Framework) (string, error)
}

// FileSystem abstracts filesystem operations for testability.
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Exists(path string) bool
}

// CommandRunner abstracts external CLI execution.
type CommandRunner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}
