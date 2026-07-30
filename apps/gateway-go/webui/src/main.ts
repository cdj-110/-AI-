import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './styles.css'
import './tree.css'
import './modules.css'
import './legacy-interactions.css'
import './protocol-tools.css'
import './button-system.css'
import './ai-assistant.css'
import './confirm.css'

createApp(App).use(createPinia()).mount('#app')
