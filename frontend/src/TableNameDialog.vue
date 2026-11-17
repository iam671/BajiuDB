<script setup lang="ts">
import {nextTick,onMounted,ref} from 'vue'
import {X} from '@lucide/vue'
import {vDraggableDialog} from './draggableDialog'
export type TableNameRequest={value:string;save:(name:string)=>void|Promise<void>}
const props=defineProps<{value:string}>()
const emit=defineEmits<{save:[name:string];cancel:[]}>()
const name=ref(props.value),error=ref(''),input=ref<HTMLInputElement>()
function save(){const value=name.value.trim();if(!value||Array.from(value).length>64||/[\0\r\n]/.test(value)){error.value='表名须为 1–64 个字符，不能包含换行或空字符。';return}emit('save',value)}
function keys(event:KeyboardEvent){if(event.key==='Escape'){event.preventDefault();event.stopPropagation();emit('cancel')}else if(event.key==='Tab'){const nodes=Array.from((event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('button,input'));if(event.shiftKey&&document.activeElement===nodes[0]){event.preventDefault();nodes.at(-1)?.focus()}else if(!event.shiftKey&&document.activeElement===nodes.at(-1)){event.preventDefault();nodes[0]?.focus()}}}
onMounted(()=>void nextTick(()=>input.value?.focus()))
</script>
<template><div class="table-name-overlay" @keydown="keys"><section v-draggable-dialog class="table-name-dialog" role="dialog" aria-modal="true" aria-labelledby="table-name-title"><header><h2 id="table-name-title">保存新建表</h2><button class="tiny-button" aria-label="关闭表名输入" @click="emit('cancel')"><X :size="14"/></button></header><form @submit.prevent="save"><div class="table-name-content"><label for="new-table-name">表名</label><input id="new-table-name" ref="input" v-model="name" autocomplete="off" maxlength="64" @input="error=''"/><p v-if="error" class="index-fields-error" role="alert">{{error}}</p></div><footer><button class="native-button default-button" type="submit">保存</button><button class="native-button" type="button" @click="emit('cancel')">取消</button></footer></form></section></div></template>
