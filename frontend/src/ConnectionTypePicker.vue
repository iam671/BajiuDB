<script setup lang="ts">
import {computed,nextTick,onMounted,ref} from 'vue'
import {Database,Layers,Leaf,File,Server,Search,X,Plug} from '@lucide/vue'
import {databaseEngines,loadRecentEngines} from './databaseEngines'
import {vDraggableDialog} from './draggableDialog'
const props=withDefaults(defineProps<{recent?:string[]}>(),{recent:()=>[]})
const emit=defineEmits<{choose:[engine:string];cancel:[]}>()
const query=ref(''),input=ref<HTMLInputElement>()
const ids=loadRecentEngines(props.recent)
const order=['mysql','postgresql','sqlserver','oracle','sqlite','mariadb','mongodb','redis']
const all=computed(()=>order.flatMap(id=>databaseEngines.filter(e=>e.value===id)).filter(matches))
const recent=computed(()=>ids.flatMap(id=>databaseEngines.filter(e=>e.value===id)).filter(matches))
function matches(engine:{value:string;label:string}){const term=query.value.trim().toLowerCase();return !term||(engine.value+' '+engine.label).toLowerCase().includes(term)}
const icons:Record<string,typeof Database>={mysql:Database,postgresql:Database,sqlserver:Server,oracle:Database,sqlite:File,mariadb:Database,mongodb:Leaf,redis:Layers}
const marks:Record<string,string>={mysql:'My',postgresql:'Pg',sqlserver:'SQL',oracle:'O',sqlite:'Lite',mariadb:'Ma',mongodb:'M',redis:'R'}
function chooseFirst(){if(all.value[0])emit('choose',all.value[0].value)}
function trap(event:KeyboardEvent){
 const nodes=Array.from((event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('button,input')).filter(e=>!e.hasAttribute('disabled')&&e.getClientRects().length>0)
 const first=nodes[0],last=nodes.at(-1)
 if(event.shiftKey&&document.activeElement===first){event.preventDefault();last?.focus()}else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first?.focus()}
}
onMounted(()=>void nextTick(()=>input.value?.focus()))
</script>
<template><div class="connection-type-overlay">
 <section v-draggable-dialog class="connection-type-dialog" role="dialog" aria-modal="true" aria-labelledby="connection-type-title" @keydown.tab="trap">
  <header class="dialog-titlebar connection-drag-handle"><Plug :size="15"/><h2 id="connection-type-title">新建连接</h2><button type="button" class="window-close" aria-label="关闭类型选择" @click="emit('cancel')"><X :size="16"/></button></header>
  <div class="connection-type-body">
   <div class="connection-type-heading"><h3>选择一个连接类型：</h3><label class="connection-type-search"><Search :size="14" aria-hidden="true"/><input ref="input" v-model="query" type="search" aria-label="搜索数据库类型" placeholder="搜索" @keydown.enter.prevent="chooseFirst"/></label></div>
   <section v-if="recent.length" class="connection-type-section" aria-labelledby="recent-types-title"><h3 id="recent-types-title">最近使用过的</h3><div class="connection-type-grid recent-types">
    <button v-for="engine in recent" :key="engine.value" type="button" class="connection-type-card" :data-engine="engine.value" @click="emit('choose',engine.value)"><span class="connection-type-symbol" :class="'engine-'+engine.value" aria-hidden="true"><component :is="icons[engine.value]" :size="28"/><small>{{marks[engine.value]}}</small></span><span>{{engine.label}}</span></button>
   </div></section>
   <section class="connection-type-section" aria-labelledby="all-types-title"><h3 id="all-types-title">全部</h3><div class="connection-type-grid">
    <button v-for="engine in all" :key="engine.value" type="button" class="connection-type-card" :data-engine="engine.value" @click="emit('choose',engine.value)"><span class="connection-type-symbol" :class="'engine-'+engine.value" aria-hidden="true"><component :is="icons[engine.value]" :size="28"/><small>{{marks[engine.value]}}</small></span><span>{{engine.label}}</span></button>
   </div><p v-if="!all.length" class="connection-type-empty" role="status">没有匹配的数据库类型</p></section>
  </div>
  <footer class="dialog-footer"><span class="dialog-footer-spacer"></span><button type="button" class="native-button" @click="emit('cancel')">取消</button></footer>
 </section>
</div></template>
