// Package skills 实现了 Agent Skills 开放标准。
// 详见规范：https://agentskills.io
//
// Skill 以 SKILL.md 文件形式存储，包含 YAML frontmatter 元数据和 Markdown 指令内容。
// 支持自动发现、解析、验证和注入到系统提示词中。
package skills

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
	"gopkg.in/yaml.v3"
)

const (
	// SkillFileName 是技能文件的标准文件名。
	SkillFileName = "SKILL.md"
	// MaxNameLength 是技能名称的最大字符数。
	MaxNameLength = 64
	// MaxDescriptionLength 是技能描述的最大字符数。
	MaxDescriptionLength = 1024
	// MaxCompatibilityLength 是兼容性说明的最大字符数。
	MaxCompatibilityLength = 500
)

// namePattern 是技能名称的合法格式正则：字母数字组合，可用连字符分隔。
var namePattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

// Skill 表示一个解析后的 SKILL.md 文件。
type Skill struct {
	Name          string            `yaml:"name" json:"name"`                                       // 技能名称，必须与目录名匹配
	Description   string            `yaml:"description" json:"description"`                         // 技能描述
	License       string            `yaml:"license,omitempty" json:"license,omitempty"`              // 许可证（可选）
	Compatibility string            `yaml:"compatibility,omitempty" json:"compatibility,omitempty"` // 兼容性说明（可选）
	Metadata      map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`           // 自定义元数据（可选）
	Instructions  string            `yaml:"-" json:"instructions"`                                  // Markdown 指令内容（frontmatter 之后的部分）
	Path          string            `yaml:"-" json:"path"`                                          // 技能所在目录路径
	SkillFilePath string            `yaml:"-" json:"skill_file_path"`                               // SKILL.md 文件的完整路径
}

// Validate 检查技能是否符合规范要求。
//
// 验证规则：
//   - name 必填，不超过 64 字符，符合命名规范，且必须与目录名匹配
//   - description 必填，不超过 1024 字符
//   - compatibility 不超过 500 字符
func (s *Skill) Validate() error {
	var errs []error

	if s.Name == "" {
		errs = append(errs, errors.New("name is required"))
	} else {
		if len(s.Name) > MaxNameLength {
			errs = append(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
		}
		if !namePattern.MatchString(s.Name) {
			errs = append(errs, errors.New("name must be alphanumeric with hyphens, no leading/trailing/consecutive hyphens"))
		}
		if s.Path != "" && !strings.EqualFold(filepath.Base(s.Path), s.Name) {
			errs = append(errs, fmt.Errorf("name %q must match directory %q", s.Name, filepath.Base(s.Path)))
		}
	}

	if s.Description == "" {
		errs = append(errs, errors.New("description is required"))
	} else if len(s.Description) > MaxDescriptionLength {
		errs = append(errs, fmt.Errorf("description exceeds %d characters", MaxDescriptionLength))
	}

	if len(s.Compatibility) > MaxCompatibilityLength {
		errs = append(errs, fmt.Errorf("compatibility exceeds %d characters", MaxCompatibilityLength))
	}

	return errors.Join(errs...)
}

// Parse 解析一个 SKILL.md 文件，提取 YAML frontmatter 和 Markdown 指令内容。
func Parse(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	frontmatter, body, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	skill.Instructions = strings.TrimSpace(body)
	skill.Path = filepath.Dir(path)
	skill.SkillFilePath = path

	return &skill, nil
}

// splitFrontmatter 从 Markdown 内容中提取 YAML frontmatter 和正文。
// frontmatter 必须以 "---" 开头和结尾。
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	// Normalize line endings to \n for consistent parsing.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", errors.New("no YAML frontmatter found")
	}

	rest := strings.TrimPrefix(content, "---\n")
	before, after, ok := strings.Cut(rest, "\n---")
	if !ok {
		return "", "", errors.New("unclosed frontmatter")
	}

	return before, after, nil
}

// Discover 在给定的路径列表中递归发现所有有效的技能文件。
// 使用 fastwalk 并发遍历，支持符号链接目录。
// 无效的技能文件会被跳过并记录警告日志。
func Discover(paths []string) []*Skill {
	var skills []*Skill
	var mu sync.Mutex
	seen := make(map[string]bool)

	for _, base := range paths {
		// We use fastwalk with Follow: true instead of filepath.WalkDir because
		// WalkDir doesn't follow symlinked directories at any depth—only entry
		// points. This ensures skills in symlinked subdirectories are discovered.
		// fastwalk is concurrent, so we protect shared state (seen, skills) with mu.
		conf := fastwalk.Config{
			Follow:  true,
			ToSlash: fastwalk.DefaultToSlash(),
		}
		fastwalk.Walk(&conf, base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() || d.Name() != SkillFileName {
				return nil
			}
			mu.Lock()
			if seen[path] {
				mu.Unlock()
				return nil
			}
			seen[path] = true
			mu.Unlock()
			skill, err := Parse(path)
			if err != nil {
				slog.Warn("Failed to parse skill file", "path", path, "error", err)
				return nil
			}
			if err := skill.Validate(); err != nil {
				slog.Warn("Skill validation failed", "path", path, "error", err)
				return nil
			}
			slog.Debug("Successfully loaded skill", "name", skill.Name, "path", path)
			mu.Lock()
			skills = append(skills, skill)
			mu.Unlock()
			return nil
		})
	}

	return skills
}

// ToPromptXML 将技能列表转换为 XML 格式，用于注入到系统提示词中。
// 包含每个技能的名称、描述和文件位置。
// 如果技能列表为空，返回空字符串。
func ToPromptXML(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for _, s := range skills {
		sb.WriteString("  <skill>\n")
		fmt.Fprintf(&sb, "    <name>%s</name>\n", escape(s.Name))
		fmt.Fprintf(&sb, "    <description>%s</description>\n", escape(s.Description))
		fmt.Fprintf(&sb, "    <location>%s</location>\n", escape(s.SkillFilePath))
		sb.WriteString("  </skill>\n")
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}

// escape 对字符串进行 XML 实体转义，防止注入。
func escape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}
