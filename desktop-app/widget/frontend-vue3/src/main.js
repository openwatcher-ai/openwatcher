import { createApp } from 'vue'
import App from './App.vue'
import './styles/main.css'

if (['http:', 'https:'].includes(window.location.protocol)) {
  document.documentElement.classList.add('browser-preview')
}

createApp(App).mount('#app')
