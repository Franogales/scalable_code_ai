package domain

// AvailableLanguages returns the selectable programming languages.
func AvailableLanguages() []Option {
	return []Option{
		{
			Value:       string(LanguageDart),
			Label:       "Dart",
			Description: "Lenguaje Dart (compatible con Flutter)",
		},
	}
}

// AvailableFrameworks returns the selectable frameworks for the given language.
func AvailableFrameworks(lang Language) []Option {
	switch lang {
	case LanguageDart:
		return []Option{
			{
				Value:       string(FrameworkFlutter),
				Label:       "Flutter",
				Description: "Framework multiplataforma UI de Dart",
			},
		}
	default:
		return nil
	}
}

// AvailableAIProviders returns the selectable AI assistants/platforms.
func AvailableAIProviders() []Option {
	return []Option{
		{
			Value:       string(AIProviderGemini),
			Label:       "Gemini (Antigravity / Gemini CLI)",
			Description: "Instala skills en .agents/skills/ y actualiza AGENTS.md",
		},
	}
}
