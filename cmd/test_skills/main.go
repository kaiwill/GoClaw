package main

import (
	"context"
	"fmt"
	"log"

	"github.com/zeroclaw-labs/goclaw/pkg/skills"
)

func main() {
	// 创建 skill loader 并加载 skills
	loader := skills.NewSkillLoader(filepath.Join(os.Getenv("HOME"), ".zeroclaw/workspace/skills"))

	if err := loader.LoadSkills(); err != nil {
		log.Fatalf("Failed to load skills: %v", err)
	}

	// 列出所有加载的 skills
	allSkills := loader.ListSkills()
	fmt.Printf("Loaded %d skills:\n", len(allSkills))
	for _, s := range allSkills {
		fmt.Printf("  - %s: %s (version: %s)\n", s.Name, s.Description, s.Version)
		if len(s.Tools) > 0 {
			fmt.Printf("    Tools:\n")
			for _, t := range s.Tools {
				fmt.Printf("      - %s (%s): %s\n", t.Name, t.Kind, t.Description)
			}
		}
	}
	fmt.Println()

	// 测试 hello-world skill 的工具
	for _, skill := range allSkills {
		if skill.Name == "hello-world" {
			fmt.Printf("Testing skill: %s\n\n", skill.Name)

			for _, tool := range skill.Tools {
				testTool(skill, tool)
			}
		}
	}
}

func testTool(skill *skills.Skill, tool skills.SkillTool) {
	fmt.Printf("=== Testing tool: %s:%s ===\n", skill.Name, tool.Name)
	fmt.Printf("Kind: %s\n", tool.Kind)
	fmt.Printf("Command: %s\n", tool.Command)

	executor := skills.NewSkillToolExecutor(skill, tool, "")

	ctx := context.Background()

	// 根据工具类型构造不同的参数
	var args map[string]interface{}
	switch tool.Kind {
	case "shell":
		if tool.Name == "greet" {
			args = map[string]interface{}{
				"name": "World",
			}
		} else {
			args = map[string]interface{}{}
		}
	case "http":
		args = map[string]interface{}{
			"path": "/get",
		}
	case "script":
		args = map[string]interface{}{}
	}

	result, err := executor.Execute(ctx, args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Success: %v\n", result.Success)
	fmt.Printf("Output: %s\n", result.Output)
	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}
	fmt.Println()
}
