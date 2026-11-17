<script setup lang="ts">
import {computed,nextTick,ref,watch} from 'vue'
import {ChevronDown} from '@lucide/vue'
export type SelectOption=string|{value:string;label:string}
const props=withDefaults(defineProps<{id:string;label:string;modelValue:string;options:SelectOption[];disabled?:boolean;allowCustom?:boolean;placeholder?:string}>(),{allowCustom:true})
const emit=defineEmits<{'update:modelValue':[value:string];change:[value:string]}>()
const open=ref(false),active=ref(-1),query=ref(''),typing=ref(false)
const root=ref<HTMLElement>(),anchor=ref<HTMLElement>(),popupStyle=ref<Record<string,string>>({})
const items=computed(()=>props.options.map(option=>typeof option==='string'?{value:option,label:option}:option))
const display=computed(()=>typing.value?query.value:items.value.find(item=>item.value===props.modelValue)?.label??props.modelValue)
const matches=computed(()=>items.value.filter(item=>(item.label+' '+item.value).toLowerCase().includes(query.value.toLowerCase())))
function position(){
 if(!open.value||!anchor.value)return
 const box=anchor.value.getBoundingClientRect(),space=window.innerHeight-box.bottom
 // Preview dialogs use transforms for dragging; fixed popups are then relative to that ancestor.
 let context=anchor.value.parentElement
 while(context){const transform=getComputedStyle(context).transform;if(transform&&transform!=='none')break;context=context.parentElement}
 const origin=context?.getBoundingClientRect(),left=(origin?.left??0)+(context?.clientLeft??0),top=(origin?.top??0)+(context?.clientTop??0)
 popupStyle.value={position:'fixed',left:box.left-left+'px',width:box.width+'px',top:(space<150&&box.top>150?Math.max(0,box.top-150):box.bottom+2)-top+'px',maxHeight:Math.max(60,Math.min(145,space<150&&box.top>150?box.top-4:space-8))+'px'}
}
function close(){open.value=false}
function outside(event:Event){if(!(event.target instanceof Node&&root.value?.contains(event.target)))close()}
watch(open,(value,_,cleanup)=>{if(!value){typing.value=false;return}void nextTick(position);window.addEventListener('resize',position);window.addEventListener('scroll',position,true);window.addEventListener('blur',close);document.addEventListener('pointerdown',outside,true);document.addEventListener('contextmenu',outside,true);cleanup(()=>{window.removeEventListener('resize',position);window.removeEventListener('scroll',position,true);window.removeEventListener('blur',close);document.removeEventListener('pointerdown',outside,true);document.removeEventListener('contextmenu',outside,true)})})
watch(active,()=>void nextTick(()=>document.getElementById(props.id+'-option-'+active.value)?.scrollIntoView?.({block:'nearest'})))
watch(()=>props.disabled,value=>{if(value)open.value=false})
function commit(value:string){emit('update:modelValue',value);emit('change',value)}
function choose(value:string){commit(value);open.value=false;typing.value=false;active.value=-1}
function toggle(){const next=!open.value;anchor.value?.querySelector('input')?.focus();query.value='';typing.value=false;active.value=-1;open.value=next}
function input(event:Event){const value=(event.target as HTMLInputElement).value;query.value=value;typing.value=true;open.value=true;active.value=-1;const match=items.value.find(item=>item.label.toLowerCase()===value.toLowerCase()||item.value.toLowerCase()===value.toLowerCase());if(match)commit(match.value);else if(props.allowCustom)commit(value)}
function keys(event:KeyboardEvent){
 if(event.key==='Escape'&&open.value){event.stopPropagation();event.preventDefault();open.value=false;return}
 if(event.key==='ArrowDown'||event.key==='ArrowUp'){event.preventDefault();open.value=true;active.value=Math.max(0,Math.min(matches.value.length-1,active.value+(event.key==='ArrowDown'?1:-1)))}
 if(event.key==='Enter'&&open.value){event.preventDefault();const match=matches.value[active.value]??(matches.value.length===1?matches.value[0]:undefined);if(match)choose(match.value);else if(props.allowCustom){choose(query.value||props.modelValue)}else open.value=false}
}
</script>
<template><div ref="root" class="searchable-select" :class="{'is-open':open,'is-disabled':disabled}" @focusout="event=>{if(!(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node))open=false}">
 <div ref="anchor" class="searchable-select-input"><input :id="id" :aria-label="label" :value="display" :placeholder="placeholder" :disabled="disabled" role="combobox" aria-autocomplete="list" :aria-expanded="open" :aria-controls="id+'-options'" :aria-activedescendant="open&&active>=0?id+'-option-'+active:undefined" autocomplete="off" @input="input" @focus="query='';typing=false;open=true" @keydown="keys"/><button type="button" :disabled="disabled" :class="{active:open}" :aria-expanded="open" :aria-controls="id+'-options'" tabindex="-1" :aria-label="'展开'+label+'选项'" @mousedown.prevent @click="toggle"><ChevronDown :size="12" :stroke-width="1.5"/></button></div>
 <ul v-if="open&&!disabled" :id="id+'-options'" role="listbox" :style="popupStyle"><li v-for="(item,index) in matches" :id="id+'-option-'+index" :key="item.value" role="option" :aria-selected="item.value===modelValue" :class="{active:index===active}" @mousedown.prevent="choose(item.value)">{{item.label}}</li><li v-if="!matches.length" class="empty-option">{{allowCustom?'无匹配项，可继续输入':'无匹配项，请输入匹配的选项'}}</li></ul>
</div></template>
