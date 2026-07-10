package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/1node/airon/internal/provider"
	"github.com/1node/airon/internal/tool"
	"gopkg.in/yaml.v3"
)

var skillNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func Load(path string) (tool.Tool, error) {
	targetPath, baseName, err := resolveSkillPath(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read skill file: %w", err)
	}

	meta, body, err := parseFrontmatterAndBody(string(content))
	if err != nil {
		return nil, err
	}

	name, err := resolveSkillName(meta, baseName)
	if err != nil {
		return nil, err
	}

	return NewSkillTool(
		name,
		strings.TrimSpace(meta.Description),
		strings.TrimSpace(body),
		nil,
		0,
	), nil
}

func LoadAll(paths []string) ([]tool.Tool, error) {
	tools := make([]tool.Tool, 0, len(paths))
	for _, path := range paths {
		loaded, err := Load(path)
		if err != nil {
			return nil, fmt.Errorf("load skill %q: %w", path, err)
		}
		tools = append(tools, loaded)
	}
	return tools, nil
}

func LoadConfigured(path string, providerClient provider.Provider, maxDepth int) (tool.Tool, error) {
	targetPath, baseName, err := resolveSkillPath(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read skill file: %w", err)
	}

	meta, body, err := parseFrontmatterAndBody(string(content))
	if err != nil {
		return nil, err
	}

	name, err := resolveSkillName(meta, baseName)
	if err != nil {
		return nil, err
	}

	return NewSkillTool(
		name,
		strings.TrimSpace(meta.Description),
		strings.TrimSpace(body),
		providerClient,
		maxDepth,
	), nil
}

func LoadAllConfigured(paths []string, providerClient provider.Provider, maxDepth int) ([]tool.Tool, error) {
	tools := make([]tool.Tool, 0, len(paths))
	for _, path := range paths {
		loaded, err := LoadConfigured(path, providerClient, maxDepth)
		if err != nil {
			return nil, fmt.Errorf("load skill %q: %w", path, err)
		}
		tools = append(tools, loaded)
	}
	return tools, nil
}

func resolveSkillName(meta frontmatter, baseName string) (string, error) {
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = baseName
	}
	if !skillNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid skill name %q: must match [a-z][a-z0-9_-]*", name)
	}
	return name, nil
}

func resolveSkillPath(path string) (string, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("stat skill path: %w", err)
	}

	if info.IsDir() {
		return filepath.Join(path, "SKILL.md"), filepath.Base(path), nil
	}

	base := filepath.Base(path)
	return path, strings.TrimSuffix(base, filepath.Ext(base)), nil
}

func parseFrontmatterAndBody(content string) (frontmatter, string, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return frontmatter{}, "", fmt.Errorf("missing YAML frontmatter")
	}

	rest := strings.TrimPrefix(normalized, "---\n")
	endIndex := strings.Index(rest, "\n---\n")
	if endIndex < 0 {
		return frontmatter{}, "", fmt.Errorf("invalid YAML frontmatter")
	}

	fmBlock := rest[:endIndex]
	body := rest[endIndex+len("\n---\n"):]

	var meta frontmatter
	if err := yaml.Unmarshal([]byte(fmBlock), &meta); err != nil {
		return frontmatter{}, "", fmt.Errorf("parse YAML frontmatter: %w", err)
	}

	return meta, body, nil
}
