<script setup lang="ts">
import {computed,nextTick,onMounted,ref} from 'vue'
import {ChevronLeft,ChevronRight,X} from '@lucide/vue'
import {vDraggableDialog} from './draggableDialog'
import SearchableSelect from './SearchableSelect.vue'
import {cellValueError,enumValues,formatJSON,localDate,localTime,quickKind,stepNumber,validDate,type CellValuePayload} from './cellValueTools'
import type {CellDraft} from './tableEditing'
const props=defineProps<CellValuePayload>()
const emit=defineEmits<{apply:[cell:CellDraft];cancel:[]}>()
const kind=quickKind(props.column),cell=ref<CellDraft>({...props.cell}),error=ref(''),root=ref<HTMLElement>()
const initial=props.cell.mode==='value'?props.cell.value:'',date=ref(validDate(initial.slice(0,10))?initial.slice(0,10):localDate()),time=ref(kind==='time'?initial||'00:00:00':initial.slice(11)||'00:00:00')
const year=ref(date.value.slice(0,4)),month=ref(String(Number(date.value.slice(5,7))))
const options=enumValues(props.column.type),selected=ref(initial?initial.split(','):[])
const years=Array.from({length:41},(_,i)=>String(Number(year.value)-20+i)).filter(y=>Number(y)>=1000&&Number(y)<=9999)
const months=Array.from({length:12},(_,i)=>({value:String(i+1),label:(i+1)+' 月'}))
const calendar=computed(()=>{
 const y=Number(year.value),m=Number(month.value);if(!Number.isInteger(y)||y<1000||y>9999||!Number.isInteger(m)||m<1||m>12)return []
 const first=new Date(y,m-1,1),offset=(first.getDay()+6)%7
 return Array.from({length:42},(_,i)=>{const d=new Date(y,m-1,1-offset+i);return {value:localDate(d),day:d.getDate(),outside:d.getMonth()!==m-1,disabled:d.getFullYear()<1000||d.getFullYear()>9999}})
})
function setValue(value:string){cell.value={mode:'value',value};error.value=''}
function input(event:Event){setValue((event.target as HTMLInputElement).value)}
function chooseDate(value:string){date.value=value;year.value=value.slice(0,4);month.value=String(Number(value.slice(5,7)));error.value=''}
function moveMonth(delta:number){const y=Number(year.value),m=Number(month.value);if(!Number.isInteger(y)||!Number.isInteger(m))return;const d=new Date(y,m-1+delta,1);if(d.getFullYear()<1000||d.getFullYear()>9999)return;year.value=String(d.getFullYear());month.value=String(d.getMonth()+1)}
function calendarKeys(event:KeyboardEvent){const delta=({ArrowLeft:-1,ArrowRight:1,ArrowUp:-7,ArrowDown:7} as Record<string,number>)[event.key];if(delta===undefined||!validDate(date.value))return;event.preventDefault();const parts=date.value.split('-').map(Number),d=new Date(parts[0]!,parts[1]!-1,parts[2]!+delta),value=localDate(d);if(!validDate(value))return;chooseDate(value);void nextTick(()=>root.value?.querySelector<HTMLElement>('.cell-calendar button[aria-pressed="true"]')?.focus())}
function now(){const current=new Date();if(kind==='date'||kind==='datetime')chooseDate(localDate(current));if(kind==='datetime'||kind==='time')time.value=localTime(props.column.type,current);if(kind==='year')setValue(String(current.getFullYear()));error.value=''}
function toggle(value:string,event:Event){if((event.target as HTMLInputElement).checked)selected.value.push(value);else selected.value=selected.value.filter(v=>v!==value);setValue(options.filter(v=>selected.value.includes(v)).join(','))}
function step(delta:1|-1){try{setValue(stepNumber(cell.value.value||'0',delta))}catch(e){error.value=String(e)}}
function pretty(){try{setValue(formatJSON(cell.value.value))}catch(e){error.value=String(e)}}
function uuid(){const length=Number(props.column.type.match(/\((\d+)\)/)?.[1]??36);setValue(length===32?crypto.randomUUID().replace(/-/g,''):crypto.randomUUID())}
function save(){const value:CellDraft=kind==='date'?{mode:'value',value:date.value}:kind==='datetime'?{mode:'value',value:date.value+' '+time.value}:kind==='time'?{mode:'value',value:time.value}:{...cell.value};error.value=cellValueError({...props,cell:value});if(!error.value)emit('apply',value)}
function keys(event:KeyboardEvent){if(event.key==='Escape'){event.preventDefault();event.stopPropagation();emit('cancel')}if((event.ctrlKey||event.metaKey)&&event.key==='Enter'){event.preventDefault();save()}if(event.key==='Tab'){const nodes=Array.from(root.value?.querySelectorAll<HTMLElement>('button:not(:disabled):not([tabindex="-1"]),input:not(:disabled),textarea:not(:disabled)')??[]);if(event.shiftKey&&document.activeElement===nodes[0]){event.preventDefault();nodes.at(-1)?.focus()}else if(!event.shiftKey&&document.activeElement===nodes.at(-1)){event.preventDefault();nodes[0]?.focus()}}}
onMounted(()=>void nextTick(()=>{const focus=root.value?.querySelector<HTMLElement>('.cell-calendar button[aria-pressed="true"]')??root.value?.querySelector<HTMLElement>('.cell-quick-body input,.cell-quick-body textarea');focus?.focus()}))
</script>
<template><div class="cell-value-overlay" @keydown="keys"><section ref="root" v-draggable-dialog class="cell-value-dialog" :class="{'wide-editor':['json','text','binary'].includes(kind)}" role="dialog" aria-modal="true" aria-labelledby="cell-value-title">
 <header><h2 id="cell-value-title">编辑 {{column.name}}</h2><button class="tiny-button" aria-label="关闭快捷编辑" @click="emit('cancel')"><X :size="14"/></button></header>
 <div class="cell-quick-body"><p class="cell-quick-type">{{column.name}} <span>{{column.type}}{{column.binary?' · HEX':''}}</span></p>
  <template v-if="kind==='date'||kind==='datetime'">
   <div class="cell-calendar-nav"><button class="tiny-button" aria-label="上个月" @click="moveMonth(-1)"><ChevronLeft :size="15"/></button><SearchableSelect id="calendar-year" label="年份" v-model="year" :options="years"/><SearchableSelect id="calendar-month" label="月份" v-model="month" :options="months" :allowCustom="false"/><button class="tiny-button" aria-label="下个月" @click="moveMonth(1)"><ChevronRight :size="15"/></button></div>
   <div class="cell-calendar" role="group" aria-label="选择日期" @keydown="calendarKeys"><span v-for="day in ['一','二','三','四','五','六','日']" :key="day" class="weekday">{{day}}</span><button v-for="day in calendar" :key="day.value" :disabled="day.disabled" :tabindex="day.value===date?0:-1" :class="{outside:day.outside,today:day.value===localDate()}" :aria-label="day.value" :aria-pressed="day.value===date" @click="chooseDate(day.value)">{{day.day}}</button></div>
   <label class="cell-quick-line">日期<input v-model="date" aria-label="日期值" placeholder="YYYY-MM-DD"/></label>
  </template>
  <label v-if="kind==='datetime'||kind==='time'" class="cell-quick-line">时间<input v-model="time" aria-label="时间值" :placeholder="kind==='time'?'HH:MM:SS':'00:00:00'" spellcheck="false"/></label>
  <SearchableSelect v-if="kind==='enum'" id="cell-enum" label="枚举值" :modelValue="cell.value" :options="options" :allowCustom="false" @update:modelValue="setValue"/>
  <div v-else-if="kind==='set'" class="cell-set-options"><label v-for="value in options" :key="value"><input type="checkbox" :checked="selected.includes(value)" @change="toggle(value,$event)"/>{{value||'(空字符串)'}}</label></div>
  <SearchableSelect v-else-if="kind==='boolean'" id="cell-boolean" label="布尔值" :modelValue="cell.value" :options="column.binary?[{value:'00',label:'0 · 关'},{value:'01',label:'1 · 开'}]:[{value:'0',label:'0 · 关'},{value:'1',label:'1 · 开'}]" :allowCustom="!column.binary" @update:modelValue="setValue"/>
  <input v-else-if="kind==='year'||kind==='number'" class="cell-quick-input" :value="cell.mode==='value'?cell.value:''" :aria-label="kind==='year'?'年份值':'数值'" :placeholder="cell.mode==='value'?'':cell.mode==='null'?'NULL':'数据库默认值'" @input="input"/>
  <textarea v-else-if="['json','text','binary'].includes(kind)" class="cell-quick-text" :value="cell.mode==='value'?cell.value:''" aria-label="完整值" :placeholder="cell.mode==='value'?'':cell.mode==='null'?'NULL':'数据库默认值'" spellcheck="false" @input="input"/>
  <div class="cell-quick-actions"><button v-if="['date','datetime','time','year'].includes(kind)" class="native-button" @click="now">{{kind==='date'?'今天':kind==='year'?'今年':'当前时间'}}</button><template v-if="kind==='number'"><button class="native-button" @click="step(-1)">−1</button><button class="native-button" @click="step(1)">+1</button><button class="native-button" @click="setValue('0')">设为 0</button></template><button v-if="kind==='json'" class="native-button" @click="pretty">校验并格式化</button><button v-if="kind==='text'&&(/char|text/i.test(column.type))&&(Number(column.type.match(/\((\d+)\)/)?.[1]??36)===32||Number(column.type.match(/\((\d+)\)/)?.[1]??36)>=36)" class="native-button" @click="uuid">生成 UUID</button></div>
  <p v-if="error" class="cell-quick-error" role="alert">{{error}}</p>
 </div><footer><span>确认后仍需保存记录</span><button class="native-button default-button" @click="save">确定</button><button class="native-button" @click="emit('cancel')">取消</button></footer>
</section></div></template>
