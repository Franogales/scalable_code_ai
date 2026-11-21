package ai

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Franogales/scalable_code_ai/internal/domain"
	"github.com/Franogales/scalable_code_ai/internal/infrastructure/fs"
)

const (
	agentsFileName = "AGENTS.md"
	startMarker    = "<!-- scalable-code-ai:start -->"
	endMarker      = "<!-- scalable-code-ai:end -->"
)

// GeminiInstaller installs skills into .agents/skills/ and updates AGENTS.md.
type GeminiInstaller struct {
	fs domain.FileSystem
}

// NewGeminiInstaller creates a GeminiInstaller.
func NewGeminiInstaller(fs domain.FileSystem) *GeminiInstaller {
	return &GeminiInstaller{fs: fs}
}

// Provider returns domain.AIProviderGemini.
func (g *GeminiInstaller) Provider() domain.AIProvider {
	return domain.AIProviderGemini
}

// InstallSkills fetches and writes each skill bundle into .agents/skills/<skill-name>/.
func (g *GeminiInstaller) InstallSkills(ctx context.Context, targetDir string, skills []domain.SkillEntry, source domain.AssetSource) ([]string, error) {
	var installed []string
	for _, skill := range skills {
		skillDir := filepath.Join(targetDir, ".agents", "skills", skill.Name)
		for _, file := range skill.Files {
			content, err := source.FetchFile(ctx, file.Source)
			if err != nil {
				return installed, fmt.Errorf("fetch skill file %s: %w", file.Source, err)
			}
			destPath := filepath.Join(skillDir, filepath.FromSlash(file.Dest))
			if err := g.fs.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return installed, fmt.Errorf("create dir for skill %s: %w", destPath, err)
			}
			if err := g.fs.WriteFile(destPath, content, 0o644); err != nil {
				return installed, fmt.Errorf("write skill file %s: %w", destPath, err)
			}
		}
		installed = append(installed, filepath.ToSlash(filepath.Join(".agents", "skills", skill.Name)))
	}
	return installed, nil
}

// UpdateInstructions inserts or updates the scalable_code_ai instructions block inside AGENTS.md.
func (g *GeminiInstaller) UpdateInstructions(_ context.Context, targetDir string, lang domain.Language, fw domain.Framework) (string, error) {
	agentsPath := filepath.Join(targetDir, agentsFileName)
	var existing string
	if g.fs.Exists(agentsPath) {
		raw, err := g.fs.ReadFile(agentsPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", agentsFileName, err)
		}
		existing = string(raw)
	}

	body := buildAgentsInstructions(lang, fw)
	updated := fs.UpsertMarkedBlock(existing, startMarker, endMarker, body)
	if err := g.fs.WriteFile(agentsPath, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", agentsFileName, err)
	}
	return agentsFileName, nil
}

func buildAgentsInstructions(lang domain.Language, fw domain.Framework) string {
	stackLabel := string(lang)
	if fw != "" && fw != domain.FrameworkNone {
		stackLabel = fmt.Sprintf("%s/%s", lang, fw)
	}

	return strings.TrimSpace(fmt.Sprintf(`
## Scalable Code Quality & Architecture (%s)

Este proyecto utiliza **scalable_code_ai** para validar empíricamente limpieza de código, alta cohesión, bajo acoplamiento y respeto a las fronteras arquitectónicas.

### Skills disponibles en %s
- **%s** (%s): Analiza la arquitectura del proyecto, confirma capas/fronteras y umbrales con el usuario, y genera %s y %s. Ejecutar inicialmente si %s aún no existe.
- **%s**: Validación empírica obligatoria **al finalizar cualquier tarea de implementación o refactorización**.

### Regla obligatoria al terminar una tarea de código
1. Activa la skill **scalable-code-check** (o ejecuta %s).
2. Si existen findings en %s, delega o ejecuta el ciclo de corrección (máximo 3 rondas) hasta pasar las validaciones y tests sin deshabilitar reglas arbitrariamente.
`,
		stackLabel,
		"`.agents/skills/`",
		"`scalable-code-setup`",
		"`/scalable-code-setup`",
		"`analysis_options.yaml`",
		"`.scalable-code/config.json`",
		"`.scalable-code/config.json`",
		"`scalable-code-check`",
		"`dart run tool/scalable_code_check.dart`",
		"`.scalable-code/report.md`",
	))
}
