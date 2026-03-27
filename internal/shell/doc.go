// Package shell 提供 Shell 命令执行环境的管理功能。
//
// 支持跨命令的状态保持（环境变量、工作目录），
// 以及后台命令执行和超时控制。
//
// 示例用法：
//
// 1. 执行单次命令：
//
//	shell := shell.NewShell(nil)
//	stdout, stderr, err := shell.Exec(context.Background(), "echo hello")
//
// 2. 跨命令保持状态：
//
//	shell := shell.NewShell(&shell.Options{
//	    WorkingDir: "/tmp",
//	    Logger: myLogger,
//	})
//	shell.Exec(ctx, "export FOO=bar")
//	shell.Exec(ctx, "echo $FOO")  // 输出 "bar"
//
// 3. 管理环境变量和工作目录：
//
//	shell := shell.NewShell(nil)
//	shell.SetEnv("MY_VAR", "value")
//	shell.SetWorkingDir("/tmp")
//	cwd := shell.GetWorkingDir()
//	env := shell.GetEnv()
