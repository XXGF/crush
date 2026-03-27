// Package commands 提供自定义命令的加载和管理功能。
//
// 自定义命令支持三种来源：
//   - 用户级命令：从 XDG 配置目录或 ~/.crush/commands/ 加载
//   - 项目级命令：从项目 .crush/commands/ 目录加载
//   - MCP 命令：从已连接的 MCP 服务器动态加载
//
// 命令以 Markdown 文件形式存储，支持通过 $ARG_NAME 语法定义参数占位符。
// 命令 ID 采用 "来源前缀:路径" 的命名规则，如 "user:my-command" 或 "project:tools:lint"。
package commands

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
)

// namedArgPattern 匹配命令内容中的命名参数占位符。
// 参数格式为 $ARG_NAME，其中 ARG_NAME 由大写字母、数字和下划线组成，且必须以大写字母开头。
// 示例：$FILE_PATH、$OUTPUT_DIR。
var namedArgPattern = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)`)

const (
	// userCommandPrefix 是用户级自定义命令的 ID 前缀。
	userCommandPrefix = "user:"
	// projectCommandPrefix 是项目级自定义命令的 ID 前缀。
	projectCommandPrefix = "project:"
)

// Argument 表示命令参数的元数据。
type Argument struct {
	// ID 是参数的唯一标识符，对应占位符中的名称（如 $FILE_PATH 中的 "FILE_PATH"）。
	ID string
	// Title 是参数的显示名称，用于 UI 展示。
	Title string
	// Description 是参数的详细描述信息。
	Description string
	// Required 标识该参数是否为必填项。自定义命令中提取的参数默认为必填。
	Required bool
}

// MCPPrompt 表示从 MCP 服务器加载的提示词命令。
// MCP 提示词通过 MCP 协议的 prompts/list 和 prompts/get 接口获取，
// 支持动态参数传递。
type MCPPrompt struct {
	// ID 是命令的唯一标识符，格式为 "mcpServerName:promptName"。
	ID string
	// Title 是命令的显示标题。
	Title string
	// Description 是命令的描述信息。
	Description string
	// PromptID 是 MCP 服务器中提示词的原始名称。
	PromptID string
	// ClientID 是提供该提示词的 MCP 服务器名称。
	ClientID string
	// Arguments 是该提示词支持的参数列表。
	Arguments []Argument
}

// CustomCommand 表示从 Markdown 文件加载的用户自定义命令。
// 命令内容即为 Markdown 文件的原始文本，其中的 $ARG_NAME 占位符
// 会在执行时被替换为实际参数值。
type CustomCommand struct {
	// ID 是命令的唯一标识符，格式为 "前缀:相对路径"（不含 .md 扩展名）。
	// 例如 "user:refactor" 或 "project:tools:lint"。
	ID string
	// Name 是命令的显示名称，当前与 ID 相同。
	Name string
	// Content 是 Markdown 文件的原始内容，包含命令模板文本。
	Content string
	// Arguments 是从内容中提取的参数列表。
	Arguments []Argument
}

// commandSource 描述一个命令文件的加载来源。
type commandSource struct {
	// path 是命令文件所在的目录路径。
	path string
	// prefix 是该来源下命令 ID 的前缀（"user:" 或 "project:"）。
	prefix string
}

// LoadCustomCommands 从多个来源加载用户自定义命令。
//
// 加载顺序（后加载的同名命令不会覆盖先加载的）：
//  1. XDG 配置目录：$XDG_CONFIG_HOME/crush/commands/
//  2. 用户主目录：~/.crush/commands/
//  3. 项目目录：<project>/.crush/commands/
//
// 每个来源目录下的 .md 文件都会被解析为一条自定义命令。
// 目录不存在时会自动创建。
func LoadCustomCommands(cfg *config.Config) ([]CustomCommand, error) {
	return loadAll(buildCommandSources(cfg))
}

// LoadMCPPrompts 从所有已连接的 MCP 服务器加载提示词命令。
//
// 遍历每个 MCP 服务器提供的 prompts 列表，将其转换为统一的 MCPPrompt 结构。
// 命令 ID 格式为 "mcpServerName:promptName"，确保跨服务器的唯一性。
// 如果提示词参数未设置 Title，则回退使用 Name 作为显示名称。
func LoadMCPPrompts() ([]MCPPrompt, error) {
	var commands []MCPPrompt
	for mcpName, prompts := range mcp.Prompts() {
		for _, prompt := range prompts {
			key := mcpName + ":" + prompt.Name
			var args []Argument
			for _, arg := range prompt.Arguments {
				title := arg.Title
				if title == "" {
					title = arg.Name
				}
				args = append(args, Argument{
					ID:          arg.Name,
					Title:       title,
					Description: arg.Description,
					Required:    arg.Required,
				})
			}
			commands = append(commands, MCPPrompt{
				ID:          key,
				Title:       prompt.Title,
				Description: prompt.Description,
				PromptID:    prompt.Name,
				ClientID:    mcpName,
				Arguments:   args,
			})
		}
	}
	return commands, nil
}

// buildCommandSources 构建命令文件的加载来源列表。
// 按优先级从高到低排列：XDG 配置目录 > 用户主目录 > 项目目录。
// 用户级来源使用 "user:" 前缀，项目级来源使用 "project:" 前缀。
func buildCommandSources(cfg *config.Config) []commandSource {
	var sources []commandSource

	// XDG 配置目录：$XDG_CONFIG_HOME/crush/commands/
	if dir := getXDGCommandsDir(); dir != "" {
		sources = append(sources, commandSource{
			path:   dir,
			prefix: userCommandPrefix,
		})
	}

	// 用户主目录：~/.crush/commands/
	if home := home.Dir(); home != "" {
		sources = append(sources, commandSource{
			path:   filepath.Join(home, ".crush", "commands"),
			prefix: userCommandPrefix,
		})
	}

	// 项目目录：<project>/.crush/commands/
	sources = append(sources, commandSource{
		path:   filepath.Join(cfg.Options.DataDirectory, "commands"),
		prefix: projectCommandPrefix,
	})

	return sources
}

// loadAll 从所有来源加载自定义命令并合并为统一列表。
// 单个来源加载失败时静默跳过，不影响其他来源的加载。
func loadAll(sources []commandSource) ([]CustomCommand, error) {
	var commands []CustomCommand

	for _, source := range sources {
		if cmds, err := loadFromSource(source); err == nil {
			commands = append(commands, cmds...)
		}
	}

	return commands, nil
}

// loadFromSource 从指定来源目录递归加载所有 Markdown 命令文件。
// 如果目录不存在，会自动创建。无效的文件会被静默跳过。
func loadFromSource(source commandSource) ([]CustomCommand, error) {
	if err := ensureDir(source.path); err != nil {
		return nil, err
	}

	var commands []CustomCommand

	err := filepath.WalkDir(source.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isMarkdownFile(d.Name()) {
			return err
		}

		cmd, err := loadCommand(path, source.path, source.prefix)
		if err != nil {
			return nil // 跳过无效文件，不中断遍历
		}

		commands = append(commands, cmd)
		return nil
	})

	return commands, err
}

// loadCommand 从指定路径加载单个命令文件。
// 读取文件内容后，根据文件路径生成命令 ID，并从内容中提取参数占位符。
func loadCommand(path, baseDir, prefix string) (CustomCommand, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return CustomCommand{}, err
	}

	id := buildCommandID(path, baseDir, prefix)

	return CustomCommand{
		ID:        id,
		Name:      id,
		Content:   string(content),
		Arguments: extractArgNames(string(content)),
	}, nil
}

// extractArgNames 从命令内容中提取所有命名参数占位符。
// 使用正则匹配 $ARG_NAME 格式的占位符，自动去重并保持首次出现的顺序。
// 自定义命令中提取的参数默认均为必填项。
func extractArgNames(content string) []Argument {
	matches := namedArgPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var args []Argument

	for _, match := range matches {
		arg := match[1]
		if !seen[arg] {
			seen[arg] = true
			// 自定义命令中的所有参数均为必填
			args = append(args, Argument{ID: arg, Title: arg, Required: true})
		}
	}

	return args
}

// buildCommandID 根据文件路径构建命令的唯一标识符。
// 将文件相对于基础目录的路径转换为冒号分隔的 ID，并去除 .md 扩展名。
// 例如：baseDir/tools/lint.md → "prefix:tools:lint"。
func buildCommandID(path, baseDir, prefix string) string {
	relPath, _ := filepath.Rel(baseDir, path)
	parts := strings.Split(relPath, string(filepath.Separator))

	// 移除最后一段的文件扩展名
	if len(parts) > 0 {
		lastIdx := len(parts) - 1
		parts[lastIdx] = strings.TrimSuffix(parts[lastIdx], filepath.Ext(parts[lastIdx]))
	}

	return prefix + strings.Join(parts, ":")
}

// getXDGCommandsDir 获取 XDG 规范下的命令文件目录路径。
// 优先使用 $XDG_CONFIG_HOME 环境变量，若未设置则回退到 ~/.config。
// 返回空字符串表示无法确定 XDG 配置目录。
func getXDGCommandsDir() string {
	xdgHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgHome == "" {
		if home := home.Dir(); home != "" {
			xdgHome = filepath.Join(home, ".config")
		}
	}
	if xdgHome != "" {
		return filepath.Join(xdgHome, "crush", "commands")
	}
	return ""
}

// ensureDir 确保指定目录存在，若不存在则递归创建（权限 0755）。
func ensureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0o755)
	}
	return nil
}

// isMarkdownFile 判断文件名是否为 Markdown 文件（大小写不敏感）。
func isMarkdownFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".md")
}

// GetMCPPrompt 获取指定 MCP 服务器的提示词内容。
// 通过 MCP 协议的 prompts/get 接口获取提示词消息列表，并将多条消息用空格拼接为单一字符串。
//
// 参数：
//   - cfg: 配置存储，用于获取 MCP 服务器连接信息
//   - clientID: MCP 服务器名称
//   - promptID: 提示词名称
//   - args: 提示词参数键值对
func GetMCPPrompt(cfg *config.ConfigStore, clientID, promptID string, args map[string]string) (string, error) {
	// TODO: 应当将 context 从调用方传递下来，而非使用 Background
	result, err := mcp.GetPromptMessages(context.Background(), cfg, clientID, promptID, args)
	if err != nil {
		return "", err
	}
	return strings.Join(result, " "), nil
}
