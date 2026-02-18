package services

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Workspace personality/configuration files, inspired by PicoClaw.
// Each project can have optional markdown files that shape agent behavior:
//   - AGENT.md   : Agent behavior instructions (tool usage patterns, guidelines)
//   - USER.md    : User preferences (tone, language, context)
//   - IDENTITY.md: Agent identity (name, personality, values)
//   - TOOLS.md   : Extra tool descriptions / usage examples

const (
	AgentFileName    = "AGENT.md"
	UserFileName     = "USER.md"
	IdentityFileName = "IDENTITY.md"
	ToolsFileName    = "TOOLS.md"
)

// WorkspaceContext holds all optional workspace personality files for a project.
type WorkspaceContext struct {
	Agent    string // AGENT.md content
	User     string // USER.md content
	Identity string // IDENTITY.md content
	Tools    string // TOOLS.md content
	Memory   string // MEMORY.md content (already existed)
	Skills   string // Aggregated skills context
}

// LoadWorkspaceContext reads all workspace personality files for a project.
func LoadWorkspaceContext(ws *WorkspaceManager, projectID string, logger *slog.Logger) WorkspaceContext {
	if ws == nil || projectID == "" {
		return WorkspaceContext{}
	}

	projectPath := ws.GetProjectPath(projectID)
	ctx := WorkspaceContext{}

	files := map[string]*string{
		AgentFileName:    &ctx.Agent,
		UserFileName:     &ctx.User,
		IdentityFileName: &ctx.Identity,
		ToolsFileName:    &ctx.Tools,
		"MEMORY.md":      &ctx.Memory,
	}

	for name, dest := range files {
		data, err := os.ReadFile(filepath.Join(projectPath, name))
		if err != nil {
			continue // File doesn't exist — normal
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			*dest = content
			if logger != nil {
				logger.Debug("workspace file loaded", "file", name, "bytes", len(content))
			}
		}
	}

	return ctx
}

// FormatForPrompt builds a unified context block from all workspace files.
// Returns empty string if no files are present.
func (wc WorkspaceContext) FormatForPrompt() string {
	var sections []string

	if wc.Identity != "" {
		sections = append(sections, fmt.Sprintf("IDENTITY:\n%s", wc.Identity))
	}

	if wc.Agent != "" {
		sections = append(sections, fmt.Sprintf("AGENT INSTRUCTIONS:\n%s", wc.Agent))
	}

	if wc.User != "" {
		sections = append(sections, fmt.Sprintf("USER PREFERENCES:\n%s", wc.User))
	}

	if wc.Tools != "" {
		sections = append(sections, fmt.Sprintf("TOOL USAGE GUIDE:\n%s", wc.Tools))
	}

	if wc.Skills != "" {
		sections = append(sections, fmt.Sprintf("AVAILABLE SKILLS:\n%s", wc.Skills))
	}

	if wc.Memory != "" {
		sections = append(sections, fmt.Sprintf("LONG-TERM MEMORY:\n%s", wc.Memory))
	}

	if len(sections) == 0 {
		return ""
	}

	return "---\nWORKSPACE CONTEXT:\n" + strings.Join(sections, "\n---\n") + "\n---"
}

// ─── Skills System ──────────────────────────────────────────────────────────

// SkillInfo describes a discovered skill.
type SkillInfo struct {
	Name        string
	Description string
	Path        string
	Source      string // "project", "global", or "builtin"
}

// SkillsLoader discovers and loads SKILL.md files from multiple directories.
// Priority: project skills > global skills > builtin skills (higher priority overrides).
type SkillsLoader struct {
	logger *slog.Logger
}

func NewSkillsLoader(logger *slog.Logger) *SkillsLoader {
	return &SkillsLoader{logger: logger}
}

// ListSkills discovers all SKILL.md files across the three search paths.
func (sl *SkillsLoader) ListSkills(projectSkillsDir, globalSkillsDir, builtinSkillsDir string) []SkillInfo {
	seen := map[string]bool{}
	var skills []SkillInfo

	// Priority order: project > global > builtin
	for _, source := range []struct {
		dir    string
		source string
	}{
		{projectSkillsDir, "project"},
		{globalSkillsDir, "global"},
		{builtinSkillsDir, "builtin"},
	} {
		if source.dir == "" {
			continue
		}
		dirs, err := os.ReadDir(source.dir)
		if err != nil {
			continue
		}
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			name := d.Name()
			if seen[name] {
				continue // Higher-priority source already loaded this skill
			}
			skillFile := filepath.Join(source.dir, name, "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue
			}

			info := SkillInfo{
				Name:   name,
				Path:   skillFile,
				Source: source.source,
			}

			// Try to extract description from frontmatter
			if data, err := os.ReadFile(skillFile); err == nil {
				info.Description = extractSkillDescription(string(data))
			}

			skills = append(skills, info)
			seen[name] = true
		}
	}

	return skills
}

// LoadSkill reads and returns the content of a specific skill by name.
func (sl *SkillsLoader) LoadSkill(name string, searchDirs ...string) (string, bool) {
	for _, dir := range searchDirs {
		if dir == "" {
			continue
		}
		skillFile := filepath.Join(dir, name, "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}
		return stripFrontmatter(string(data)), true
	}
	return "", false
}

// BuildSkillsSummary builds a summary of all available skills for injection into prompts.
func (sl *SkillsLoader) BuildSkillsSummary(projectSkillsDir, globalSkillsDir, builtinSkillsDir string) string {
	allSkills := sl.ListSkills(projectSkillsDir, globalSkillsDir, builtinSkillsDir)
	if len(allSkills) == 0 {
		return ""
	}

	var lines []string
	for _, s := range allSkills {
		desc := s.Description
		if desc == "" {
			desc = "(no description)"
		}
		lines = append(lines, fmt.Sprintf("- **%s** [%s]: %s", s.Name, s.Source, desc))
	}
	return strings.Join(lines, "\n")
}

// LoadSkillsForContext loads full content of specific skills by name.
func (sl *SkillsLoader) LoadSkillsForContext(names []string, searchDirs ...string) string {
	if len(names) == 0 {
		return ""
	}

	var parts []string
	for _, name := range names {
		content, ok := sl.LoadSkill(name, searchDirs...)
		if ok {
			parts = append(parts, fmt.Sprintf("### Skill: %s\n\n%s", name, content))
		}
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// ─── Helpers ────────────────────────────────────────────────────────────────

var frontmatterRe = regexp.MustCompile(`(?s)^---\n(.*?)\n---`)

// extractSkillDescription extracts the "description" from YAML frontmatter.
func extractSkillDescription(content string) string {
	match := frontmatterRe.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}

	for _, line := range strings.Split(match[1], "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			desc := strings.TrimPrefix(line, "description:")
			desc = strings.TrimSpace(desc)
			desc = strings.Trim(desc, "\"'")
			return desc
		}
	}

	return ""
}

// stripFrontmatter removes YAML frontmatter from markdown content.
func stripFrontmatter(content string) string {
	return frontmatterRe.ReplaceAllString(content, "")
}
