<script setup lang="ts">
import {onMounted,nextTick} from 'vue'
import {AlertCircle} from '@lucide/vue'
import {vDraggableDialog} from './draggableDialog'
defineProps<{title:string;body:string;label:string}>()
const emit=defineEmits<{cancel:[];accept:[]}>()
function trap(e:KeyboardEvent){const nodes=Array.from((e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('button'));if(e.shiftKey&&document.activeElement===nodes[0]){e.preventDefault();nodes.at(-1)?.focus()}else if(!e.shiftKey&&document.activeElement===nodes.at(-1)){e.preventDefault();nodes[0]?.focus()}}
onMounted(()=>nextTick(()=>document.querySelector<HTMLButtonElement>('[data-confirm-cancel]')?.focus()))
</script>
<template><div class="confirmation-overlay" @keydown.tab="trap" @keydown.esc.stop.prevent="emit('cancel')"><section v-draggable-dialog class="confirmation-dialog" role="alertdialog" aria-modal="true" aria-labelledby="confirmation-title" aria-describedby="confirmation-text"><header><AlertCircle :size="16"/><h2 id="confirmation-title">{{title}}</h2></header><p id="confirmation-text">{{body}}</p><footer><button class="native-button" data-confirm-cancel @click="emit('cancel')">取消</button><button class="native-button default-button" @click="emit('accept')">{{label}}</button></footer></section></div></template>
