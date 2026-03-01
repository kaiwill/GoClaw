package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Skill struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Commands    []SkillCommand         `json:"commands"`
	Metadata    map[string]interface{} `json:"metadata"`
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

		skillPath := filepath.Join(l.skillsDir, entry.Name(), "skill.json")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		var skill Skill
		if err := json.Unmarshal(data, &skill); err != nil {
			continue
		}

		l.mu.Lock()
		l.skills[skill.Name] = &skill
		l.mu.Unlock()
	}

	return nil
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
