import Vue from 'vue'
import ElementUI from 'element-ui'
import 'element-ui/lib/theme-chalk/index.css'

import App from './App.vue'
import './styles/index.scss'

Vue.config.productionTip = false
Vue.use(ElementUI)

new Vue({
  render: (createElement) => createElement(App),
}).$mount('#app')
