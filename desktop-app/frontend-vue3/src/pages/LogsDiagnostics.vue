<script setup>
import { logTabs } from "../state/defaults.js"
import { useAppStore } from "../state/useAppStore.js"
import AppButton from "../components/ui/AppButton.vue"
import AppSelect from "../components/ui/AppSelect.vue"
import PageTabs from "../components/ui/PageTabs.vue"
import StatusCard from "../components/ui/StatusCard.vue"
import ToneChip from "../components/ui/ToneChip.vue"

const store = useAppStore()

const sources = [
  ["全部", "all"],
  ["Desktop", "Desktop"],
  ["本机服务", "本机服务"],
  ["开发环境", "开发环境"],
  ["ADB 安装", "ADB 安装"],
  ["手表 App", "手表 App"],
  ["网络访问", "网络访问"],
  ["托管隧道", "托管隧道"],
  ["更新", "更新"],
  ["安全", "安全"]
]
</script>

<template>
  <section class="page-stack">
    <div class="status-strip dense-five">
      <StatusCard v-for="card in store.logsSummaryCards.value" :key="card.title" :card="card" />
    </div>

    <div class="tab-toolbar">
      <PageTabs v-model="store.state.logsTab" :tabs="logTabs" />
      <div v-if="store.state.logsTab === 'events' || store.state.logsTab === 'raw'" class="source-filter">
        <AppSelect v-model="store.state.selectedLogSource" :options="sources" aria-label="日志来源筛选" />
      </div>
    </div>

    <section class="tab-panel">
      <section v-if="store.state.logsTab === 'events'" class="panel compact-panel">
        <header class="panel-head">
          <div>
            <h2>事件时间线</h2>
            <p>按当前日志来源过滤后的最近事件。</p>
          </div>
          <ToneChip>{{ store.eventTimeline.value.length }} 条</ToneChip>
        </header>
        <div class="timeline-table">
          <div v-for="item in store.eventTimeline.value" :key="item.time + item.event" class="timeline-row">
            <span class="timeline-time">{{ item.time }}</span>
            <span class="timeline-event">{{ item.event }}</span>
            <ToneChip :tone="item.level === 'ERROR' ? 'warn' : (item.level === 'SUCCESS' ? 'ok' : 'blue')">{{ item.level }}</ToneChip>
            <span class="timeline-source">{{ item.source }}</span>
          </div>
        </div>
      </section>

      <section v-else-if="store.state.logsTab === 'raw'" class="panel compact-panel">
        <header class="panel-head">
          <div>
            <h2>原始日志查看器</h2>
            <p>显示 Desktop、本机服务、开发环境与安装流程的合并日志。</p>
          </div>
          <ToneChip>{{ store.rawLogLines.value.length }} 行</ToneChip>
        </header>
        <div class="console-window">
          <div v-for="line in store.rawLogLines.value" :key="line">{{ line }}</div>
        </div>
      </section>

      <section v-else-if="store.state.logsTab === 'diagnosis'" class="split-layout">
        <article class="panel compact-panel">
          <header class="panel-head">
            <div>
              <h2>自动诊断建议</h2>
              <p>根据当前状态生成优先检查项。</p>
            </div>
          </header>
          <div class="warning-box">
            <strong>{{ store.live.value.backendHealthy ? "当前没有严重问题" : "发现 1 个可能问题" }}</strong>
            <span>{{ store.live.value.backendHealthy ? "本机服务健康检查通过。" : "手表可能无法访问本机服务。" }}</span>
            <AppButton v-if="!store.live.value.backendHealthy" small tone="primary" @click="store.state.currentPage = 'install'">查看安装向导</AppButton>
          </div>
        </article>
        <article class="panel compact-panel">
          <header class="panel-head">
            <div>
              <h2>推荐操作</h2>
              <p>保留高频排查入口，不把说明摊满整页。</p>
            </div>
          </header>
          <div class="action-list">
            <AppButton block icon="Globe2" @click="store.state.currentPage = 'install'">返回安装向导调整地址</AppButton>
            <AppButton block icon="ShieldCheck" @click="store.state.currentPage = 'install'">检查写入配置</AppButton>
            <AppButton block icon="Copy" :copied="store.selectors.copyFeedbackActive('logs-copy-diagnostics')" @click="store.actions.copyDiagnosticsAction('logs-copy-diagnostics')">复制诊断信息</AppButton>
          </div>
        </article>
      </section>

      <section v-else class="panel compact-panel narrow-content">
        <header class="panel-head">
          <div>
            <h2>导出脱敏诊断包</h2>
            <p>收集必要的日志与环境信息，并排除敏感内容。</p>
          </div>
        </header>
        <div class="export-grid">
          <div>
            <strong>包含</strong>
            <ul>
              <li>Desktop 日志</li>
              <li>本机服务日志</li>
              <li>开发环境日志</li>
              <li>ADB 操作日志</li>
              <li>设备信息</li>
            </ul>
          </div>
          <div>
            <strong>不包含</strong>
            <ul>
              <li>Codex access token</li>
              <li>完整 device token</li>
              <li>tunnel token</li>
            </ul>
          </div>
        </div>
        <div class="actions">
          <AppButton tone="primary" icon="FileText" @click="store.actions.exportDiagnosticsAction">导出脱敏诊断包</AppButton>
          <AppButton icon="Copy" :copied="store.selectors.copyFeedbackActive('logs-copy-raw')" @click="store.actions.copyText(store.allRawLogLines.value.join('\n'), 'logs-copy-raw')">复制当前日志</AppButton>
        </div>
      </section>
    </section>
  </section>
</template>
