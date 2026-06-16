<script setup>
import { useAppStore } from "../../state/useAppStore.js"
import AppIcon from "../ui/Icon.vue"

const store = useAppStore()
</script>

<template>
  <div v-if="store.state.notificationPanelOpen" class="notification-panel">
    <div class="notification-panel-head">
      <div>
        <strong>通知中心</strong>
        <p>健康检查、自动修复和操作结果会显示在这里。</p>
      </div>
      <button class="icon-chip" type="button" title="关闭通知" @click="store.actions.closeNotificationPanel">
        <AppIcon name="X" :size="16" />
      </button>
    </div>
    <div class="notification-panel-body">
      <div v-if="store.state.notifications.length === 0" class="empty-state">当前还没有通知事件。</div>
      <template v-else>
        <article
          v-for="item in store.state.notifications"
          :key="item.id"
          class="notification-item"
          :class="[`tone-${store.selectors.notificationLevelTone(item.level)}`, { 'is-unread': !item.read }]"
        >
          <div class="notification-item-head">
            <strong>{{ item.title }}</strong>
            <span>{{ item.timeLabel }}</span>
          </div>
          <p>{{ item.detail || item.source }}</p>
          <small>{{ item.source }}</small>
        </article>
      </template>
    </div>
  </div>
</template>
