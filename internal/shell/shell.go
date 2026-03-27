// Package shell 提供跨平台的 Shell 命令执行能力。
//
// 每个 Shell 实例拥有独立的工作目录和环境变量，每次执行相互独立。
//
// Windows 兼容性：
// 使用 mvdan.cc/sh/v3 提供 POSIX Shell 仿真，即使在 Windows 上也能正常工作。
// 命令中应使用正斜杠 (/) 作为路径分隔符。
package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/x/exp/slice"
	"mvdan.cc/sh/moreinterp/coreutils"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// ShellType 表示 Shell 的类型。
type ShellType int

const (
	// ShellTypePOSIX 表示 POSIX Shell（默认）。
	ShellTypePOSIX ShellType = iota
	// ShellTypeCmd 表示 Windows CMD。
	ShellTypeCmd
	// ShellTypePowerShell 表示 PowerShell。
	ShellTypePowerShell
)

// Logger 是可选的日志记录接口。
type Logger interface {
	InfoPersist(msg string, keysAndValues ...any)
}

// noopLogger 是一个空操作的日志记录器。
type noopLogger struct{}

func (noopLogger) InfoPersist(msg string, keysAndValues ...any) {}

// BlockFunc 是用于判断命令是否应被阻止执行的函数。
type BlockFunc func(args []string) bool

// Shell 提供跨平台的 Shell 命令执行，支持可选的状态持久化。
type Shell struct {
	env        []string    // 环境变量列表
	cwd        string      // 当前工作目录
	mu         sync.Mutex  // 保护并发访问的互斥锁
	logger     Logger      // 日志记录器
	blockFuncs []BlockFunc // 命令阻止函数列表
}

// Options 是创建 Shell 实例的配置选项。
type Options struct {
	WorkingDir string
	Env        []string
	Logger     Logger
	BlockFuncs []BlockFunc
}

// NewShell 创建一个新的 Shell 实例。
// 如果未指定工作目录，使用当前目录；未指定环境变量，继承父进程环境。
// 自动添加 CRUSH=1、AGENT=crush、AI_AGENT=crush 环境变量。
func NewShell(opts *Options) *Shell {
	if opts == nil {
		opts = &Options{}
	}

	cwd := opts.WorkingDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	env := opts.Env
	if env == nil {
		env = os.Environ()
	}

	// Allow tools to detect execution by Crush.
	env = append(
		env,
		"CRUSH=1",
		"AGENT=crush",
		"AI_AGENT=crush",
	)

	logger := opts.Logger
	if logger == nil {
		logger = noopLogger{}
	}

	return &Shell{
		cwd:        cwd,
		env:        env,
		logger:     logger,
		blockFuncs: opts.BlockFuncs,
	}
}

// Exec 执行 Shell 命令并返回标准输出和标准错误。
func (s *Shell) Exec(ctx context.Context, command string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.exec(ctx, command)
}

// ExecStream 执行 Shell 命令并将输出流式写入指定的 Writer。
func (s *Shell) ExecStream(ctx context.Context, command string, stdout, stderr io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.execStream(ctx, command, stdout, stderr)
}

// GetWorkingDir 返回当前工作目录。
func (s *Shell) GetWorkingDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}

// SetWorkingDir 设置工作目录，目录不存在时返回错误。
func (s *Shell) SetWorkingDir(dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证目录是否存在
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("directory does not exist: %w", err)
	}

	s.cwd = dir
	return nil
}

// GetEnv 返回环境变量的副本。
func (s *Shell) GetEnv() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	env := make([]string, len(s.env))
	copy(env, s.env)
	return env
}

// SetEnv 设置环境变量，已存在则更新，不存在则添加。
func (s *Shell) SetEnv(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新或添加环境变量
	keyPrefix := key + "="
	for i, env := range s.env {
		if strings.HasPrefix(env, keyPrefix) {
			s.env[i] = keyPrefix + value
			return
		}
	}
	s.env = append(s.env, keyPrefix+value)
}

