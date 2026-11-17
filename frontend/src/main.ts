import { createApp } from 'vue'
import App from './App.vue'
import NativeDialogHost from './NativeDialogHost.vue'
import {installNativeBridge} from './api'
import './style.css'
async function start(){if(new URLSearchParams(location.search).get('desktop')==='1'){const {Call}=await import('@wailsio/runtime');installNativeBridge((name,...args)=>Call.ByName(name,...args))};createApp(new URLSearchParams(location.search).has('dialog')?NativeDialogHost:App).mount('#app')}
void start()
