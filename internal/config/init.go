package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/fsext"
)

const (
	// InitFlagFilename 是项目初始化标志文件的名称。
	InitFlagFilename = "init"
)

// ProjectInitFlag 表示项目初始化状态。
type ProjectInitFlag struct {
	Initialized bool `json:"initialized"`
}

// Init 加载配置并初始化配置存储。
func Init(workingDir, dataDir string, debug bool) (*ConfigStore, error) {
	store, err := Load(workingDir, dataDir, debug)
	if err != nil {
		return nil, err
	}
	return store, nil
}

// ProjectNeedsInitialization 检查项目是否需要初始化。
//
// 以下情况无需初始化：
//   - 已存在初始化标志文件
//   - 已存在上下文文件（如 SKILL.md、.cursorrules 等）
//   - 工作目录为空
func ProjectNeedsInitialization(store *ConfigStore) (bool, error) {
	if store == nil {
		return false, fmt.Errorf("config not loaded")
	}

	cfg := store.Config()
	flagFilePath := filepath.Join(cfg.Options.DataDirectory, InitFlagFilename)

	_, err := os.Stat(flagFilePath)
	if err == nil {
		return false, nil
	}

	if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check init flag file: %w", err)
	}

	someContextFileExists, err := contextPathsExist(store.WorkingDir())
	if err != nil {
		return false, fmt.Errorf("failed to check for context files: %w", err)
	}
	if someContextFileExists {
		return false, nil
	}

	// If the working directory has no non-ignored files, skip initialization step
	empty, err := dirHasNoVisibleFiles(store.WorkingDir())
	if err != nil {
		return false, fmt.Errorf("failed to check if directory is empty: %w", err)
	}
	if empty {
		return false, nil
	}

	return true, nil
}

// contextPathsExist 检查目录中是否存在默认的上下文文件（大小写不敏感）。
func contextPathsExist(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	// Create a slice of lowercase filenames for lookup with slices.Contains
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, strings.ToLower(entry.Name()))
		}
	}

	// Check if any of the default context paths exist in the directory
	for _, path := range defaultContextPaths {
		// Extract just the filename from the path
		_, filename := filepath.Split(path)
		filename = strings.ToLower(filename)

		if slices.Contains(files, filename) {
			return true, nil
		}
	}

	return false, nil
}

// dirHasNoVisibleFiles 检查目录在应用忽略规则后是否为空。
func dirHasNoVisibleFiles(dir string) (bool, error) {
	files, _, err := fsext.ListDirectory(dir, nil, 1, 1)
	if err != nil {
		return false, err
	}
	return len(files) == 0, nil
}

// MarkProjectInitialized 创建初始化标志文件，标记项目已完成初始化。
func MarkProjectInitialized(store *ConfigStore) error {
	if store == nil {
		return fmt.Errorf("config not loaded")
	}
	flagFilePath := filepath.Join(store.Config().Options.DataDirectory, InitFlagFilename)

	file, err := os.Create(flagFilePath)
	if err != nil {
		return fmt.Errorf("failed to create init flag file: %w", err)
	}
	defer file.Close()

	return nil
}

// HasInitialDataConfig 检查是否存在全局数据配置文件且已完成配置。
func HasInitialDataConfig(store *ConfigStore) bool {
	if store == nil {
		return false
	}
	cfgPath := GlobalConfigData()
	if _, err := os.Stat(cfgPath); err != nil {
		return false
	}
	return store.Config().IsConfigured()
}
