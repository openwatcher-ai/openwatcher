package logging

import (
	"os"
	"regexp"
	"strings"
)

type Redactor struct {
	patterns []redactionPattern
}

type redactionPattern struct {
	replacement string
	regex       *regexp.Regexp
}

func NewRedactor() *Redactor {
	homeDir, _ := os.UserHomeDir()
	return &Redactor{
		patterns: []redactionPattern{
			{
				regex:       regexp.MustCompile(`(?i)(authorization\s*[:=]\s*bearer\s+)([^\s",]+)`),
				replacement: `${1}[REDACTED]`,
			},
			{
				regex:       regexp.MustCompile(`(?i)(("?(?:deviceToken|token|tokenHash|access_token|accessToken|tunnelToken|tunnelSecret|TunnelSecret|cloudflare_tunnel_secret|pairingCode|pairing code|inviteCode|configCode|tunnelCode|redeemCode|配置码)"?\s*[:=]\s*"?))([^"\s,]+)`),
				replacement: `${2}[REDACTED]`,
			},
			{
				regex:       regexp.MustCompile(`(?i)(pairing\s*code\s*[=:]\s*)([^\s",]+)`),
				replacement: `${1}[REDACTED]`,
			},
			pathPattern(homeDir),
		},
	}
}

func (r *Redactor) RedactLine(line string) string {
	redacted := line
	for _, pattern := range r.patterns {
		redacted = pattern.regex.ReplaceAllString(redacted, pattern.replacement)
	}
	return redacted
}

func pathPattern(homeDir string) redactionPattern {
	trimmed := strings.TrimSpace(homeDir)
	if trimmed == "" {
		return redactionPattern{
			regex:       regexp.MustCompile(`$^`),
			replacement: "",
		}
	}
	return redactionPattern{
		regex:       regexp.MustCompile(regexp.QuoteMeta(trimmed)),
		replacement: "~",
	}
}
