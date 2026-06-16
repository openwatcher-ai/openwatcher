<script setup>
import { watchTabs } from "../state/defaults.js"
import { useAppStore } from "../state/useAppStore.js"
import AppButton from "../components/ui/AppButton.vue"
import AppSelect from "../components/ui/AppSelect.vue"
import InfoTable from "../components/ui/InfoTable.vue"
import PageTabs from "../components/ui/PageTabs.vue"
import StatusCard from "../components/ui/StatusCard.vue"
import FieldRow from "../components/ui/FieldRow.vue"
import ToneChip from "../components/ui/ToneChip.vue"

const store = useAppStore()
</script>

<template>
  <section class="page-stack">
    <div class="status-strip">
      <StatusCard v-for="card in store.watchSummaryCards.value" :key="card.title" :card="card" />
    </div>

    <PageTabs v-model="store.state.watchTab" :tabs="watchTabs" />

    <section class="tab-panel">
      <div v-if="store.state.watchTab === 'overview'" class="split-layout">
        <section class="panel compact-panel">
          <header class="panel-head">
            <div>
              <h2>设备信息</h2>
              <p>来自当前 ADB 连接状态。</p>
            </div>
            <ToneChip :tone="store.selectedDevice.value ? 'ok' : 'warn'">{{ store.selectedDevice.value ? "已连接" : "未连接" }}</ToneChip>
          </header>
          <InfoTable :rows="[
            ['设备名', store.selectedDevice.value?.displayName || '未检测'],
            ['制造商', store.selectedDevice.value?.product || '未检测'],
            ['型号', store.selectedDevice.value?.model || '未检测'],
            ['Android 版本', store.selectedDevice.value?.isEmulator ? '模拟器' : '待补充'],
            ['API level', '待检测'],
            ['ABI', store.selectedDevice.value?.device || '待检测'],
            ['ADB serial', store.selectedDevice.value?.serial || '未检测'],
            ['设备类型', store.selectedDevice.value?.isEmulator ? 'Wear OS 模拟器' : (store.selectedDevice.value?.isWatch ? '手表设备' : '未知设备')]
          ]" />
        </section>

        <section class="panel compact-panel">
          <header class="panel-head">
            <div>
              <h2>连接管理</h2>
              <p>安装和维护时需要 ADB 连接。</p>
            </div>
          </header>
          <InfoTable :rows="[
            ['无线 ADB 状态', store.selectedDevice.value ? '已连接' : '未连接'],
            ['连接 IP', store.selectedDevice.value?.serial?.includes(':') ? store.selectedDevice.value.serial.split(':')[0] : '未检测'],
            ['端口', store.state.installerState?.selectedPort || '未检测']
          ]" />
          <div class="actions">
            <AppButton icon="RefreshCw" :loading="store.state.installerRefreshRunning" :disabled="store.state.installerRefreshRunning" @click="store.actions.refreshInstallerState">刷新设备列表</AppButton>
            <AppButton icon="RefreshCw" @click="store.state.currentPage = 'install'">重新配对</AppButton>
            <AppButton tone="danger" icon="Trash2" @click="store.state.currentPage = 'settings'; store.state.settingsTab = 'danger'">忘记设备</AppButton>
          </div>
        </section>
      </div>

      <section v-else-if="store.state.watchTab === 'app'" class="panel compact-panel narrow-content">
        <header class="panel-head">
          <div>
            <h2>应用状态</h2>
            <p>确认手表端 OpenWatcher App 安装和版本。</p>
          </div>
          <ToneChip :tone="store.state.installerState.apk?.installed ? 'ok' : (store.state.installerState.apk?.available ? 'blue' : 'warn')">
            {{ store.state.installerState.apk?.installed ? "已安装" : (store.state.installerState.apk?.available ? "可安装" : "未就绪") }}
          </ToneChip>
        </header>
        <InfoTable :rows="[
          ['安装状态', store.state.installerState.apk?.installed ? '已安装' : (store.state.installerState.apk?.available ? '可安装' : '未找到安装包')],
          ['版本', store.state.installerState.apk?.installedVersionName || store.state.installerState.apk?.versionName || '未检测'],
          ['versionCode', store.state.installerState.apk?.installedVersionCode || store.state.installerState.apk?.versionCode || '未检测'],
          ['包名', store.state.installerState.apk?.packageName || '未检测'],
          ['签名', store.state.installerState.apk?.debug ? 'debug（仅本地验证）' : 'release'],
          ['APK 来源', store.state.installerState.apk?.label || '未检测'],
          ['SHA-256', store.state.installerState.apk?.sha256 ? store.state.installerState.apk.sha256.slice(0, 12) + '...' : '未检测']
        ]" />
        <div class="actions">
          <AppButton icon="Watch" @click="store.actions.installWatchAppAction">安装或覆盖安装</AppButton>
          <AppButton icon="Play" @click="store.actions.launchWatchAppAction">启动手表 App</AppButton>
        </div>
      </section>

      <section v-else-if="store.state.watchTab === 'config'" class="panel compact-panel narrow-content">
        <header class="panel-head">
          <div>
            <h2>配置状态</h2>
            <p>最近一次写入到手表的 bootstrap 信息。</p>
          </div>
        </header>
        <InfoTable :rows="[
          ['API 基址', store.currentApi.value],
          ['设备名', store.state.watchForm.deviceName || 'watch'],
          ['Token 指纹', store.state.generatedBootstrap?.tokenFingerprint || '尚未生成'],
          ['配置来源', 'Desktop bootstrap'],
          ['配置时间', store.state.generatedBootstrap?.createdAt || '尚未写入']
        ]" />
        <div class="actions">
          <AppButton tone="primary" icon="RefreshCw" @click="store.actions.applyWatchBootstrapAction()">重新写入配置</AppButton>
          <AppButton icon="Link2" @click="store.state.currentPage = 'install'">调整写入地址</AppButton>
          <AppButton icon="Copy" :copied="store.selectors.copyFeedbackActive('watch-bootstrap-uri')" @click="store.actions.prepareWatchBootstrapAction('watch-bootstrap-uri')">仅生成 bootstrap URI</AppButton>
          <AppButton tone="danger" icon="Trash2" @click="store.state.currentPage = 'settings'; store.state.settingsTab = 'danger'">重置配对</AppButton>
        </div>
        <div v-if="store.state.generatedBootstrap" class="inline-alert">
          <strong>最近一次已生成配置链接</strong>
          <span>{{ store.state.generatedBootstrap.deviceName }} · token 指纹 {{ store.state.generatedBootstrap.tokenFingerprint }} · {{ store.state.generatedBootstrap.apiBase }}</span>
        </div>
      </section>

      <section v-else-if="store.state.watchTab === 'connection'" class="panel compact-panel narrow-content">
        <header class="panel-head">
          <div>
            <h2>可用设备</h2>
            <p>多个 ADB 设备同时存在时，先确认目标手表。</p>
          </div>
          <AppButton small icon="RefreshCw" :loading="store.state.installerRefreshRunning" :disabled="store.state.installerRefreshRunning" @click="store.actions.refreshInstallerState">刷新</AppButton>
        </header>
        <div v-if="store.installerDevicesList.value.length > 0" class="device-choice-grid">
          <button
            v-for="device in store.installerDevicesList.value"
            :key="device.serial"
            class="device-choice"
            :class="{ 'is-active': device.serial === store.state.installerState?.selectedSerial }"
            type="button"
            @click="store.actions.selectInstallerDevice(device.serial)"
          >
            <strong>{{ device.displayName || device.serial }}</strong>
            <span>{{ device.serial }}</span>
            <em>{{ device.isEmulator ? "模拟器" : (device.isWatch ? "手表" : "其他设备") }} · {{ device.state }}</em>
          </button>
        </div>
        <div v-else class="empty-state">当前没有检测到 ADB 设备。</div>
      </section>

      <section v-else-if="store.state.watchTab === 'remote'" class="panel compact-panel narrow-content">
        <header class="panel-head">
          <div>
            <h2>远程初始化</h2>
            <p>手表和电脑不在同一网络时，使用手表上的临时配置码发送 API 基址。</p>
          </div>
        </header>
        <div class="form-grid two">
          <FieldRow label="临时配置码">
            <input v-model="store.state.remoteBootstrapForm.bootstrapCode" placeholder="例如 AB12CD34" :disabled="store.state.remoteBootstrapForm.submitting" />
          </FieldRow>
          <FieldRow label="环境类型">
            <AppSelect
              v-model="store.state.remoteBootstrapForm.environment"
              :disabled="store.state.remoteBootstrapForm.submitting"
              :options="[
                { label: 'beta', value: 'beta' },
                { label: 'dev', value: 'dev' }
              ]"
              aria-label="环境类型"
            />
          </FieldRow>
          <FieldRow label="API 基址">
            <input v-model="store.state.remoteBootstrapForm.apiBase" :placeholder="store.selectors.remoteBootstrapDefaultApiBase()" :disabled="store.state.remoteBootstrapForm.submitting" />
          </FieldRow>
          <FieldRow label="隧道配置码（可选）">
            <input v-model="store.state.remoteBootstrapForm.tunnelCode" placeholder="留空则使用上面的 API 基址" :disabled="store.state.remoteBootstrapForm.submitting" />
          </FieldRow>
        </div>
        <div class="actions">
          <AppButton tone="primary" icon="Link2" :disabled="store.state.remoteBootstrapForm.submitting" @click="store.actions.submitRemoteWatchBootstrapAction">
            {{ store.state.remoteBootstrapForm.submitting ? "发送中" : "发送到手表临时配置" }}
          </AppButton>
          <AppButton icon="RefreshCw" :disabled="store.state.remoteBootstrapForm.submitting" @click="store.state.remoteBootstrapForm.apiBase = store.selectors.remoteBootstrapDefaultApiBase()">填入当前建议地址</AppButton>
        </div>
        <InfoTable :rows="[
          ['环境', store.selectors.remoteBootstrapEnvironmentLabel(store.state.remoteBootstrapForm.result?.environment || store.state.remoteBootstrapForm.environment)],
          ['API 基址', store.state.remoteBootstrapForm.result?.apiBase || store.state.remoteBootstrapForm.apiBase || store.selectors.remoteBootstrapDefaultApiBase()],
          ['健康检查', store.selectors.remoteBootstrapHealthLabel()],
          ['提交时间', store.state.remoteBootstrapForm.result?.submittedAt || '尚未提交']
        ]" />
      </section>

      <section v-else class="panel compact-panel">
        <header class="panel-head">
          <div>
            <h2>最近设备 / 兼容性记录</h2>
            <p>用于识别常见手表型号与安装状态。</p>
          </div>
        </header>
        <table class="data-table">
          <thead>
            <tr>
              <th>设备名</th>
              <th>型号</th>
              <th>Android 版本</th>
              <th>最近连接时间</th>
              <th>ADB 状态</th>
              <th>兼容性</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="device in store.selectors.deviceHistoryRows()" :key="device.name + device.lastSeen">
              <td>{{ device.name }} <ToneChip v-if="device.badge" tone="blue">{{ device.badge }}</ToneChip></td>
              <td>{{ device.model }}</td>
              <td>{{ device.android }}</td>
              <td>{{ device.lastSeen }}</td>
              <td>{{ device.adbState }}</td>
              <td>{{ device.compatibility }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </section>
  </section>
</template>
