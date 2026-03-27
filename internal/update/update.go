// Package update 提供应用程序版本更新检查功能。
//
// 通过 GitHub Releases API 获取最新版本信息，并与当前版本进行比较，
// 支持稳定版和预发布版的智能判断。
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	// githubApiUrl 是 GitHub Releases API 的端点地址。
	githubApiUrl = "https://api.github.com/repos/charmbracelet/crush/releases/latest"
	// userAgent 是 HTTP 请求中使用的 User-Agent 头。
	userAgent = "crush/1.0"
)

// Default 是默认的更新检查客户端，使用 GitHub API。
var Default Client = &github{}

// Info 包含版本更新的相关信息。
type Info struct {
	Current string // 当前版本号
	Latest  string // 最新版本号
	URL     string // 最新版本的下载页面 URL
}

// goInstallRegexp 匹配通过 `go install` 安装时生成的版本号格式。
// 示例：v0.0.0-0.20251231235959-06c807842604
var goInstallRegexp = regexp.MustCompile(`^v?\d+\.\d+\.\d+-\d+\.\d{14}-[0-9a-f]{12}$`)

// IsDevelopment 判断当前版本是否为开发版本。
// 开发版本包括："devel"、"unknown"、包含 "dirty" 的版本以及 go install 生成的版本号。
func (i Info) IsDevelopment() bool {
	return i.Current == "devel" || i.Current == "unknown" || strings.Contains(i.Current, "dirty") || goInstallRegexp.MatchString(i.Current)
}

// Available 判断是否有可用的更新版本。
//
// 判断规则：
//   - 当前为预发布版且最新为稳定版：返回 true
//   - 最新为预发布版且当前为稳定版：返回 false
//   - 其他情况：版本号不同则返回 true
func (i Info) Available() bool {
	cpr := strings.Contains(i.Current, "-")
	lpr := strings.Contains(i.Latest, "-")
	// current is pre release && latest isn't a prerelease
	if cpr && !lpr {
		return true
	}
	// latest is pre release && current isn't a prerelease
	if lpr && !cpr {
		return false
	}
	return i.Current != i.Latest
}

// Check 检查是否有新版本可用。
// 通过指定的 Client 获取最新版本信息，并与当前版本进行比较。
func Check(ctx context.Context, current string, client Client) (Info, error) {
	info := Info{
		Current: current,
		Latest:  current,
	}

	release, err := client.Latest(ctx)
	if err != nil {
		return info, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	info.Latest = strings.TrimPrefix(release.TagName, "v")
	info.Current = strings.TrimPrefix(info.Current, "v")
	info.URL = release.HTMLURL
	return info, nil
}

// Release 表示一个 GitHub 发布版本。
type Release struct {
	TagName string `json:"tag_name"` // 版本标签名，如 "v1.0.0"
	HTMLURL string `json:"html_url"` // 发布页面的 URL
}

// Client 定义了获取最新版本信息的客户端接口。
type Client interface {
	// Latest 获取最新的发布版本信息。
	Latest(ctx context.Context) (*Release, error)
}

// github 是基于 GitHub API 的 Client 实现。
type github struct{}

// Latest 通过 GitHub Releases API 获取最新的发布版本。
// 请求超时时间为 30 秒。
func (c *github) Latest(ctx context.Context) (*Release, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", githubApiUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}
