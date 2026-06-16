package codex

import (
	"os"
	"path/filepath"

	rootconfig "openwatcher/internal/config"
)

type Detector struct{}

type Status struct {
	HomeLabel        string `json:"homeLabel"`
	HomeDetected     bool   `json:"homeDetected"`
	AuthDetected     bool   `json:"authDetected"`
	SessionsDetected bool   `json:"sessionsDetected"`
	Readable         bool   `json:"readable"`
	Message          string `json:"message"`
}

func NewDetector() *Detector {
	return &Detector{}
}

func (d *Detector) Inspect() Status {
	resolvedHome, err := rootconfig.ResolveCodexHome("")
	if err != nil {
		return Status{
			HomeLabel: "~/.codex",
			Message:   "无法解析 Codex 目录",
		}
	}

	authPath := filepath.Join(resolvedHome, "auth.json")
	sessionsPath := filepath.Join(resolvedHome, "sessions")

	authInfo, authErr := os.Stat(authPath)
	sessionsInfo, sessionsErr := os.Stat(sessionsPath)

	readable := authErr == nil && sessionsErr == nil && !authInfo.IsDir() && sessionsInfo.IsDir()
	status := Status{
		HomeLabel:        homeLabel(),
		HomeDetected:     true,
		AuthDetected:     authErr == nil && !authInfo.IsDir(),
		SessionsDetected: sessionsErr == nil && sessionsInfo.IsDir(),
		Readable:         readable,
	}
	switch {
	case !status.AuthDetected && !status.SessionsDetected:
		status.Message = "未发现 auth.json 和 sessions，可能尚未登录 Codex。"
	case !status.AuthDetected:
		status.Message = "找到了 sessions，但没有找到 auth.json。"
	case !status.SessionsDetected:
		status.Message = "找到了 auth.json，但没有找到 sessions。"
	case !status.Readable:
		status.Message = "Codex 目录可见，但当前状态不可读。"
	default:
		status.Message = "Codex 目录状态正常，可继续接入后端检测。"
	}
	return status
}

func homeLabel() string {
	if _, ok := os.LookupEnv("CODEX_HOME"); ok {
		return "CODEX_HOME"
	}
	return "~/.codex"
}
