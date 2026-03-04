package onboard

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	fmt.Println("=== GoClaw 配置向导 ===")
	fmt.Println()

	for i, step := range w.steps {
		fmt.Printf("[%d/%d] ", i+1, len(w.steps))
		if err := step(ctx, w.state); err != nil {
			return fmt.Errorf("步骤 %d 失败: %w", i+1, err)
		}
	}

	fmt.Println()
	fmt.Println("配置完成！")
	fmt.Printf("配置已保存到: %s\n", w.state.ConfigPath)
	fmt.Println()
	fmt.Println("运行 'goclaw agent' 开始使用！")

	return nil
}

func selectProvider(ctx context.Context, state *State) error {
	fmt.Println("选择 AI 提供商:")
	fmt.Println("  1. OpenAI")
	fmt.Println("  2. Anthropic")
	fmt.Println("  3. Gemini")
	fmt.Println("  4. GLM")
	fmt.Println("  5. Ollama (本地)")
	fmt.Println("  6. GiteeAI (免费)")
	fmt.Println("  7. 百炼 (阿里云)")

	fmt.Print("选择 (1-7): ")
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
	case "6":
		state.Provider = "gitee"
	case "7":
		state.Provider = "bailian"
	default:
		state.Provider = "openai"
	}

	fmt.Printf("已选择: %s\n", state.Provider)
	return nil
}

func enterAPIKey(ctx context.Context, state *State) error {
	if state.Provider == "ollama" {
		fmt.Println("Ollama 不需要 API 密钥。")
		state.APIKey = ""
		return nil
	}

	fmt.Printf("输入 %s API 密钥: ", state.Provider)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	state.APIKey = strings.TrimSpace(input)

	if state.APIKey == "" {
		return fmt.Errorf("API 密钥不能为空")
	}

	fmt.Println("API 密钥已保存。")
	return nil
}

func selectModel(ctx context.Context, state *State) error {
	models := map[string][]string{
		"openai":    {"gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"},
		"anthropic": {"claude-3-5-sonnet-20241022", "claude-3-opus-20240229", "claude-3-haiku-20240307"},
		"gemini":    {"gemini-2.0-flash-exp", "gemini-1.5-pro", "gemini-1.5-flash"},
		"glm":       {"glm-4", "glm-4-flash", "glm-4-plus"},
		"ollama":    {"llama3", "mistral", "codellama"},
		"gitee":     {"Qwen3-8B", "GLM-4.7-Flash", "InternLM3-8B-Instruct", "DeepSeek-R1-Distill-Qwen-14B", "GLM-4.6V-Flash", "SenseVoiceSmall", "GLM-ASR"},
		"bailian":    {"qwen-turbo", "qwen-plus", "qwen-max", "qwen-long"},
	}

	available, ok := models[state.Provider]
	if !ok {
		available = []string{"default"}
	}

	fmt.Println("可用模型:")
	for i, model := range available {
		fmt.Printf("  %d. %s\n", i+1, model)
	}

	fmt.Print("选择 (输入数字): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var idx int
	fmt.Sscanf(input, "%d", &idx)
	if idx < 1 || idx > len(available) {
		idx = 1
	}

	state.Model = available[idx-1]
	fmt.Printf("已选择: %s\n", state.Model)
	return nil
}

func selectMemory(ctx context.Context, state *State) error {
	fmt.Println("选择存储后端:")
	fmt.Println("  1. 无 (不使用存储)")
	fmt.Println("  2. SQLite (本地文件)")
	fmt.Println("  3. Qdrant (向量数据库)")

	fmt.Print("选择 (1-3): ")
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

	fmt.Printf("已选择: %s\n", state.Memory)
	return nil
}

func selectChannels(ctx context.Context, state *State) error {
	fmt.Println("选择要启用的通道 (用逗号分隔的数字，留空表示不启用):")
	fmt.Println("  1. Telegram")
	fmt.Println("  2. Discord")
	fmt.Println("  3. Slack")
	fmt.Println("  4. WhatsApp")
	fmt.Println("  5. Matrix")
	fmt.Println("  6. Email")
	fmt.Println("  7. 钉钉")
	fmt.Println("  8. 飞书")

	fmt.Print("选择: ")
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
		"7": "dingtalk",
		"8": "lark",
	}

	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if ch, ok := channelMap[part]; ok {
			state.Channels = append(state.Channels, ch)
		}
	}

	fmt.Printf("已选择的通道: %v\n", state.Channels)
	return nil
}

func selectWorkspace(ctx context.Context, state *State) error {
	homeDir, _ := os.UserHomeDir()
	defaultPath := filepath.Join(homeDir, ".goclaw", "workspace")

	fmt.Printf("工作空间目录 (默认: %s): ", defaultPath)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		state.Workspace = defaultPath
	} else {
		state.Workspace = input
	}

	if err := os.MkdirAll(state.Workspace, 0755); err != nil {
		return fmt.Errorf("创建工作空间失败: %w", err)
	}

	fmt.Printf("工作空间: %s\n", state.Workspace)
	return nil
}

func generateConfig(ctx context.Context, state *State) error {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".goclaw", "config.toml")

	state.ConfigPath = configPath

	var b strings.Builder
	b.WriteString("# GoClaw 配置\n\n")
	b.WriteString("[provider]\n")
	b.WriteString(fmt.Sprintf("name = \"%s\"\n", state.Provider))
	b.WriteString(fmt.Sprintf("model = \"%s\"\n", state.Model))

	if state.Provider == "gitee" {
		b.WriteString("url = \"custom:https://ai.gitee.com/v1\"\n")
	}

	if state.Provider == "bailian" {
		b.WriteString("url = \"custom:https://dashscope.aliyuncs.com/api/v1\"\n")
	}

	if state.APIKey != "" {
		b.WriteString(fmt.Sprintf("api_key = \"%s\"\n\n", state.APIKey))
	} else {
		b.WriteString("\n")
	}

	b.WriteString("[memory]\n")
	b.WriteString(fmt.Sprintf("backend = \"%s\"\n\n", state.Memory))

	b.WriteString("[gateway]\n")
	b.WriteString("host = \"0.0.0.0\"\n")
	b.WriteString("port = 4096\n")

	if len(state.Channels) > 0 {
		b.WriteString("\n[channels]\n")
		for _, ch := range state.Channels {
			b.WriteString(fmt.Sprintf("# %s 配置\n", ch))
		}
	}

	if err := os.WriteFile(configPath, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}

	return nil
}
