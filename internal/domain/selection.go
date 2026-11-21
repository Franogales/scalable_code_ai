package domain

import "fmt"

// Language represents a supported programming language.
type Language string

const (
	LanguageDart Language = "dart"
)

// Framework represents a supported framework within a language.
type Framework string

const (
	FrameworkNone    Framework = "none"
	FrameworkFlutter Framework = "flutter"
)

// AIProvider represents the AI coding assistant / platform used in the project.
type AIProvider string

const (
	AIProviderGemini AIProvider = "gemini"
)

// StackKey uniquely identifies a language + framework combination.
type StackKey struct {
	Language  Language
	Framework Framework
}

// String returns the canonical manifest key for the stack (e.g., "dart/flutter" or "dart/none").
func (k StackKey) String() string {
	fw := k.Framework
	if fw == "" {
		fw = FrameworkNone
	}
	return fmt.Sprintf("%s/%s", k.Language, fw)
}

// Option represents a selectable choice in the CLI wizard.
type Option struct {
	Value       string
	Label       string
	Description string
}

// InstallRequest holds the validated user inputs and target workspace directory.
type InstallRequest struct {
	TargetDir     string
	Language      Language
	UsesFramework bool
	Framework     Framework
	AI            AIProvider
}

// StackKey returns the StackKey derived from the request.
func (r InstallRequest) StackKey() StackKey {
	fw := FrameworkNone
	if r.UsesFramework && r.Framework != "" {
		fw = r.Framework
	}
	return StackKey{
		Language:  r.Language,
		Framework: fw,
	}
}

// Manifest describes the remote/local asset registry layout.
type Manifest struct {
	Version string                 `json:"version"`
	Stacks  map[string]StackAssets `json:"stacks"`
}

// StackAssets lists scripts and skills associated with a specific stack.
type StackAssets struct {
	Scripts []ScriptEntry `json:"scripts"`
	Skills  []SkillEntry  `json:"skills"`
}

// ScriptEntry maps a registry script file to its destination in the target project.
type ScriptEntry struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

// SkillFile maps a file inside a skill directory to its destination relative to the skill root.
type SkillFile struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

// SkillEntry represents an installable skill bundle.
type SkillEntry struct {
	Name  string      `json:"name"`
	Files []SkillFile `json:"files"`
}

// InstallReport summarizes all actions performed during installation.
type InstallReport struct {
	TargetDir             string
	Stack                 StackKey
	AI                    AIProvider
	InstalledDependencies []string
	InstalledScripts      []string
	InstalledSkills       []string
	UpdatedFiles          []string
	Warnings              []string
	NextSteps             []string
}
