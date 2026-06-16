import test from "node:test"
import assert from "node:assert/strict"

import {
  backendStatusWithHealthResult,
  backendTargetFromHealthResult,
  clearDraftField,
  isDraftField,
  markDraftField,
  setValueAtPath
} from "./interaction_state.mjs"

test("草稿字段标记会按 TTL 过期", () => {
  const registry = {}

  markDraftField(registry, "developerForm.devBaseUrl", 1000)

  assert.equal(isDraftField(registry, "developerForm.devBaseUrl", 1500, 1000), true)
  assert.equal(isDraftField(registry, "developerForm.devBaseUrl", 2501, 1000), false)
  assert.deepEqual(registry, {})
})

test("草稿字段标记可以显式清理", () => {
  const registry = {}

  markDraftField(registry, "remoteBootstrapForm.bootstrapCode", 1000)
  clearDraftField(registry, "remoteBootstrapForm.bootstrapCode")

  assert.equal(isDraftField(registry, "remoteBootstrapForm.bootstrapCode", 1000), false)
})

test("setValueAtPath 只更新已存在的嵌套状态路径", () => {
  const state = {
    installForm: {
      pairIp: ""
    }
  }

  assert.equal(setValueAtPath(state, "installForm.pairIp", "192.168.31.88"), true)
  assert.equal(state.installForm.pairIp, "192.168.31.88")
  assert.equal(setValueAtPath(state, "missing.value", "ignored"), false)
  assert.deepEqual(Object.keys(state), ["installForm"])
})

test("健康检查失败会覆盖 backend 状态和顶部目标状态", () => {
  const backend = {
    running: true,
    state: "running",
    message: "OpenWatcher 本机服务已启动。",
    lastHealth: {
      ok: true,
      message: "HTTP 200"
    }
  }
  const health = {
    ok: false,
    message: "尚未连接到 sidecar /healthz。"
  }

  const nextBackend = backendStatusWithHealthResult(backend, health)
  const target = backendTargetFromHealthResult(health)

  assert.equal(nextBackend.state, "error")
  assert.equal(nextBackend.lastHealth, health)
  assert.equal(nextBackend.friendlyError, "尚未连接到 sidecar /healthz。")
  assert.deepEqual(target, {
    ok: false,
    detail: "尚未连接到 sidecar /healthz。",
    source: "本机服务"
  })
})

test("健康检查恢复会清理旧错误状态", () => {
  const backend = {
    running: true,
    state: "error",
    friendlyError: "尚未连接到 sidecar /healthz。"
  }
  const health = {
    ok: true,
    message: "HTTP 200"
  }

  const nextBackend = backendStatusWithHealthResult(backend, health)
  const target = backendTargetFromHealthResult(health)

  assert.equal(nextBackend.state, "running")
  assert.equal(nextBackend.message, "OpenWatcher 本机服务已启动。")
  assert.equal(nextBackend.friendlyError, "")
  assert.equal(target.ok, true)
  assert.equal(target.detail, "HTTP 200")
})
