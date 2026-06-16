package installer

type WizardStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func DefaultSteps() []WizardStep {
	return []WizardStep{
		{
			ID:          "same-wifi",
			Title:       "确认同一 Wi-Fi",
			Description: "后续向导会指导手表与电脑进入相同网络。",
		},
		{
			ID:          "wireless-debugging",
			Title:       "开启无线调试",
			Description: "后续阶段接入 ADB pair / connect / install。",
		},
		{
			ID:          "bootstrap",
			Title:       "写入 OpenWatcher 配置",
			Description: "后续阶段会通过 desktop bootstrap 发送 baseUrl 和 deviceToken，请在手表上确认保存。",
		},
	}
}
