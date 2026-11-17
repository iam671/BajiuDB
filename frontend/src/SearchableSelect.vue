<script setup lang="ts">
import {computed,nextTick,onMounted,onUnmounted,ref,watch} from 'vue'
import {ChevronDown} from '@lucide/vue'
const props=defineProps<{id:string;label:string;modelValue:string;options:string[];disabled?:boolean}>()
const emit=defineEmits<{'update:modelValue':[value:string];change:[value:string]}>()
const open=ref(false),active=ref(-1),query=ref('')
const anchor=ref<HTMLElement>(),popupStyle=ref<Record<string,string>>({})
function position(){if(!anchor.value)return;const box=anchor.value.getBoundingClientRect(),space=window.innerHeight-box.bottom;popupStyle.value={position:'fixed',left:box.left+'px',width:box.width+'px',top:space<150&&box.top>150?Math.max(0,box.top-150)+'px':box.bottom+2+'px',maxHeight:Math.max(60,Math.min(145,space<150&&box.top>150?box.top-4:space-8))+'px'}}
watch(open,value=>{if(value)void nextTick(position)})
watch(active,()=>void nextTick(()=>document.getElementById(props.id+'-option-'+active.value)?.scrollIntoView?.({block:'nearest'})))
onMounted(()=>{window.addEventListener('resize',position);window.addEventListener('scroll',position,true)})
onUnmounted(()=>{window.removeEventListener('resize',position);window.removeEventListener('scroll',position,true)})
const matches=computed(()=>props.options.filter(value=>value.toLowerCase().includes(query.value.toLowerCase())))
function choose(value:string){emit('update:modelValue',value);emit('change',value);open.value=false;active.value=-1}
function toggle(){const next=!open.value;anchor.value?.querySelector('input')?.focus();query.value='';active.value=-1;open.value=next}
function input(event:Event){const value=(event.target as HTMLInputElement).value;emit('update:modelValue',value);emit('change',value);query.value=value;open.value=true;active.value=-1}
function keys(event:KeyboardEvent){
 if(event.key==='Escape'&&open.value){event.stopPropagation();event.preventDefault();open.value=false;return}
 if(event.key==='ArrowDown'||event.key==='ArrowUp'){event.preventDefault();open.value=true;active.value=Math.max(0,Math.min(matches.value.length-1,active.value+(event.key==='ArrowDown'?1:-1)))}
 if(event.key==='Enter'&&open.value&&active.value>=0&&matches.value[active.value]){event.preventDefault();choose(matches.value[active.value]!)}
}
</script>
<template><div class="searchable-select" @focusout="event=>{if(!(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node))open=false}">
 <div ref="anchor" class="searchable-select-input"><input :id="id" :aria-label="label" :value="modelValue" :disabled="disabled" role="combobox" aria-autocomplete="list" :aria-expanded="open" :aria-controls="id+'-options'" :aria-activedescendant="open&&active>=0?id+'-option-'+active:undefined" autocomplete="off" @input="input" @focus="query='';open=true" @keydown="keys"/><button type="button" :disabled="disabled" tabindex="-1" :aria-label="'展开'+label+'选项'" @mousedown.prevent @click="toggle"><ChevronDown :size="13"/></button></div>
 <ul v-if="open&&!disabled" :id="id+'-options'" role="listbox" :style="popupStyle"><li v-for="(value,index) in matches" :id="id+'-option-'+index" :key="value" role="option" :aria-selected="value===modelValue" :class="{active:index===active}" @mousedown.prevent="choose(value)">{{value}}</li><li v-if="!matches.length" class="empty-option">无匹配项，可继续输入</li></ul>
</div></template>
