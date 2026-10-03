import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'

import App from './App.vue'
import { router } from './router'

const app = createApp(App)
app.use(createPinia())
app.use(router)
// 界面全中文：管理端与教师工作台的使用者都是内部人员
app.use(ElementPlus, { locale: zhCn })
app.mount('#app')
