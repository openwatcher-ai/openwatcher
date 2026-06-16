package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func ProbePublicHealth(ctx context.Context, baseURL string, targetLabel string) (HealthStatus, error) {
	trimmedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	endpoint := trimmedBase + "/healthz"
	label := strings.TrimSpace(targetLabel)
	if label == "" {
		label = "公网地址"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return HealthStatus{
			OK:       false,
			Endpoint: endpoint,
			Message:  fmt.Sprintf("构造%s /healthz 请求失败", label),
		}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return HealthStatus{
			OK:       false,
			Endpoint: endpoint,
			Message:  fmt.Sprintf("无法连接%s /healthz", label),
		}, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if readErr != nil {
		return HealthStatus{
			OK:       false,
			Endpoint: endpoint,
			Message:  fmt.Sprintf("读取%s /healthz 响应失败", label),
		}, readErr
	}

	var payload struct {
		OK     bool        `json:"ok"`
		Build  BuildInfo   `json:"build"`
		Config RuntimeInfo `json:"config"`
		Codex  CodexInfo   `json:"codex"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return HealthStatus{
			OK:       false,
			HTTPCode: resp.StatusCode,
			Endpoint: endpoint,
			Message:  "/healthz 返回无法解析",
			RawBody:  strings.TrimSpace(string(body)),
		}, err
	}

	return HealthStatus{
		OK:       payload.OK && resp.StatusCode >= 200 && resp.StatusCode < 300,
		Message:  fmt.Sprintf("HTTP %d", resp.StatusCode),
		HTTPCode: resp.StatusCode,
		Endpoint: endpoint,
		Build:    payload.Build,
		Config:   payload.Config,
		Codex:    payload.Codex,
		RawBody:  strings.TrimSpace(string(body)),
	}, nil
}
