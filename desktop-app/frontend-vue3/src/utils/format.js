export function shortTimeLabel(date = new Date()) {
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit"
  }).format(date)
}

export function formatTimeAgo(isoText) {
  const target = new Date(isoText)
  if (Number.isNaN(target.getTime())) {
    return "刚刚"
  }
  const diffMs = Date.now() - target.getTime()
  if (diffMs < 60 * 1000) {
    return "刚刚"
  }
  const diffMinutes = Math.round(diffMs / (60 * 1000))
  if (diffMinutes < 60) {
    return `${diffMinutes} 分钟前`
  }
  const diffHours = Math.round(diffMinutes / 60)
  if (diffHours < 24) {
    return `${diffHours} 小时前`
  }
  return `${Math.round(diffHours / 24)} 天前`
}

export function formatDeveloperLogTime(raw) {
  const text = String(raw || "").trim()
  if (!text) {
    return "--:--:--"
  }
  const matched = text.match(/^(\d{4})-(\d{2})-(\d{2})T(\d{2}:\d{2}:\d{2})/)
  return matched ? `${matched[2]}-${matched[3]} ${matched[4]}` : text
}

export function trimTrailingSlash(value) {
  return String(value || "").trim().replace(/\/+$/, "")
}

export function filenameFromPath(path) {
  return String(path || "").split(/[\\/]/).filter(Boolean).pop() || ""
}

export function formatByteSize(bytes) {
  const value = Number(bytes || 0)
  if (!Number.isFinite(value) || value <= 0) {
    return ""
  }
  const units = ["B", "KB", "MB", "GB"]
  let size = value
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  const digits = index === 0 || size >= 10 ? 0 : 1
  return `${size.toFixed(digits)} ${units[index]}`
}
