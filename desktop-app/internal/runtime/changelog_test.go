package runtime

import "testing"

func TestParseChangelogFeedAndFilterDesktopEntries(t *testing.T) {
	payload := []byte(`{
		"schemaVersion": 1,
		"channel": "beta",
		"updatedAt": "2026-06-11T14:00:00Z",
		"entries": [
			{
				"id": "beta-2026.06.11.3",
				"publishedAt": "2026-06-11T14:00:00Z",
				"scope": ["watch"],
				"components": {
					"desktop": { "status": "reused" },
					"watch": { "status": "updated" },
					"runtime": { "status": "reused" },
					"compatibility": { "status": "not_included" },
					"docs": { "status": "not_included" }
				},
				"notes": {
					"features": [],
					"improvements": [],
					"fixes": [
						{ "component": "手表应用", "text": "修复手表安装提示。" }
					],
					"compatibility": []
				}
			},
			{
				"id": "beta-2026.06.11.2",
				"publishedAt": "2026-06-11T13:00:00Z",
				"scope": ["desktop", "compatibility"],
				"components": {
					"desktop": { "status": "updated" },
					"watch": { "status": "reused" },
					"runtime": { "status": "reused" },
					"compatibility": { "status": "updated" },
					"docs": { "status": "not_included" }
				},
				"notes": {
					"features": [],
					"improvements": [],
					"fixes": [
						{ "component": "桌面应用", "text": "修复 macOS Desktop bundle id 仍使用默认值的问题。" }
					],
					"compatibility": [
						{ "component": "兼容性", "text": "本次未更新手表应用，不会触发手表端版本更新。" }
					]
				}
			},
			{
				"id": "beta-2026.06.11.1",
				"publishedAt": "2026-06-11T12:00:00Z",
				"scope": ["runtime-pointer"],
				"components": {
					"desktop": { "status": "reused" },
					"watch": { "status": "reused" },
					"runtime": { "status": "updated" },
					"compatibility": { "status": "not_included" },
					"docs": { "status": "not_included" }
				},
				"notes": {
					"features": [],
					"improvements": [
						{ "component": "运行时依赖", "text": "beta 通道现已指向新的 Runtime Release。" }
					],
					"fixes": [],
					"compatibility": []
				}
			}
		]
	}`)

	feed, err := ParseChangelogFeed(payload)
	if err != nil {
		t.Fatalf("ParseChangelogFeed err = %v", err)
	}

	filtered := feed.FilterDesktopEntries()
	if len(filtered) != 2 {
		t.Fatalf("FilterDesktopEntries len = %d, want 2", len(filtered))
	}
	if filtered[0].ID != "beta-2026.06.11.2" {
		t.Fatalf("first filtered entry = %s, want beta-2026.06.11.2", filtered[0].ID)
	}
	if filtered[0].Summary() != "修复 macOS Desktop bundle id 仍使用默认值的问题。" {
		t.Fatalf("unexpected desktop summary: %q", filtered[0].Summary())
	}
	if filtered[1].ID != "beta-2026.06.11.1" {
		t.Fatalf("second filtered entry = %s, want beta-2026.06.11.1", filtered[1].ID)
	}
	if filtered[1].Summary() != "beta 通道现已指向新的 Runtime Release。" {
		t.Fatalf("unexpected runtime summary: %q", filtered[1].Summary())
	}
}

func TestParseChangelogFeedRejectsInvalidSchema(t *testing.T) {
	_, err := ParseChangelogFeed([]byte(`{"schemaVersion":2,"channel":"beta","updatedAt":"2026-06-11T14:00:00Z","entries":[]}`))
	if err == nil {
		t.Fatal("ParseChangelogFeed err = nil, want invalid schema")
	}
	if got := err.Error(); got != "不支持的 changelog schemaVersion：2" {
		t.Fatalf("ParseChangelogFeed err = %q", got)
	}
}
