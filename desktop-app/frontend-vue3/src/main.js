import { createApp } from "vue"
import App from "./App.vue"
import { appStoreKey, createAppStore } from "./state/useAppStore.js"
import "./styles/main.css"

const store = createAppStore()

createApp(App)
  .provide(appStoreKey, store)
  .mount("#app")
