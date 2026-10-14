import { createApp } from 'vue';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import './assets/main.css';
import App from './App.vue';
import router from './router';
import { pinia } from './stores';
import { portalLogo } from './portal';

if (portalLogo) {
  const favicon = document.createElement('link');
  favicon.rel = 'icon';
  favicon.type = 'image/png';
  favicon.href = portalLogo;
  document.head.appendChild(favicon);

  const touchIcon = document.createElement('link');
  touchIcon.rel = 'apple-touch-icon';
  touchIcon.href = portalLogo;
  document.head.appendChild(touchIcon);
}

createApp(App).use(ElementPlus).use(pinia).use(router).mount('#app');
