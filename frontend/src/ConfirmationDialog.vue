<script setup lang="ts">
import {onMounted,nextTick} from 'vue'
import {AlertCircle} from '@lucide/vue'
import {vDraggableDialog} from './draggableDialog'
defineProps<{title:string;body:string;label:string;items?:string[];notice?:boolean;details?:string;sql?:string}>()
const emit=defineEmits<{cancel:[];accept:[]}>()
function trap(e:KeyboardEvent){const nodes=Array.from((e.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('button:not([disabled]),textarea,[tabindex="0"]'));if(e.shiftKey&&document.activeElement===nodes[0]){e.preventDefault();nodes.at(-1)?.focus()}else if(!e.shiftKey&&document.activeElement===nodes.at(-1)){e.preventDefault();nodes[0]?.focus()}}
onMounted(()=>nextTick(()=>document.querySelector<HTMLButtonElement>('[data-confirm-cancel]')?.focus()))
</script>
<template><div class="confirmation-overlay" @keydown.tab="trap" @keydown.esc.stop.prevent="emit('cancel')"><section v-draggable-dialog class="confirmation-dialog" :class="{'detail-dialog':notice,'list-dialog':!!items?.length}" role="alertdialog" aria-modal="true" aria-labelledby="confirmation-title" aria-describedby="confirmation-text"><header><AlertCircle :size="16"/><h2 id="confirmation-title">{{title}}</h2></header><p id="confirmation-text">{{body}}</p><ul v-if="items?.length" class="confirmation-items" tabindex="0" aria-label="将删除的表"><li v-for="item in items" :key="item">{{item}}</li></ul><div v-if="notice" class="confirmation-details"><label>详细信息</label><pre tabindex="0">{{details}}</pre><template v-if="sql"><label for="dialog-detail-sql">相关 SQL</label><textarea id="dialog-detail-sql" :value="sql" readonly spellcheck="false"/></template></div><footer><button class="native-button" :class="{'default-button':notice}" data-confirm-cancel @click="emit('cancel')">{{notice?'关闭':'取消'}}</button><button v-if="!notice" class="native-button default-button" @click="emit('accept')">{{label}}</button></footer></section></div></template>
