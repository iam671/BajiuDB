<script setup lang="ts">
import {computed,nextTick,onMounted,ref} from 'vue'
import {ArrowUp,ArrowDown,Plus,Minus,X} from '@lucide/vue'
import {vDraggableDialog} from './draggableDialog'
import SearchableSelect from './SearchableSelect.vue'
import {indexPrefixField,indexFieldSelectionSQL,parseIndexFieldSelection,type IndexFieldOption,type IndexFieldSelection,type DesignIndex} from './tableDesigner'
export type IndexFieldsPayload={fields:IndexFieldOption[];value:string;kind:DesignIndex['kind'];method:string;sqlMode:string}
export type IndexFieldsRequest=IndexFieldsPayload & {apply:(value:string)=>void}
const props=defineProps<IndexFieldsPayload>()
const emit=defineEmits<{apply:[value:string];cancel:[]}>()
type Row=IndexFieldSelection & {checked:boolean;missing:boolean}
const root=ref<HTMLElement>(),rows=ref<Row[]>([]),selected=ref(''),error=ref(''),unreadable=ref(false)
const directions=[{value:'',label:'默认'},{value:'ASC',label:'ASC'},{value:'DESC',label:'DESC'}]
try{
 const current=parseIndexFieldSelection(props.value,props.sqlMode),seen=new Set<string>()
 for(const item of current){if(seen.has(item.name))throw new Error('索引包含重复字段，请直接编辑字段文本。');seen.add(item.name);rows.value.push({...item,checked:true,missing:!props.fields.some(field=>field.name===item.name)})}
 for(const field of props.fields)if(!seen.has(field.name)){seen.add(field.name);rows.value.push({name:field.name,prefix:'',order:'',checked:false,missing:false})}
 selected.value=rows.value[0]?.name??''
}catch(e){unreadable.value=true;error.value=String(e instanceof Error?e.message:e)}
const checked=computed(()=>rows.value.filter(row=>row.checked))
const canOrder=computed(()=>props.method!=='HASH'&&props.kind!=='SPATIAL'&&props.kind!=='FULLTEXT')
function prefixAllowed(row:Row){const field=props.fields.find(field=>field.name===row.name);return !!field&&indexPrefixField(field)&&canOrder.value}
function neighbor(direction:number){const index=checked.value.findIndex(row=>row.name===selected.value);return index<0?undefined:checked.value[index+direction]}
function move(direction:number){const other=neighbor(direction);if(!other)return;const from=rows.value.findIndex(row=>row.name===selected.value),to=rows.value.indexOf(other),row=rows.value[from]!;rows.value.splice(from,1);rows.value.splice(to,0,row);error.value=''}
function chooseAll(value:boolean){for(const row of rows.value)if(!row.missing||!value)row.checked=value;error.value=''}
function accept(){try{const value=indexFieldSelectionSQL(checked.value,props.fields,props.kind,props.method);emit('apply',value)}catch(e){error.value=e instanceof Error?e.message:String(e)}}
function keys(event:KeyboardEvent){if(event.key==='Escape'){event.stopPropagation();event.preventDefault();emit('cancel')}else if(event.key==='Tab'){const nodes=Array.from(root.value?.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled)')??[]);if(event.shiftKey&&document.activeElement===nodes[0]){event.preventDefault();nodes.at(-1)?.focus()}else if(!event.shiftKey&&document.activeElement===nodes.at(-1)){event.preventDefault();nodes[0]?.focus()}}}
onMounted(()=>void nextTick(()=>root.value?.querySelector<HTMLInputElement>('input[type=checkbox]')?.focus()))
</script>
<template><div class="index-fields-overlay" @keydown="keys"><section ref="root" v-draggable-dialog class="index-fields-dialog" role="dialog" aria-modal="true" aria-labelledby="index-fields-title">
 <header><h2 id="index-fields-title">选择索引字段</h2><button class="tiny-button" aria-label="关闭索引字段选择" @click="emit('cancel')"><X :size="14"/></button></header>
 <div class="index-fields-list"><table class="designer-grid index-fields-grid" aria-label="索引字段选择"><colgroup><col style="width:30px"/><col style="width:200px"/><col style="width:80px"/><col style="width:100px"/></colgroup><thead><tr><th></th><th>名称</th><th>子部分</th><th>排序顺序</th></tr></thead><tbody><tr v-for="row in rows" :key="row.name" :class="{selected:selected===row.name,missing:row.missing}" @click="selected=row.name" @focusin="selected=row.name"><td class="designer-check"><input v-model="row.checked" type="checkbox" :aria-label="'选择字段 '+row.name" @change="error=''"/></td><td :title="row.missing?'字段已不存在，请取消选择':row.name"><label class="index-field-name" @click="selected=row.name">{{row.name}}<span v-if="row.missing">（已移除）</span></label></td><td><input v-model="row.prefix" :aria-label="row.name+'子部分'" inputmode="numeric" :disabled="!row.checked||!prefixAllowed(row)" @input="error=''"/></td><td><SearchableSelect :id="'index-field-order-'+row.name" :label="row.name+'排序顺序'" v-model="row.order" :options="directions" :allowCustom="false" :disabled="!row.checked||!canOrder"/></td></tr></tbody></table><p v-if="!rows.length&&!unreadable" class="index-fields-empty">请先在字段页添加并命名字段。</p></div>
 <p v-if="error" class="index-fields-error" role="alert">{{error}}</p>
 <footer><div class="index-fields-actions"><button class="tiny-button" title="上移所选字段" aria-label="上移所选字段" :disabled="!neighbor(-1)" @click="move(-1)"><ArrowUp :size="13"/></button><button class="tiny-button" title="下移所选字段" aria-label="下移所选字段" :disabled="!neighbor(1)" @click="move(1)"><ArrowDown :size="13"/></button><button class="tiny-button" title="选择全部字段" aria-label="选择全部字段" :disabled="unreadable||!rows.some(row=>!row.checked&&!row.missing)" @click="chooseAll(true)"><Plus :size="13"/></button><button class="tiny-button" title="清空字段选择" aria-label="清空字段选择" :disabled="!checked.length" @click="chooseAll(false)"><Minus :size="13"/></button></div><button class="native-button default-button" :disabled="unreadable||!checked.length" @click="accept">确定</button><button class="native-button" @click="emit('cancel')">取消</button></footer>
</section></div></template>
