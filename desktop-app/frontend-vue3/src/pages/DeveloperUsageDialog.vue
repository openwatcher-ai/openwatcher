<script setup>
import { useAppStore } from "../state/useAppStore.js"
import AppButton from "../components/ui/AppButton.vue"
import AppIcon from "../components/ui/Icon.vue"

const store = useAppStore()
</script>

<template>
  <div v-if="store.state.developerConfirmModalOpen" class="modal-backdrop" @click="store.state.developerConfirmModalOpen = false">
    <div class="dialog developer-dialog" @click.stop>
      <header class="dialog-head">
        <div>
          <h2>开发环境使用说明</h2>
          <p>这个页面只管理当前仓库的本机开发服务、开发隧道和发送到手表的调试入口。</p>
        </div>
        <button class="icon-chip" type="button" @click="store.state.developerConfirmModalOpen = false">
          <AppIcon name="X" :size="16" />
        </button>
      </header>
      <div class="dialog-grid">
        <article class="dialog-note">
          <strong>这个页面能做什么</strong>
          <ul>
            <li>选择当前仓库目录，查看服务地址、Healthz 地址和启动脚本。</li>
            <li>启动、停止或重新启动当前仓库的本机开发环境。</li>
            <li>激活开发隧道，并把当前开发环境发送到手表确认。</li>
          </ul>
        </article>
        <article class="dialog-note">
          <strong>状态是怎么判断的</strong>
          <ul>
            <li>Desktop 自己启动服务时，会记录启动时间并持续刷新日志。</li>
            <li>手动启动的本机开发服务也会识别为运行中。</li>
            <li>服务异常时，可直接查看最近日志并重新启动。</li>
          </ul>
        </article>
        <article class="dialog-note warn">
          <strong>需要注意</strong>
          <ul>
            <li>不要把未准备好的地址发送到手表。</li>
            <li>停止环境会结束当前端口对应的进程。</li>
            <li>激活开发隧道后，选择“开发隧道”访问方式才会作为当前 Base URL。</li>
          </ul>
        </article>
      </div>
      <footer class="dialog-foot">
        <AppButton tone="primary" @click="store.state.developerConfirmModalOpen = false">我知道了</AppButton>
      </footer>
    </div>
  </div>
</template>
