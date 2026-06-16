export const DRAFT_FIELD_TTL_MS = 5 * 60 * 1000

export function markDraftField(registry, path, now = Date.now()) {
  if (!registry || !path) {
    return registry
  }
  registry[path] = now
  return registry
}

export function clearDraftField(registry, path) {
  if (!registry || !path) {
    return registry
  }
  delete registry[path]
  return registry
}

export function isDraftField(registry, path, now = Date.now(), ttlMs = DRAFT_FIELD_TTL_MS) {
  if (!registry || !path) {
    return false
  }
  const markedAt = Number(registry[path] || 0)
  if (!markedAt) {
    return false
  }
  if (now - markedAt > ttlMs) {
    delete registry[path]
    return false
  }
  return true
}
export function setValueAtPath(root, path, value) {
  const segments = String(path || "").split(".").filter(Boolean)
  if (!root || segments.length < 2) {
    return false
  }
  let cursor = root
  for (let index = 0; index < segments.length - 1; index += 1) {
    cursor = cursor?.[segments[index]]
    if (!cursor || typeof cursor !== "object") {
      return false
    }
  }
  cursor[segments[segments.length - 1]] = value
  return true
}

export function backendStatusWithHealthResult(backend, health) {
  const next = { ...(backend || {}) }
  if (!health) {
    return next
  }
  next.lastHealth = health
  if (!health.ok) {
    const message = health.message || "本机服务未通过 /healthz"
    next.message = message
    next.friendlyError = message
    next.state = "error"
  } else if (next.running && next.state === "error") {
    next.state = "running"
    next.message = "OpenWatcher 本机服务已启动。"
    next.friendlyError = ""
  }
  return next
}

export function backendTargetFromHealthResult(health, fallbackDetail = "本机服务未就绪") {
  return {
    ok: Boolean(health?.ok),
    detail: health?.message || fallbackDetail,
    source: "本机服务"
  }
}
