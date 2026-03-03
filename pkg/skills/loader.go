package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Skill struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Commands    []SkillCommand         `json:"commands"`
	Metadata    map[string]interface{} `json:"metadata"`
	Tools       []SkillTool            `json:"tools,omitempty"`
	Prompts     []string               `json:"prompts,omitempty"`
}

type SkillCommand struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Aliases     []string         `json:"aliases,omitempty"`
	Parameters  []SkillParameter `json:"parameters,omitempty"`
}

type SkillParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// SkillTool defines a tool that can be executed by a skill.
// Similar to zeroclaw-fix-cn's SkillTool, supporting shell/http/script types.
type SkillTool struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Kind        string            `json:"kind"` // "shell", "http", "script"
	Command     string            `json:"command"`
	Args        map[string]string `json:"args,omitempty"`
}

type SkillLoader struct {
	mu        sync.RWMutex
	skills    map[string]*Skill
	skillsDir string
}

func NewSkillLoader(skillsDir string) *SkillLoader {
	return &SkillLoader{
		skills:    make(map[string]*Skill),
		skillsDir: skillsDir,
	}
}

// GetSkillsDir returns the skills directory path.
func (l *SkillLoader) GetSkillsDir() string {
	return l.skillsDir
}

// LoadSkills loads skills from both skill.json and SKILL.toml/SKILL.md files.
// This enables zero-code skill loading similar to zeroclaw-fix-cn.
func (l *SkillLoader) LoadSkills() error {
	if l.skillsDir == "" {
		return nil
	}

	entries, err := os.ReadDir(l.skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(l.skillsDir, entry.Name())

		// Try skill.json first (legacy format)
		jsonPath := filepath.Join(skillPath, "skill.json")
		if data, err := os.ReadFile(jsonPath); err == nil {
			var skill Skill
			if err := json.Unmarshal(data, &skill); err == nil {
				l.mu.Lock()
				l.skills[skill.Name] = &skill
				l.mu.Unlock()
				continue
			}
		}

		// Try SKILL.toml (new format similar to zeroclaw-fix-cn)
		tomlPath := filepath.Join(skillPath, "SKILL.toml")
		if data, err := os.ReadFile(tomlPath); err == nil {
			var manifest SkillManifest
			if err := json.Unmarshal(data, &manifest); err == nil {
				skill := manifest.toSkill()
				skill.Name = entry.Name()
				l.mu.Lock()
				l.skills[skill.Name] = skill
				l.mu.Unlock()
				continue
			}
		}

		// Try SKILL.md (simple markdown format)
		mdPath := filepath.Join(skillPath, "SKILL.md")
		if data, err := os.ReadFile(mdPath); err == nil {
			skill := loadSkillFromMD(entry.Name(), string(data))
			l.mu.Lock()
			l.skills[skill.Name] = skill
			l.mu.Unlock()
		}
	}

	return nil
}

// SkillManifest represents a skill loaded from SKILL.toml
type SkillManifest struct {
	Skill   SkillMeta   `json:"skill"`
	Tools   []SkillTool `json:"tools,omitempty"`
	Prompts []string    `json:"prompts,omitempty"`
}

type SkillMeta struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      *string  `json:"author,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func (m *SkillManifest) toSkill() *Skill {
	version := m.Skill.Version
	if version == "" {
		version = "0.1.0"
	}
	return &Skill{
		Name:        m.Skill.Name,
		Description: m.Skill.Description,
		Version:     version,
		Tools:       m.Tools,
		Prompts:     m.Prompts,
	}
}

func loadSkillFromMD(name string, content string) *Skill {
	// Extract description from first non-heading, non-empty line
	desc := "No description"
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(trimmed) > 0 && trimmed[0] == '#' {
			continue
		}
		desc = trimmed
		break
	}

	return &Skill{
		Name:        name,
		Description: desc,
		Version:     "0.1.0",
		Prompts:     []string{content},
	}
}

func (l *SkillLoader) GetSkill(name string) (*Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	skill, exists := l.skills[name]
	return skill, exists
}

func (l *SkillLoader) ListSkills() []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	skills := make([]*Skill, 0, len(l.skills))
	for _, skill := range l.skills {
		skills = append(skills, skill)
	}

	return skills
}

// GetAllTools returns all tools from all loaded skills.
func (l *SkillLoader) GetAllTools() []SkillTool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var tools []SkillTool
	for _, skill := range l.skills {
		tools = append(tools, skill.Tools...)
	}
	return tools
}

func (l *SkillLoader) AddSkill(skill *Skill) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.skills[skill.Name] = skill

	skillDir := filepath.Join(l.skillsDir, skill.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	data, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal skill: %w", err)
	}

	skillPath := filepath.Join(skillDir, "skill.json")
	if err := os.WriteFile(skillPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	return nil
}

func (l *SkillLoader) RemoveSkill(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.skills[name]; !exists {
		return fmt.Errorf("skill not found: %s", name)
	}

	delete(l.skills, name)

	skillDir := filepath.Join(l.skillsDir, name)
	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("failed to remove skill directory: %w", err)
	}

	return nil
}

type SkillExecutor struct {
	mu       sync.RWMutex
	handlers map[string]SkillHandler
}

type SkillHandler func(ctx context.Context, skill *Skill, command string, args map[string]interface{}) (string, error)

func NewSkillExecutor() *SkillExecutor {
	return &SkillExecutor{
		handlers: make(map[string]SkillHandler),
	}
}

func (e *SkillExecutor) RegisterHandler(skillName string, handler SkillHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[skillName] = handler
}

func (e *SkillExecutor) Execute(ctx context.Context, skill *Skill, command string, args map[string]interface{}) (string, error) {
	e.mu.RLock()
	handler, exists := e.handlers[skill.Name]
	e.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("no handler registered for skill: %s", skill.Name)
	}

	return handler(ctx, skill, command, args)
}

type SkillMetadata struct {
	Author     string    `json:"author"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	License    string    `json:"license"`
	Repository string    `json:"repository"`
	Tags       []string  `json:"tags"`
}

func (s *Skill) GetCommand(name string) *SkillCommand {
	for _, cmd := range s.Commands {
		if cmd.Name == name {
			return &cmd
		}
		for _, alias := range cmd.Aliases {
			if alias == name {
				return &cmd
			}
		}
	}
	return nil
}

func (s *Skill) ValidateCommand(name string, args map[string]interface{}) error {
	cmd := s.GetCommand(name)
	if cmd == nil {
		return fmt.Errorf("command not found: %s", name)
	}

	for _, param := range cmd.Parameters {
		if param.Required {
			if _, exists := args[param.Name]; !exists {
				return fmt.Errorf("required parameter missing: %s", param.Name)
			}
		}
	}

	return nil
}

// GetTool returns a specific tool from the skill by name.
func (s *Skill) GetTool(name string) *SkillTool {
	for _, tool := range s.Tools {
		if tool.Name == name {
			return &tool
		}
	}
	return nil
}
