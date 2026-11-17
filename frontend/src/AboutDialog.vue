<script setup lang="ts">
import {onMounted,nextTick} from 'vue'
import {version} from '../package.json'
import {APP_ICON} from './api'
import {vDraggableDialog} from './draggableDialog'
const emit=defineEmits<{close:[]}>()
onMounted(()=>nextTick(()=>document.querySelector<HTMLButtonElement>('[data-about-close]')?.focus()))
</script>
<template><div class="confirmation-overlay" @keydown.esc.stop.prevent="emit('close')"><section v-draggable-dialog class="about-dialog" role="dialog" aria-modal="true" aria-labelledby="about-title"><header><img :src="APP_ICON" alt="" width="52" height="52"/><h2 id="about-title">Bajiu DB · 捌玖</h2></header><p>版本 {{version}}</p><p>MySQL 连接与数据库对象浏览工具</p><button class="native-button default-button" data-about-close @click="emit('close')">确定</button></section></div></template>