// SetBlockFuncs sets the command block functions for the shell
func (s *Shell) SetBlockFuncs(blockFuncs []BlockFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockFuncs = blockFuncs
}

// CommandsBlocker creates a BlockFunc that blocks exact command matches
func CommandsBlocker(cmds []string) BlockFunc {
	bannedSet := make(map[string]struct{})
	for _, cmd := range cmds {
		bannedSet[cmd] = struct{}{}
	}

	return func(args []string) bool {
		if len(args) == 0 {
			return false
		}
		_, ok := bannedSet[args[0]]
		return ok
	}
}

// ArgumentsBlocker creates a BlockFunc that blocks specific subcommand
func ArgumentsBlocker(cmd string, args []string, flags []string) BlockFunc {
	return func(parts []string) bool {
		if len(parts) == 0 || parts[0] != cmd {
			return false
		}

		argParts, flagParts := splitArgsFlags(parts[1:])
		if len(argParts) < len(args) || len(flagParts) < len(flags) {
			return false
		}

		argsMatch := slices.Equal(argParts[:len(args)], args)
		flagsMatch := slice.IsSubset(flags, flagParts)

		return argsMatch && flagsMatch
	}
}

func splitArgsFlags(parts []string) (args []string, flags []string) {
	args = make([]string, 0, len(parts))
	flags = make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.HasPrefix(part, "-") {
			// Extract flag name before '=' if present
			flag := part
			if before, _, ok := strings.Cut(part, "="); ok {
				flag = before
			}
			flags = append(flags, flag)
		} else {
			args = append(args, part)
		}
	}
	return args, flags
}

func (s *Shell) blockHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}

			for _, blockFunc := range s.blockFuncs {
				if blockFunc(args) {
					return fmt.Errorf("command is not allowed for security reasons: %q", args[0])
				}
			}

			return next(ctx, args)
		}
	}
}

// newInterp creates a new interpreter with the current shell state
func (s *Shell) newInterp(stdout, stderr io.Writer) (*interp.Runner, error) {
	return interp.New(
		interp.StdIO(nil, stdout, stderr),
		interp.Interactive(false),
		interp.Env(expand.ListEnviron(s.env...)),
		interp.Dir(s.cwd),
		interp.ExecHandlers(s.execHandlers()...),
	)
}

// updateShellFromRunner updates the shell from the interpreter after execution.
func (s *Shell) updateShellFromRunner(runner *interp.Runner) {
	s.cwd = runner.Dir
	s.env = s.env[:0]
	for name, vr := range runner.Vars {
		if vr.Exported {
			s.env = append(s.env, name+"="+vr.Str)
		}
	}
}

// execCommon is the shared implementation for executing commands
func (s *Shell) execCommon(ctx context.Context, command string, stdout, stderr io.Writer) (err error) {
	var runner *interp.Runner
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("command execution panic: %v", r)
		}
		if runner != nil {
			s.updateShellFromRunner(runner)
		}
		s.logger.InfoPersist("command finished", "command", command, "err", err)
	}()

	line, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return fmt.Errorf("could not parse command: %w", err)
	}

	runner, err = s.newInterp(stdout, stderr)
	if err != nil {
		return fmt.Errorf("could not run command: %w", err)
	}

	err = runner.Run(ctx, line)
	return err
}

// exec executes commands using a cross-platform shell interpreter.
func (s *Shell) exec(ctx context.Context, command string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	err := s.execCommon(ctx, command, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// execStream executes commands using POSIX shell emulation with streaming output
func (s *Shell) execStream(ctx context.Context, command string, stdout, stderr io.Writer) error {
	return s.execCommon(ctx, command, stdout, stderr)
}

func (s *Shell) execHandlers() []func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	handlers := []func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc{
		s.blockHandler(),
	}
	if useGoCoreUtils {
		handlers = append(handlers, coreutils.ExecHandler)
	}
	return handlers
}

// IsInterrupt checks if an error is due to interruption
func IsInterrupt(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

// ExitCode extracts the exit code from an error
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr interp.ExitStatus
	if errors.As(err, &exitErr) {
		return int(exitErr)
	}
	return 1
}
