package chat

import (
	"strings"
)

// Trait defines the behavior profile and reasoning mode used in system prompting.
type Trait struct {
	Description     string
	Prompt          string
	ReasoningEffort string
}

// SystemPromptTraits are compatible with the web-agent-framework trait pattern.
var SystemPromptTraits = map[string]Trait{
	"monitoring": {
		Description:     "Camera monitoring and incident analysis",
		ReasoningEffort: "medium",
		Prompt: `You are an AI camera monitoring assistant.
Focus on concrete observations from events/frames and be explicit about uncertainty.
Prioritize actionable safety and operations insights over generic advice.`,
	},
	"analyst": {
		Description:     "Professional and concise information gathering",
		ReasoningEffort: "high",
		Prompt: `You are a helpful, professional assistant.
Be concise, accurate, and practical.`,
	},
}

const (
	DefaultTrait                 = "monitoring"
	DefaultSystemPromptCharacter = "You are Unblink, created by Zapdos Labs. Help users understand camera events, summarize findings, and investigate incident timelines."
	SystemPromptOutputGuideline  = "- Do not use emoji. Keep answers direct and scannable."
)

func GetTraitReasoningEffort(trait string) string {
	if t, ok := SystemPromptTraits[trait]; ok {
		return t.ReasoningEffort
	}
	return SystemPromptTraits[DefaultTrait].ReasoningEffort
}

func BuildSystemPrompt(trait, character string) string {
	if trait == "" {
		trait = DefaultTrait
	}
	t, ok := SystemPromptTraits[trait]
	if !ok {
		t = SystemPromptTraits[DefaultTrait]
	}
	if strings.TrimSpace(character) == "" {
		character = DefaultSystemPromptCharacter
	}

	return "# Trait\n" + t.Prompt + "\n\n" +
		"# Character\n" + character + "\n\n" +
		"# Output Guidelines\n" + SystemPromptOutputGuideline
}
