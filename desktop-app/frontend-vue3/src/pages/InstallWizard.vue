<script setup>
import { computed } from "vue"
import { INSTALL_GUIDES } from "../data/installGuides.js"
import { useAppStore } from "../state/useAppStore.js"
import AppButton from "../components/ui/AppButton.vue"
import AppIcon from "../components/ui/Icon.vue"
import FieldRow from "../components/ui/FieldRow.vue"
import ToneChip from "../components/ui/ToneChip.vue"

const store = useAppStore()

const currentStage = computed(() => store.state.wizard.currentStage)
const selectedDevice = computed(() => store.selectedDevice.value)
const installerState = computed(() => store.state.installerState)
const installGuide = computed(() => INSTALL_GUIDES[0])
const enabledEntries = computed(() => store.selectors.wizardEnabledConfigEntries())

function stageButtonClass(stage) {
  return {
    "is-current": stage.id === store.state.wizard.currentStage,
    "is-done": store.selectors.wizardStageCompleted(stage.id)
  }
}

function setEntryField(entryId, field, value) {
  store.state.wizard.configEntries[entryId][field] = value
  store.actions.touchWizardBoundField(`wizard.configEntries.${entryId}.${field}`)
  store.actions.persistInstallNetworkState()
}
</script>

<template>
  <section class="page-stack install-page">
    <section class="install-shell">
      <nav class="stage-tabs">
        <button
          v-for="(stage, index) in store.constants.installWizardStages"
          :key="stage.id"
          class="stage-tab"
          :class="stageButtonClass(stage)"
          :disabled="!store.selectors.wizardStageUnlocked(stage.id)"
          type="button"
          @click="store.actions.setWizardStage(stage.id)"
        >
          <span class="stage-index">{{ index + 1 }}</span>
          <span class="stage-copy">
            <strong>{{ stage.title }}</strong>
            <em>{{ stage.navSummary }}</em>
          </span>
        </button>
      </nav>

      <section class="stage-panel">
        <div class="stage-body">
          <div v-if="store.wizardFailure.value" class="inline-alert error">
            <strong>{{ store.wizardFailure.value.message }}</strong>
            <span>{{ store.selectors.wizardFailureTips()[0] }}</span>
            <AppButton
              v-if="String(store.wizardFailure.value.message || '').includes('已存在配对信息')"
              small
              icon="Trash2"
              @click="store.state.currentPage = 'settings'; store.state.settingsTab = 'danger'"
            >
              去清空
            </AppButton>
          </div>

          <div v-if="currentStage === 'prepare'" class="stage-grid prepare">
            <section class="panel compact-panel">
              <header class="panel-head">
                <div>
                  <h2>准备清单</h2>
                  <p>自动项只刷新检测结果，手动确认会保留。</p>
                </div>
                <ToneChip>{{ store.state.wizard.lastRefreshLabel }}</ToneChip>
              </header>

              <div class="section-block">
                <div class="section-caption">自动检查</div>
                <div class="check-grid">
                  <div v-for="item in store.selectors.wizardAutoChecks(store.live.value)" :key="item.id" class="check-item">
                    <div class="check-top">
                      <span class="check-icon" :class="item.ok ? 'ok' : 'warn'">{{ item.ok ? "✓" : "!" }}</span>
                      <strong>{{ item.label }}</strong>
                      <ToneChip :tone="item.ok ? 'ok' : 'warn'">{{ item.tag }}</ToneChip>
                    </div>
                    <span v-for="detail in item.detail" :key="detail" class="muted-line">{{ detail }}</span>
                  </div>
                </div>
              </div>

              <div class="section-block">
                <div class="section-head">
                  <div class="section-caption">手动确认</div>
                  <button
                    class="check-all-action"
                    type="button"
                    :aria-pressed="store.selectors.wizardManualChecksDone()"
                    @click="store.selectors.toggleAllWizardManualChecks"
                  >
                    <span class="check-all-box" :class="{ 'is-checked': store.selectors.wizardManualChecksDone() }">
                      {{ store.selectors.wizardManualChecksDone() ? "✓" : "" }}
                    </span>
                    <span>{{ store.selectors.wizardManualChecksDone() ? "取消全选" : "全选" }}</span>
                  </button>
                </div>
                <div class="check-grid manual-check-grid">
                  <button
                    v-for="item in store.constants.prepareManualChecks"
                    :key="item.id"
                    class="check-item action"
                    :class="{ 'is-checked': store.state.wizard.manualChecks[item.id] }"
                    type="button"
                    @click="store.state.wizard.manualChecks[item.id] = !store.state.wizard.manualChecks[item.id]"
                  >
                    <div class="check-top">
                      <span class="check-icon" :class="store.state.wizard.manualChecks[item.id] ? 'ok' : 'todo'">
                        {{ store.state.wizard.manualChecks[item.id] ? "✓" : "□" }}
                      </span>
                      <strong>{{ item.label }}</strong>
                      <ToneChip :tone="store.state.wizard.manualChecks[item.id] ? 'ok' : 'warn'">
                        {{ store.state.wizard.manualChecks[item.id] ? "已确认" : "待确认" }}
                      </ToneChip>
                    </div>
                  </button>
                </div>
              </div>
            </section>

            <section class="panel compact-panel">
              <header class="panel-head">
                <div>
                  <h2>通用文字教程</h2>
                  <p>{{ installGuide.subtitle }}</p>
                </div>
              </header>
              <ol class="guide-step-list">
                <li v-for="(step, index) in installGuide.steps" :key="step.title" class="guide-step-item">
                  <span class="guide-step-index">{{ index + 1 }}</span>
                  <div class="guide-step-copy">
                    <strong>{{ step.title }}</strong>
                    <p>{{ step.body }}</p>
                  </div>
                </li>
              </ol>
            </section>
          </div>

          <div v-else-if="currentStage === 'connect'" class="stage-grid two">
            <section class="panel compact-panel">
              <header class="panel-head">
                <div>
                  <h2>填写手表信息</h2>
                  <p>把手表页面上的地址、端口和配对码填进来。</p>
                </div>
                <ToneChip :tone="store.selectors.wizardConnectReady() ? 'ok' : 'soft'">
                  {{ store.selectors.wizardConnectReady() ? "已就绪" : "等待连接" }}
                </ToneChip>
              </header>
              <div v-if="store.selectors.pairingSkipReason()" class="inline-alert">
                <strong>已检测到可用设备</strong>
                <span>{{ store.selectors.pairingSkipReason() }}</span>
              </div>
              <div class="form-grid two">
                <FieldRow label="手表 IP">
                  <input v-model="store.state.installForm.pairIp" placeholder="例如 192.168.31.88" @input="store.actions.touchWizardBoundField('installForm.pairIp')" />
                </FieldRow>
                <FieldRow label="配对端口">
                  <input v-model="store.state.installForm.pairPort" placeholder="例如 37153" />
                </FieldRow>
                <FieldRow label="连接端口">
                  <input v-model="store.state.installForm.connectPort" placeholder="例如 40221" />
                </FieldRow>
                <FieldRow label="配对码">
                  <input v-model="store.state.installForm.pairingCode" placeholder="六位配对码" />
                </FieldRow>
              </div>
              <button class="text-action" type="button" @click="store.state.installForm.useSeparateConnectIP = !store.state.installForm.useSeparateConnectIP">
                {{ store.state.installForm.useSeparateConnectIP ? "收起高级选项" : "高级选项" }}
              </button>
              <FieldRow v-if="store.state.installForm.useSeparateConnectIP" label="单独的连接地址">
                <input v-model="store.state.installForm.connectIp" placeholder="只有少数设备需要填写" />
              </FieldRow>
              <div class="inline-alert">
                <strong>默认只用一个地址</strong>
                <span>大多数设备只需要填写一个手表 IP。只有少数设备需要单独填写连接地址。</span>
              </div>
            </section>

            <section class="panel compact-panel">
              <header class="panel-head">
                <div>
                  <h2>连接进展</h2>
                  <p>只显示这一步需要确认的内容。</p>
                </div>
              </header>
              <div class="result-list">
                <div class="result-item">
                  <strong>当前状态 <ToneChip :tone="store.selectors.wizardConnectReady() ? 'ok' : 'soft'">{{ store.selectors.wizardConnectReady() ? "可继续" : "进行中" }}</ToneChip></strong>
                  <span>{{ selectedDevice ? `${selectedDevice.displayName || selectedDevice.serial} 已连接` : (installerState.message || "填写手表上的信息后继续") }}</span>
                </div>
                <div class="result-item">
                  <strong>目标设备 <ToneChip :tone="selectedDevice ? 'ok' : 'warn'">{{ selectedDevice ? "已确认" : "待确认" }}</ToneChip></strong>
                  <span>{{ selectedDevice ? `${selectedDevice.displayName || selectedDevice.serial} · ${selectedDevice.serial}` : (store.selectors.installerSelectionRequired() ? "检测到多个设备，请在下面点选。" : "连接成功后会自动确认。") }}</span>
                </div>
              </div>
              <div v-if="store.installerDevicesList.value.length > 0" class="device-choice-grid">
                <button
                  v-for="device in store.installerDevicesList.value"
                  :key="device.serial"
                  class="device-choice"
                  :class="{ 'is-active': device.serial === store.state.installerState?.selectedSerial || (!store.state.installerState?.selectedSerial && selectedDevice?.serial === device.serial) }"
                  type="button"
                  @click="store.actions.selectInstallerDevice(device.serial)"
                >
                  <strong>{{ device.displayName || device.serial }}</strong>
                  <span>{{ device.serial }}</span>
                  <em>{{ device.isEmulator ? "模拟器" : (device.isWatch ? "手表" : "其他设备") }} · {{ device.state }}</em>
                </button>
              </div>
              <div v-else class="inline-alert">
                <strong>等待设备出现</strong>
                <span>完成连接后，这里会显示这次可安装的设备。</span>
              </div>
              <div class="actions right">
                <AppButton tone="primary" icon="Watch" @click="store.actions.runConnectStageAction">开始连接</AppButton>
              </div>
            </section>
          </div>

          <div v-else-if="currentStage === 'install'" class="stage-grid two">
            <section class="panel compact-panel">
              <header class="panel-head">
                <div>
                  <h2>安装目标</h2>
                  <p>确认目标设备和安装包后执行安装或打开应用。</p>
                </div>
              </header>
              <div class="summary-grid">
                <div class="summary-box">
                  <h3>目标设备</h3>
                  <div class="mini-table">
                    <span>设备名称</span><strong>{{ selectedDevice?.displayName || "未选择" }}</strong>
                    <span>设备标识</span><strong>{{ selectedDevice?.serial || "未连接" }}</strong>
                    <span>设备类型</span><strong>{{ selectedDevice ? (selectedDevice.isEmulator ? "模拟器" : (selectedDevice.isWatch ? "真实手表" : "其他设备")) : "等待连接" }}</strong>
                  </div>
                </div>
                <div class="summary-box">
                  <h3>安装包</h3>
                  <div class="mini-table">
                    <span>版本</span><strong>{{ installerState.apk?.versionName || "未检测" }}</strong>
                    <span>签名</span><strong>{{ installerState.apk?.debug ? "debug" : "release" }}</strong>
                    <span>来源</span><strong>{{ installerState.apk?.path ? installerState.apk.path.split('/').slice(-1)[0] : "未检测到安装包" }}</strong>
                  </div>
                </div>
              </div>
              <div class="inline-alert">
                <strong>安装后会自动启动</strong>
                <span>应用安装完成后会自动在手表上打开，方便继续下一步。</span>
              </div>
            </section>

            <section class="panel compact-panel">
              <header class="panel-head">
                <div>
                  <h2>安装结果</h2>
                  <p>这里只保留目标设备、安装包和结果摘要。</p>
                </div>
              </header>
              <div class="result-list">
                <div class="result-item">
                  <strong>安装包检查 <ToneChip :tone="installerState.apk?.available ? 'ok' : 'warn'">{{ installerState.apk?.available ? "可安装" : "未就绪" }}</ToneChip></strong>
                  <span>{{ installerState.apk?.message || store.selectors.installerSummaryNote() }}</span>
                </div>
                <div class="result-item">
                  <strong>安装状态 <ToneChip :tone="installerState.apk?.installed ? 'ok' : 'soft'">{{ installerState.apk?.installed ? "已安装" : "等待执行" }}</ToneChip></strong>
                  <span>{{ store.state.wizard.stageNotes.install || "点击开始安装后执行安装流程。" }}</span>
                </div>
              </div>
              <div class="actions right">
                <AppButton tone="primary" icon="Watch" @click="store.actions.runInstallStageAction">
                  {{ installerState.apk?.installed ? "打开应用" : "开始安装" }}
                </AppButton>
              </div>
            </section>
          </div>

          <div v-else class="stage-grid two">
            <section class="panel compact-panel">
              <header class="panel-head">
                <div>
                  <h2>启用要发送到手表的地址</h2>
                  <p>只有显式启用的地址，才会一起发送到手表确认。</p>
                </div>
              </header>
              <div class="config-entry-grid">
                <article
                  v-for="entryId in ['lan', 'public', 'tunnel']"
                  :key="entryId"
                  class="config-entry"
                  :class="{ 'is-enabled': store.state.wizard.configEntries[entryId].enabled }"
                >
                  <div class="config-entry-head">
                    <button type="button" class="switch-row" @click="store.state.wizard.configEntries[entryId].enabled = !store.state.wizard.configEntries[entryId].enabled; store.actions.persistInstallNetworkState()">
                      <span class="toggle" :class="{ 'is-on': store.state.wizard.configEntries[entryId].enabled }"></span>
                      <AppIcon :name="store.constants.installConfigMeta[entryId].icon" :size="15" />
                      <strong>{{ store.constants.installConfigMeta[entryId].label }}</strong>
                    </button>
                    <ToneChip :tone="store.state.wizard.configEntries[entryId].validation === 'valid' ? 'ok' : (store.state.wizard.configEntries[entryId].validation === 'error' ? 'warn' : 'soft')">
                      {{ store.state.wizard.configEntries[entryId].validation === "valid" ? "已通过" : (store.state.wizard.configEntries[entryId].validation === "error" ? "未通过" : (store.state.wizard.configEntries[entryId].enabled ? "待检查" : "未启用")) }}
                    </ToneChip>
                  </div>
                  <p>{{ store.constants.installConfigMeta[entryId].inactiveText }}</p>
                  <FieldRow v-if="entryId === 'tunnel'" :label="store.constants.installConfigMeta[entryId].inputLabel">
                    <input :value="store.state.wizard.configEntries.tunnel.code" placeholder="例如 OW-ABCD-1234" @input="setEntryField('tunnel', 'code', $event.target.value)" />
                  </FieldRow>
                  <FieldRow v-else :label="store.constants.installConfigMeta[entryId].inputLabel">
                    <input :value="store.state.wizard.configEntries[entryId].url" @input="setEntryField(entryId, 'url', $event.target.value)" />
                  </FieldRow>
                  <div v-if="entryId === 'tunnel' && store.state.wizard.configEntries.tunnel.redeemedDomain" class="mini-table single">
                    <span>域名</span><strong>{{ store.state.wizard.configEntries.tunnel.redeemedDomain }}</strong>
                  </div>
                  <div class="actions">
                    <AppButton small icon="RefreshCw" @click="store.actions.validateConfigEntryAction(entryId)">
                      {{ store.constants.installConfigMeta[entryId].validateLabel }}
                    </AppButton>
                    <span class="muted-line">{{ store.state.wizard.configEntries[entryId].message }}</span>
                  </div>
                </article>
              </div>
            </section>

            <section class="panel compact-panel">
              <header class="panel-head">
                <div>
                  <h2>写入前检查</h2>
                  <p>整理已启用的地址，再发送到当前手表等待确认。</p>
                </div>
              </header>
              <div class="result-list">
                <div class="result-item">
                  <strong>将一起发送的地址 <ToneChip>{{ enabledEntries.length }} 项</ToneChip></strong>
                  <span>{{ enabledEntries.length > 0 ? enabledEntries.map(([entryId]) => store.constants.installConfigMeta[entryId].label).join("、") : "还没有启用任何地址。" }}</span>
                </div>
                <div class="result-item">
                  <strong>目标手表 <ToneChip :tone="selectedDevice ? 'ok' : 'warn'">{{ selectedDevice ? "已确认" : "未确认" }}</ToneChip></strong>
                  <span>{{ selectedDevice ? `${selectedDevice.displayName || selectedDevice.serial} 将接收配置链接，并需要在手表上确认。` : "请先回到上一页确认目标设备。" }}</span>
                </div>
                <div class="result-item">
                  <strong>最近配置链接 <ToneChip :tone="store.state.generatedBootstrap?.apiBase ? 'ok' : 'soft'">{{ store.state.generatedBootstrap?.apiBase ? "已生成" : "未生成" }}</ToneChip></strong>
                  <span>{{ store.state.generatedBootstrap?.apiBase || "生成后，这里会显示最近一次发送的地址。" }}</span>
                </div>
              </div>
              <div class="inline-alert">
                <strong>会一起发送多个已启用地址</strong>
                <span>手表收到配置链接后，需要先确认保存，再选择当前可用的连接入口。</span>
              </div>
              <div class="actions right">
                <AppButton tone="primary" icon="RefreshCw" @click="store.actions.runConfigStageAction">
                  {{ store.selectors.configWriteActionLabel() }}
                </AppButton>
              </div>
            </section>
          </div>
        </div>

        <footer class="stage-footer">
          <div class="footer-note">{{ store.state.wizard.stageNotes[store.state.wizard.currentStage] || "" }}</div>
          <div class="actions right">
            <AppButton @click="store.actions.handleWizardSecondaryAction">{{ store.currentWizardStage.value.secondaryLabel }}</AppButton>
            <AppButton tone="primary" @click="store.actions.handleWizardPrimaryAction">{{ store.currentWizardStage.value.primaryLabel }}</AppButton>
          </div>
        </footer>
      </section>
    </section>

  </section>
</template>
