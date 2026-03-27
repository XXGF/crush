// Package projects 提供项目目录的跟踪和管理功能。
//
// 将用户访问过的项目目录持久化到 projects.json 文件中，
// 支持按最近访问时间排序。
package projects

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

// projectsFileName 是项目列表的存储文件名。
const projectsFileName = "projects.json"

// Project 表示一个被跟踪的项目目录。
type Project struct {
	Path         string    `json:"path"`          // 项目的工作目录路径
	DataDir      string    `json:"data_dir"`      // 项目的数据存储目录
	LastAccessed time.Time `json:"last_accessed"` // 最后访问时间
}

// ProjectList 保存所有被跟踪的项目列表。
type ProjectList struct {
	Projects []Project `json:"projects"`
}

// mu 保护文件读写操作的互斥锁。
var mu sync.Mutex

// projectsFilePath 返回 projects.json 文件的完整路径。
func projectsFilePath() string {
	return filepath.Join(filepath.Dir(config.GlobalConfigData()), projectsFileName)
}

// Load 从磁盘读取项目列表。
// 如果文件不存在，返回空列表。
func Load() (*ProjectList, error) {
	mu.Lock()
	defer mu.Unlock()

	path := projectsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectList{Projects: []Project{}}, nil
		}
		return nil, err
	}

	var list ProjectList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return &list, nil
}

// Save 将项目列表写入磁盘。
// 自动创建必要的目录结构，文件权限为 0600。
func Save(list *ProjectList) error {
	mu.Lock()
	defer mu.Unlock()

	path := projectsFilePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// Register 注册或更新项目列表中的项目。
// 如果项目已存在，更新其数据目录和访问时间；否则添加新项目。
// 列表按最近访问时间降序排列。
func Register(workingDir, dataDir string) error {
	list, err := Load()
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	// Check if project already exists
	found := false
	for i, p := range list.Projects {
		if p.Path == workingDir {
			list.Projects[i].DataDir = dataDir
			list.Projects[i].LastAccessed = now
			found = true
			break
		}
	}

	if !found {
		list.Projects = append(list.Projects, Project{
			Path:         workingDir,
			DataDir:      dataDir,
			LastAccessed: now,
		})
	}

	// Sort by last accessed (most recent first)
	slices.SortFunc(list.Projects, func(a, b Project) int {
		if a.LastAccessed.After(b.LastAccessed) {
			return -1
		}
		if a.LastAccessed.Before(b.LastAccessed) {
			return 1
		}
		return 0
	})

	return Save(list)
}

// List 返回所有被跟踪的项目，按最近访问时间降序排列。
func List() ([]Project, error) {
	list, err := Load()
	if err != nil {
		return nil, err
	}
	return list.Projects, nil
}
