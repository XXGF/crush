// Package format 提供非交互模式下的 UI 格式化工具。
package format

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/x/ansi"
)

// Spinner 封装了 Bubble Tea 的加载动画，用于非交互模式下的进度指示。
type Spinner struct {
	done chan struct{} // 动画结束信号
	prog *tea.Program // Bubble Tea 程序实例
}

// model 是 Spinner 的内部 Bubble Tea 模型。
type model struct {
	cancel context.CancelFunc // 用于取消上下文的回调
	anim   *anim.Anim         // 动画实例
}

func (m model) Init() tea.Cmd  { return m.anim.Start() }
func (m model) View() tea.View { return tea.NewView(m.anim.Render()) }

// Update 处理 Bubble Tea 消息，支持 Ctrl+C/Esc 取消和动画帧更新。
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancel()
			return m, tea.Quit
		}
	case anim.StepMsg:
		cmd := m.anim.Animate(msg)
		return m, cmd
	}
	return m, nil
}

// NewSpinner 创建一个新的加载动画实例。
//
// 参数：
//   - ctx: 用于控制动画生命周期的上下文
//   - cancel: 用户按下 Ctrl+C 时调用的取消函数
//   - animSettings: 动画配置参数
func NewSpinner(ctx context.Context, cancel context.CancelFunc, animSettings anim.Settings) *Spinner {
	m := model{
		anim:   anim.New(animSettings),
		cancel: cancel,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithContext(ctx))

	return &Spinner{
		prog: p,
		done: make(chan struct{}, 1),
	}
}

// Start 在后台 goroutine 中启动加载动画。
// 动画结束时会清除当前行内容。
func (s *Spinner) Start() {
	go func() {
		defer close(s.done)
		_, err := s.prog.Run()
		// 确保清除当前行
		fmt.Fprint(os.Stderr, ansi.EraseEntireLine)
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, tea.ErrInterrupted) {
			fmt.Fprintf(os.Stderr, "Error running spinner: %v\n", err)
		}
	}()
}

// Stop 停止加载动画并等待其完全退出。
func (s *Spinner) Stop() {
	s.prog.Quit()
	<-s.done
}
