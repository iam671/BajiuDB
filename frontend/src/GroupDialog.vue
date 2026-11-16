<script setup lang="ts">
import { nextTick, onMounted } from 'vue'
import { Folder, X, LoaderCircle } from '@lucide/vue'
import type { ConnectionGroup } from './api'

defineProps<{
 model:{groupId:string};
 groups:ConnectionGroup[];busy:boolean;error:string;
}>()
const emit=defineEmits<{save:[];cancel:[]}>()
function trap(e:KeyboardEvent){const nodes=Array.from((e.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),select:not(:disabled)'));const first=nodes[0],last=nodes[nodes.length-1];if(e.shiftKey&&document.activeElement===first){e.preventDefault();last?.focus()}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first?.focus()}}
onMounted(()=>nextTick(()=>document.getElementById('group-dialog-value')?.focus()))
</script>

<template>
 <div class="confirmation-overlay">
  <section class="group-dialog" role="dialog" aria-modal="true" aria-labelledby="group-dialog-title" @keydown.tab="trap">
   <header class="dialog-titlebar"><Folder :size="16" class="folder-icon"/><h2 id="group-dialog-title">移至分组</h2><button class="window-close" aria-label="关闭分组窗口" @click="emit('cancel')" :disabled="busy"><X :size="16"/></button></header>
   <form @submit.prevent="emit('save')">
    <div class="group-dialog-content">
     <label for="group-dialog-value">目标分组：</label>
     <select id="group-dialog-value" v-model="model.groupId" :disabled="busy"><option value="">未分组（我的连接）</option><option v-for="g in groups" :key="g.id" :value="g.id">{{g.name}}</option></select>
     <p v-if="error" role="alert" class="group-dialog-error">{{error}}</p>
    </div>
    <footer><button type="submit" class="native-button default-button" :disabled="busy"><LoaderCircle v-if="busy" :size="13" class="spin"/>确定</button><button type="button" class="native-button" :disabled="busy" @click="emit('cancel')">取消</button></footer>
   </form>
  </section>
 </div>
</template>
