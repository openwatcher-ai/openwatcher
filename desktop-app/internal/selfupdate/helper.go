package selfupdate

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type HelperOptions struct {
	SourceDir  string
	TargetPath string
	LaunchPath string
	StatusPath string
	BackupRoot string
	Platform   string
	Version    string
	Artifact   string
}

func RunHelper(options HelperOptions) error {
	options.Platform = strings.TrimSpace(options.Platform)
	if options.Platform == "" {
		options.Platform = runtime.GOOS
	}
	if err := validateOptions(options); err != nil {
		return err
	}
	_ = WriteStatus(options.StatusPath, NewStatus("installing", "正在准备替换 Desktop", options.Version, options.Artifact, ""))
	time.Sleep(1500 * time.Millisecond)

	backupPath, err := replaceWithBackup(options)
	if err != nil {
		_ = WriteStatus(options.StatusPath, NewStatus("failed", err.Error(), options.Version, options.Artifact, backupPath))
		return err
	}
	if options.Platform == "darwin" {
		_ = exec.Command("xattr", "-cr", options.TargetPath).Run()
	}
	if err := launchUpdatedApp(options); err != nil {
		message := fmt.Sprintf("更新已安装，但启动新版失败：%v", err)
		_ = WriteStatus(options.StatusPath, NewStatus("failed", message, options.Version, options.Artifact, backupPath))
		return errors.New(message)
	}
	_ = WriteStatus(options.StatusPath, NewStatus("installed", "Desktop 已更新并重新启动", options.Version, options.Artifact, backupPath))
	return nil
}

func validateOptions(options HelperOptions) error {
	if strings.TrimSpace(options.SourceDir) == "" {
		return errors.New("缺少新版应用目录")
	}
	if strings.TrimSpace(options.TargetPath) == "" {
		return errors.New("缺少当前应用路径")
	}
	if strings.TrimSpace(options.LaunchPath) == "" {
		return errors.New("缺少启动路径")
	}
	if strings.TrimSpace(options.StatusPath) == "" {
		return errors.New("缺少更新状态路径")
	}
	if strings.TrimSpace(options.BackupRoot) == "" {
		return errors.New("缺少备份目录")
	}
	if info, err := os.Stat(options.SourceDir); err != nil || !info.IsDir() {
		return fmt.Errorf("新版应用目录不可用：%s", options.SourceDir)
	}
	return nil
}

func replaceWithBackup(options HelperOptions) (string, error) {
	if err := os.MkdirAll(options.BackupRoot, 0o755); err != nil {
		return "", err
	}
	backupPath := filepath.Join(options.BackupRoot, filepath.Base(options.TargetPath)+"-"+time.Now().UTC().Format("20060102-150405"))
	var renameErr error
	deadline := time.Now().Add(90 * time.Second)
	for {
		renameErr = os.Rename(options.TargetPath, backupPath)
		if renameErr == nil {
			break
		}
		if os.IsNotExist(renameErr) {
			backupPath = ""
			break
		}
		if time.Now().After(deadline) {
			return backupPath, fmt.Errorf("替换前备份当前应用失败：%w", renameErr)
		}
		time.Sleep(time.Second)
	}

	if err := copyTree(options.SourceDir, options.TargetPath); err != nil {
		_ = os.RemoveAll(options.TargetPath)
		if backupPath != "" {
			_ = os.Rename(backupPath, options.TargetPath)
		}
		return backupPath, fmt.Errorf("复制新版应用失败：%w", err)
	}
	return backupPath, nil
}

func copyTree(sourceRoot string, targetRoot string) error {
	sourceRoot = filepath.Clean(sourceRoot)
	targetRoot = filepath.Clean(targetRoot)
	return filepath.WalkDir(sourceRoot, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(targetRoot, 0o755)
		}
		targetPath := filepath.Join(targetRoot, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(sourcePath)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}
		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(sourcePath, targetPath, info.Mode().Perm())
	})
}

func copyFile(sourcePath string, targetPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	return os.Chmod(targetPath, mode)
}

func launchUpdatedApp(options HelperOptions) error {
	if options.Platform == "darwin" {
		return exec.Command("open", "-n", options.TargetPath).Start()
	}
	return exec.Command(options.LaunchPath).Start()
}
