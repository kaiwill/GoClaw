package onboard

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

type Step func(ctx context.Context, state *State) error

type State struct {
	Provider   string
	APIKey     string
	Model      string
	Memory     string
	Channels   []string
	Workspace  string
	ConfigPath string
}

type Wizard struct {
	steps []Step
	state *State
}

func NewWizard() *Wizard {
	return &Wizard{
		steps: []Step{
			selectProvider,
			enterAPIKey,
			selectModel,
			selectMemory,
			selectChannels,
			selectWorkspace,
			generateConfig,
		},
		state: &State{},
	}
}

func (w *Wizard) Run(ctx context.Context) error {
	fmt.Println("=== GoClaw Onboarding Wizard ===")
	fmt.Println()

	for i, step := range w.steps {
		fmt.Printf("[%d/%d] ", i+1, len(w.steps))
		if err := step(ctx, w.state); err != nil {
			return fmt.Errorf("step %d failed: %w", i+1, err)
		}
	}

	fmt.Println()
	fmt.Println("Configuration complete!")
	fmt.Printf("Config saved to: %s\n", w.state.ConfigPath)
	fmt.Println()
	fmt.Println("Run 'goclaw agent' to start!")

	return nil
}

func selectProvider(ctx context.Context, state *State) error {
	fmt.Println("Select AI provider:")
	fmt.Println("  1. OpenAI")
	fmt.Println("  2. Anthropic")
	fmt.Println("  3. Gemini")
	fmt.Println("  4. GLM")
	fmt.Println("  5. Ollama (local)")

	fmt.Print("Choice (1-5): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	switch input {
	case "1":
		state.Provider = "openai"
	case "2":
		state.Provider = "anthropic"
	case "3":
		state.Provider = "gemini"
	case "4":
		state.Provider = "glm"
	case "5":
		state.Provider = "ollama"
	default:
		state.Provider = "openai"
	}

	fmt.Printf("Selected: %s\n", state.Provider)
	return nil
}

func enterAPIKey(ctx context.Context, state *State) error {
	if state.Provider == "ollama" {
		fmt.Println("Ollama doesn't require an API key.")
		state.APIKey = ""
		return nil
	}

	fmt.Printf("Enter %s API key: ", state.Provider)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	state.APIKey = strings.TrimSpace(input)

	if state.APIKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	fmt.Println("API key saved.")
	return nil
}

func selectModel(ctx context.Context, state *State) error {
	models := map[string][]string{
		"openai":    {"gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"},
		"anthropic": {"claude-3-5-sonnet-20241022", "claude-3-opus-20240229", "claude-3-haiku-20240307"},
		"gemini":    {"gemini-2.0-flash-exp", "gemini-1.5-pro", "gemini-1.5-flash"},
		"glm":       {"glm-4", "glm-4-flash", "glm-4-plus"},
		"ollama":    {"llama3", "mistral", "codellama"},
	}

	available, ok := models[state.Provider]
	if !ok {
		available = []string{"default"}
	}

	fmt.Println("Available models:")
	for i, model := range available {
		fmt.Printf("  %d. %s\n", i+1, model)
	}

	fmt.Print("Choice (number): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var idx int
	fmt.Sscanf(input, "%d", &idx)
	if idx < 1 || idx > len(available) {
		idx = 1
	}

	state.Model = available[idx-1]
	fmt.Printf("Selected: %s\n", state.Model)
	return nil
}

func selectMemory(ctx context.Context, state *State) error {
	fmt.Println("Select memory backend:")
	fmt.Println("  1. None (no memory)")
	fmt.Println("  2. SQLite (local file)")
	fmt.Println("  3. Qdrant (vector database)")

	fmt.Print("Choice (1-3): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	switch input {
	case "1":
		state.Memory = "none"
	case "2":
		state.Memory = "sqlite"
	case "3":
		state.Memory = "qdrant"
	default:
		state.Memory = "none"
	}

	fmt.Printf("Selected: %s\n", state.Memory)
	return nil
}

func selectChannels(ctx context.Context, state *State) error {
	fmt.Println("Select channels to enable (comma-separated numbers, empty for none):")
	fmt.Println("  1. Telegram")
	fmt.Println("  2. Discord")
	fmt.Println("  3. Slack")
	fmt.Println("  4. WhatsApp")
	fmt.Println("  5. Matrix")
	fmt.Println("  6. Email")

	fmt.Print("Choice: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		state.Channels = []string{}
		return nil
	}

	channelMap := map[string]string{
		"1": "telegram",
		"2": "discord",
		"3": "slack",
		"4": "whatsapp",
		"5": "matrix",
		"6": "email",
	}

	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if ch, ok := channelMap[part]; ok {
			state.Channels = append(state.Channels, ch)
		}
	}

	fmt.Printf("Selected channels: %v\n", state.Channels)
	return nil
}

func selectWorkspace(ctx context.Context, state *State) error {
	homeDir, _ := os.UserHomeDir()
	defaultPath := homeDir + "/goclaw-workspace"

	fmt.Printf("Workspace directory (default: %s): ", defaultPath)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		state.Workspace = defaultPath
	} else {
		state.Workspace = input
	}

	if err := os.MkdirAll(state.Workspace, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	fmt.Printf("Workspace: %s\n", state.Workspace)
	return nil
}

func generateConfig(ctx context.Context, state *State) error {
	configPath := state.Workspace + "/config.toml"

	state.ConfigPath = configPath

	var b strings.Builder
	b.WriteString("# GoClaw Configuration\n\n")
	b.WriteString("[provider]\n")
	b.WriteString(fmt.Sprintf("name = \"%s\"\n", state.Provider))
	b.WriteString(fmt.Sprintf("model = \"%s\"\n\n", state.Model))

	if state.APIKey != "" {
		b.WriteString(fmt.Sprintf("api_key = \"%s\"\n\n", state.APIKey))
	}

	b.WriteString("[memory]\n")
	b.WriteString(fmt.Sprintf("backend = \"%s\"\n\n", state.Memory))

	b.WriteString("[gateway]\n")
	b.WriteString("host = \"0.0.0.0\"\n")
	b.WriteString("port = 8080\n")

	if len(state.Channels) > 0 {
		b.WriteString("\n[channels]\n")
		for _, ch := range state.Channels {
			b.WriteString(fmt.Sprintf("# %s configuration\n", ch))
		}
	}

	if err := os.WriteFile(configPath, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
