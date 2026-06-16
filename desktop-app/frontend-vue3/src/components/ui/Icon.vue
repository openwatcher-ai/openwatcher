<script setup>
import { computed } from "vue"
import {
  Bell,
  Check,
  Circle,
  ChevronDown,
  Cloud,
  Copy,
  Cpu,
  Download,
  FileText,
  FolderOpen,
  Globe2,
  Link2,
  Monitor,
  Moon,
  PackageCheck,
  Play,
  RefreshCw,
  Server,
  Settings,
  ShieldCheck,
  Sun,
  Terminal,
  Trash2,
  Wand2,
  Watch,
  X
} from "@lucide/vue"

const props = defineProps({
  name: {
    type: String,
    default: "Circle"
  },
  size: {
    type: [Number, String],
    default: 17
  }
})

const icons = {
  Bell,
  Check,
  Circle,
  ChevronDown,
  Cloud,
  Copy,
  Cpu,
  Download,
  FileText,
  FolderOpen,
  Globe2,
  Link2,
  Monitor,
  Moon,
  PackageCheck,
  Play,
  RefreshCw,
  Server,
  Settings,
  ShieldCheck,
  Sun,
  Terminal,
  Trash2,
  Wand2,
  Watch,
  X
}

const Component = computed(() => icons[props.name] || Circle)
</script>

<template>
  <component :is="Component" :size="size" :stroke-width="2" aria-hidden="true" />
</template>
