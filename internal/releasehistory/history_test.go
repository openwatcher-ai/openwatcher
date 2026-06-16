package releasehistory

import (
	"strings"
	"testing"
	"time"
)

func TestParseMarkdownSupportsLegacyAndUserRows(t *testing.T) {
	input := `# 手表 APK 构建记录

| 构建时间 UTC | versionName | versionCode | Git commit | 构建分支 | APK 文件 | SHA256 | 说明类型 | 变更摘要 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-06-06T11:46:01Z | 0.14.0 | 50 | 557f45a | main | dist/a.apk | sha-a | 递增手表 release 与生产后端版本 |
| 2026-06-06T11:55:40Z | 0.15.0 | 51 | 3f67eba | main | dist/b.apk | sha-b | user | 下载进度更清楚 |
`
	entries, err := ParseMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMarkdown error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if got := entries[0].SummaryType; got != SummaryTypeLegacy {
		t.Fatalf("legacy summary type = %q", got)
	}
	if got := entries[1].SummaryType; got != SummaryTypeUser {
		t.Fatalf("user summary type = %q", got)
	}
}

func TestBuildUserChangelogKeepsLatestDuplicateVersionAndFiltersLegacy(t *testing.T) {
	entries := []Entry{
		{
			BuiltAtUTC:  time.Date(2026, 6, 6, 11, 46, 1, 0, time.UTC),
			VersionName: "0.14.0",
			VersionCode: 50,
			Summary:     "旧说明",
			SummaryType: SummaryTypeLegacy,
		},
		{
			BuiltAtUTC:  time.Date(2026, 6, 6, 11, 55, 40, 0, time.UTC),
			VersionName: "0.15.0",
			VersionCode: 51,
			Summary:     "第一条",
			SummaryType: SummaryTypeUser,
		},
		{
			BuiltAtUTC:  time.Date(2026, 6, 6, 11, 56, 0, 0, time.UTC),
			VersionName: "0.15.0",
			VersionCode: 51,
			Summary:     "最终说明",
			SummaryType: SummaryTypeUser,
		},
	}
	payload := BuildUserChangelog(entries)
	if len(payload) != 1 {
		t.Fatalf("len(payload) = %d, want 1", len(payload))
	}
	if got := payload[0].Summary; got != "最终说明" {
		t.Fatalf("summary = %q", got)
	}
	if got := payload[0].PublishedAtBeijing; got != "2026-06-06 19:56" {
		t.Fatalf("publishedAtBeijing = %q", got)
	}
}
