package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const DefaultRedeemBaseURL = "https://api.worker.openwatcher.ai"
const TokenExpiredUserMessage = "远程访问隧道已失效，请联系管理员重新绑定设备。"

var ErrBinaryNotFound = errors.New("cloudflared binary not found")

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type RedeemError struct {
	Code       string
	StatusCode int
	Message    string
}

func (e *RedeemError) Error() string {
	return e.Message
}

func NewClient() *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENWATCHER_MANAGED_TUNNEL_API_BASE")), "/")
	if baseURL == "" {
		baseURL = DefaultRedeemBaseURL
	}
	return NewClientWithBaseURL(baseURL, nil)
}

func NewClientWithBaseURL(baseURL string, httpClient *http.Client) *Client {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBaseURL == "" {
		trimmedBaseURL = DefaultRedeemBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{
		baseURL:    trimmedBaseURL,
		httpClient: httpClient,
	}
}

func (c *Client) Redeem(ctx context.Context, code string, identity Identity, desktopVersion string) (RedeemResponse, error) {
	requestBody := map[string]string{
		"code":                   strings.TrimSpace(code),
		"installId":              strings.TrimSpace(identity.InstallID),
		"machineFingerprintHash": strings.TrimSpace(identity.MachineFingerprintHash),
		"desktopVersion":         strings.TrimSpace(desktopVersion),
		"platform":               runtime.GOOS + "-" + runtime.GOARCH,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return RedeemResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/tunnel/redeem", bytes.NewReader(body))
	if err != nil {
		return RedeemResponse{}, fmt.Errorf("构造兑换请求失败")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RedeemResponse{}, fmt.Errorf("无法连接兑换服务，请检查网络后重试")
	}
	defer resp.Body.Close()

	payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if readErr != nil {
		return RedeemResponse{}, fmt.Errorf("读取兑换服务响应失败")
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result struct {
			OK bool `json:"ok"`
			RedeemResponse
		}
		if err := json.Unmarshal(payload, &result); err != nil {
			return RedeemResponse{}, fmt.Errorf("兑换服务返回了无法解析的响应")
		}
		if !result.OK {
			return RedeemResponse{}, fmt.Errorf("兑换服务未返回成功状态")
		}
		if strings.TrimSpace(result.PublicBaseURL) == "" || !hasCredentialMaterial(result.RedeemResponse) {
			return RedeemResponse{}, fmt.Errorf("兑换服务返回的数据不完整")
		}
		return result.RedeemResponse, nil
	}

	var appError struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload, &appError); err != nil {
		return RedeemResponse{}, &RedeemError{
			Code:       "internal_error",
			StatusCode: resp.StatusCode,
			Message:    "兑换服务暂时不可用，请稍后重试",
		}
	}
	return RedeemResponse{}, &RedeemError{
		Code:       strings.TrimSpace(appError.Code),
		StatusCode: resp.StatusCode,
		Message:    friendlyRedeemMessage(strings.TrimSpace(appError.Code), strings.TrimSpace(appError.Message)),
	}
}

func (c *Client) SubmitWatchBootstrapConfig(ctx context.Context, request WatchBootstrapConfigRequest) (WatchBootstrapConfigResponse, error) {
	requestBody := WatchBootstrapConfigRequest{
		BootstrapCode: strings.TrimSpace(request.BootstrapCode),
		Environment:   strings.TrimSpace(request.Environment),
		APIBase:       strings.TrimRight(strings.TrimSpace(request.APIBase), "/"),
		Source:        strings.TrimSpace(request.Source),
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return WatchBootstrapConfigResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/watch-bootstrap/config", bytes.NewReader(body))
	if err != nil {
		return WatchBootstrapConfigResponse{}, fmt.Errorf("构造临时配置提交请求失败")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return WatchBootstrapConfigResponse{}, fmt.Errorf("无法连接临时配置服务，请检查网络后重试")
	}
	defer resp.Body.Close()

	payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if readErr != nil {
		return WatchBootstrapConfigResponse{}, fmt.Errorf("读取临时配置服务响应失败")
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result struct {
			OK bool `json:"ok"`
			WatchBootstrapConfigResponse
		}
		if err := json.Unmarshal(payload, &result); err != nil {
			return WatchBootstrapConfigResponse{}, fmt.Errorf("临时配置服务返回了无法解析的响应")
		}
		if !result.OK || strings.TrimSpace(result.Config.APIBase) == "" {
			return WatchBootstrapConfigResponse{}, fmt.Errorf("临时配置服务返回的数据不完整")
		}
		return result.WatchBootstrapConfigResponse, nil
	}

	var appError struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload, &appError); err != nil {
		return WatchBootstrapConfigResponse{}, &RedeemError{
			Code:       "internal_error",
			StatusCode: resp.StatusCode,
			Message:    "临时配置服务暂时不可用，请稍后重试",
		}
	}
	return WatchBootstrapConfigResponse{}, &RedeemError{
		Code:       strings.TrimSpace(appError.Code),
		StatusCode: resp.StatusCode,
		Message:    friendlyRedeemMessage(strings.TrimSpace(appError.Code), strings.TrimSpace(appError.Message)),
	}
}

func hasCredentialMaterial(response RedeemResponse) bool {
	if strings.TrimSpace(response.TunnelToken) != "" {
		return true
	}
	if response.TunnelCredentials == nil {
		return false
	}
	return strings.TrimSpace(response.TunnelCredentials.AccountTag) != "" &&
		strings.TrimSpace(response.TunnelCredentials.TunnelID) != "" &&
		strings.TrimSpace(response.TunnelCredentials.TunnelSecret) != ""
}

func friendlyRedeemMessage(code string, fallback string) string {
	switch strings.TrimSpace(code) {
	case "invalid_code":
		return "配置码无效，请检查输入内容后重试。"
	case "code_expired":
		return "配置码已过期，请联系管理员重新发码。"
	case "code_already_redeemed":
		return "该配置码已经被其他设备使用，请联系管理员重新绑定。"
	case "device_already_bound":
		return "当前账号已绑定其他设备，请联系管理员重新绑定。"
	case "user_blocked":
		return "当前账号暂时无法使用托管隧道，请联系管理员。"
	case "tunnel_unavailable":
		return "远程隧道暂时不可用，请稍后重试或联系管理员。"
	case "internal_error", "cloudflare_request_failed", "cloudflare_invalid_response":
		return "兑换服务暂时不可用，请稍后重试。"
	case "invalid_bootstrap_code":
		return "临时配置码无效，请检查手表上的 8 位代码。"
	case "bootstrap_code_expired":
		return "临时配置码已失效，请在手表上重新获取。"
	case "bootstrap_code_consumed":
		return "临时配置码已被手表取走，请在手表上重新获取。"
	default:
		if strings.TrimSpace(fallback) != "" {
			return fallback
		}
		return "兑换失败，请稍后重试。"
	}
}
