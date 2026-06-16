<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue"
import AppIcon from "./Icon.vue"

const props = defineProps({
  modelValue: {
    type: [String, Number, Boolean],
    default: ""
  },
  options: {
    type: Array,
    required: true
  },
  disabled: {
    type: Boolean,
    default: false
  },
  placeholder: {
    type: String,
    default: ""
  },
  ariaLabel: {
    type: String,
    default: "选择"
  }
})

const emit = defineEmits(["update:modelValue"])

const rootRef = ref(null)
const isOpen = ref(false)

const normalizedOptions = computed(() =>
  props.options.map((option) => {
    if (Array.isArray(option)) {
      const [label, value, disabled = false] = option
      return { label, value, disabled }
    }
    return option
  })
)

const selectedOption = computed(() => normalizedOptions.value.find((option) => option.value === props.modelValue) || null)

function closeMenu() {
  isOpen.value = false
}

function toggleMenu() {
  if (props.disabled) {
    return
  }
  isOpen.value = !isOpen.value
}

function selectOption(option) {
  if (option.disabled) {
    return
  }
  emit("update:modelValue", option.value)
  closeMenu()
}

function handlePointerDown(event) {
  if (rootRef.value && !rootRef.value.contains(event.target)) {
    closeMenu()
  }
}

function handleWindowKeydown(event) {
  if (event.key === "Escape") {
    closeMenu()
  }
}

watch(() => props.disabled, (disabled) => {
  if (disabled) {
    closeMenu()
  }
})

onMounted(() => {
  document.addEventListener("pointerdown", handlePointerDown)
  window.addEventListener("keydown", handleWindowKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", handlePointerDown)
  window.removeEventListener("keydown", handleWindowKeydown)
})
</script>

<template>
  <div ref="rootRef" class="app-select" :class="{ 'is-open': isOpen, 'is-disabled': disabled }">
    <button
      class="app-select-trigger"
      type="button"
      :aria-expanded="isOpen ? 'true' : 'false'"
      aria-haspopup="listbox"
      :aria-label="ariaLabel"
      :disabled="disabled"
      @click="toggleMenu"
    >
      <span class="app-select-label" :class="{ 'is-placeholder': !selectedOption }">
        {{ selectedOption?.label || placeholder }}
      </span>
      <AppIcon name="ChevronDown" :size="18" />
    </button>

    <div v-if="isOpen" class="app-select-menu" role="listbox" :aria-label="ariaLabel">
      <button
        v-for="option in normalizedOptions"
        :key="String(option.value)"
        class="app-select-option"
        :class="{ 'is-selected': option.value === modelValue, 'is-disabled': option.disabled }"
        type="button"
        role="option"
        :aria-selected="option.value === modelValue ? 'true' : 'false'"
        :disabled="option.disabled"
        @click="selectOption(option)"
      >
        <span>{{ option.label }}</span>
        <AppIcon v-if="option.value === modelValue" name="Check" :size="15" />
      </button>
    </div>
  </div>
</template>
