package ui

import (
	"fmt"

	"github.com/Franogales/scalable_code_ai/internal/domain"
	"github.com/charmbracelet/huh"
)

// HuhPrompter implements domain.Prompter using interactive terminal prompts.
type HuhPrompter struct{}

// NewHuhPrompter creates a HuhPrompter.
func NewHuhPrompter() *HuhPrompter {
	return &HuhPrompter{}
}

// SelectLanguage asks the user to choose a programming language from a list.
func (p *HuhPrompter) SelectLanguage(options []domain.Option) (domain.Language, error) {
	selected, err := selectOption("¿Qué lenguaje de programación utiliza el proyecto?", options)
	if err != nil {
		return "", err
	}
	return domain.Language(selected), nil
}

// ConfirmFramework asks whether the project uses a framework.
func (p *HuhPrompter) ConfirmFramework() (bool, error) {
	var usesFramework bool
	err := huh.NewConfirm().
		Title("¿Estás utilizando algún framework?").
		Affirmative("Sí").
		Negative("No").
		Value(&usesFramework).
		Run()
	if err != nil {
		return false, err
	}
	return usesFramework, nil
}

// SelectFramework asks the user to choose a framework from a list.
func (p *HuhPrompter) SelectFramework(options []domain.Option) (domain.Framework, error) {
	selected, err := selectOption("¿Qué framework utiliza el proyecto?", options)
	if err != nil {
		return "", err
	}
	return domain.Framework(selected), nil
}

// SelectAIProvider asks the user to choose the AI assistant/platform.
func (p *HuhPrompter) SelectAIProvider(options []domain.Option) (domain.AIProvider, error) {
	selected, err := selectOption("¿Qué asistente o plataforma de IA estás utilizando?", options)
	if err != nil {
		return "", err
	}
	return domain.AIProvider(selected), nil
}

func selectOption(title string, options []domain.Option) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options available for %q", title)
	}
	huhOptions := make([]huh.Option[string], 0, len(options))
	for _, opt := range options {
		label := opt.Label
		if opt.Description != "" {
			label = fmt.Sprintf("%s — %s", opt.Label, opt.Description)
		}
		huhOptions = append(huhOptions, huh.NewOption(label, opt.Value))
	}

	var value string
	err := huh.NewSelect[string]().
		Title(title).
		Options(huhOptions...).
		Value(&value).
		Run()
	if err != nil {
		return "", err
	}
	return value, nil
}
