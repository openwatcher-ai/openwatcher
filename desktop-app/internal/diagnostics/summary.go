package diagnostics

import (
	"encoding/json"

	"openwatcher/desktop-app/internal/logging"
)

type Builder struct {
	redactor *logging.Redactor
}

func NewBuilder(redactor *logging.Redactor) *Builder {
	return &Builder{redactor: redactor}
}

func (b *Builder) Build(payload any) string {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "诊断信息生成失败"
	}
	return b.redactor.RedactLine(string(data))
}
