package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Franogales/scalable_code_ai/internal/application"
	"github.com/Franogales/scalable_code_ai/internal/domain"
	"github.com/Franogales/scalable_code_ai/internal/infrastructure/ai"
	"github.com/Franogales/scalable_code_ai/internal/infrastructure/exec"
	"github.com/Franogales/scalable_code_ai/internal/infrastructure/fs"
	"github.com/Franogales/scalable_code_ai/internal/infrastructure/source"
	"github.com/Franogales/scalable_code_ai/internal/infrastructure/stacks"
	"github.com/Franogales/scalable_code_ai/internal/ui"
)

func main() {
	var (
		targetDirFlag = flag.String("dir", ".", "Ruta del proyecto destino")
		sourceDirFlag = flag.String("source", "", "Ruta local al directorio registry/ (opcional, para desarrollo/pruebas)")
		repoFlag      = flag.String("repo", source.DefaultRepo, "Repositorio GitHub fuente (owner/repo)")
		refFlag       = flag.String("ref", source.DefaultRef, "Rama o tag del repositorio GitHub fuente")
		langFlag      = flag.String("lang", "", "Lenguaje de programación (omite selección interactiva si se combina con --ai)")
		frameworkFlag = flag.String("framework", "", "Framework (opcional, ej. flutter)")
		aiFlag        = flag.String("ai", "", "Asistente/plataforma de IA (ej. gemini)")
	)
	flag.Parse()

	absTargetDir, err := filepath.Abs(*targetDirFlag)
	if err != nil {
		exitWithError(fmt.Errorf("resolver directorio destino: %w", err))
	}

	osfs := fs.NewOSFileSystem()
	runner := exec.NewOSRunner()

	var assetSource domain.AssetSource
	if *sourceDirFlag != "" {
		absSourceDir, err := filepath.Abs(*sourceDirFlag)
		if err != nil {
			exitWithError(fmt.Errorf("resolver directorio source: %w", err))
		}
		assetSource = source.NewLocalSource(absSourceDir, osfs)
	} else {
		assetSource = source.NewGitHubSource(*repoFlag, *refFlag)
	}

	stackRegistry := stacks.NewRegistry(
		stacks.NewFlutterInstaller(osfs, runner),
	)
	aiRegistry := ai.NewRegistry(
		ai.NewGeminiInstaller(osfs),
	)

	prompter := ui.NewHuhPrompter()
	useCase := application.NewInstallUseCase(prompter, assetSource, stackRegistry, aiRegistry, osfs)

	fmt.Println("scalable_code_ai — Instalador de validación empírica de arquitectura y código limpio")
	fmt.Printf("Proyecto destino: %s\n\n", absTargetDir)

	var req domain.InstallRequest
	if *langFlag != "" && *aiFlag != "" {
		fw := domain.Framework(*frameworkFlag)
		req = domain.InstallRequest{
			TargetDir:     absTargetDir,
			Language:      domain.Language(*langFlag),
			UsesFramework: fw != "" && fw != domain.FrameworkNone,
			Framework:     fw,
			AI:            domain.AIProvider(*aiFlag),
		}
	} else {
		req, err = useCase.GatherRequest(absTargetDir)
		if err != nil {
			exitWithError(err)
		}
	}

	fmt.Println("\nInstalando herramientas, scripts y skills...")
	report, err := useCase.Execute(context.Background(), req)
	if err != nil {
		exitWithError(err)
	}

	printReport(report)
}

func printReport(report domain.InstallReport) {
	fmt.Println("\nInstalación completada con éxito.")
	fmt.Printf("  Stack: %s | IA: %s\n", report.Stack.String(), report.AI)

	printList("Dependencias instaladas", report.InstalledDependencies)
	printList("Scripts copiados", report.InstalledScripts)
	printList("Skills instaladas", report.InstalledSkills)
	printList("Archivos actualizados", report.UpdatedFiles)
	printList("Advertencias", report.Warnings)
	printList("Siguientes pasos", report.NextSteps)
}

func printList(title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", title)
	for _, item := range items {
		fmt.Printf("  - %s\n", item)
	}
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
	os.Exit(1)
}
