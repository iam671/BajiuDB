<script setup lang="ts">
import {computed,nextTick,onMounted,onUnmounted,ref,watch} from 'vue'
import {Plus,Save,Trash2,RefreshCw,Search,ChevronLeft,ChevronRight,ChevronsLeft,LayoutGrid,List,Undo2,LoaderCircle,Square,Columns3,SlidersHorizontal,PanelRight,GitBranch,Check,RotateCcw,X} from '@lucide/vue'
import {api,errorText,type AdminRequest,type AdminResult,type Profile,type QueryResult} from './api'
import {deleteRowSQL,makeDraft,metadataSQL,pageSQL,readColumns,readIndexes,saveRowSQL,uniqueKey,type CellDraft,type TableColumn,type TableFilter,type TableIndex,type TableSection} from './tableEditing'
import {clampColumnWidth,defaultColumnWidth,readColumnLayout} from './tablePresentation'
import {cellClipboardText,copyTableRecord,type TableCopyFormat} from './tableClipboard'
import {useMenuDismissal} from './menuDismissal'
const instanceID=crypto.randomUUID()
const props=defineProps<{profile:Profile;database:string;table:string;section:TableSection;active:boolean;connected:boolean;sessionKey:string}>()
type Confirmation={title:string;body:string;label:string;action:()=>void|Promise<void>}
const emit=defineEmits<{confirm:[request:Confirmation];dirty:[value:boolean];section:[value:TableSection]}>()
class StatementError extends Error{}
const sections:[TableSection,string][]=[['data','数据'],['fields','字段'],['indexes','索引'],['foreignKeys','外键'],['triggers','触发器']]
const section=ref<TableSection>(props.section),view=ref<'form'|'grid'>('grid'),running=ref(false),error=ref(''),message=ref(''),columns=ref<TableColumn[]>([]),indexes=ref<TableIndex[]>([]),foreignKeys=ref<QueryResult>(),triggers=ref<QueryResult>(),data=ref<QueryResult>(),page=ref(1),pageInput=ref(1),selectedIndex=ref(-1),original=ref<(string|null)[]>(),draft=ref<CellDraft[]>([]),isNew=ref(false),baseline=ref('[]'),uncertain=ref(false),sessionChanged=ref(false),sort=ref(''),descending=ref(false)
const selectedColumn=ref<number|null>(null),editingColumn=ref<number|null>(null)
let cellBeforeEdit:CellDraft|undefined
type CellRow=number|'new'
const filter=ref<TableFilter>({column:'',operator:'contains',value:''}),appliedFilter=ref<TableFilter>({...filter.value})
const keys=computed(()=>uniqueKey(columns.value,indexes.value)),rows=computed(()=>data.value?.rows??[]),hasNext=computed(()=>!!data.value?.truncated),dirty=computed(()=>isNew.value||JSON.stringify(draft.value)!==baseline.value)
const hasRecord=computed(()=>draft.value.length===columns.value.length&&columns.value.length>0&&(isNew.value||!!original.value))
const canWrite=computed(()=>props.connected&&!running.value&&!uncertain.value&&!sessionChanged.value&&hasRecord.value&&(isNew.value||keys.value.length>0))
const transactionID=ref(''),transactionPending=ref(false),transactionUncertain=ref(false)
const showFilters=ref(false),showColumns=ref(false),showCellEditor=ref(false),filterSort=ref(''),filterDescending=ref(false)
const columnWidths=ref<Record<string,number>>(Object.create(null)),hiddenColumns=ref(new Set<string>()),layoutWarning=ref(''),cellMenu=ref<{row:CellRow;column:number;x:number;y:number}|null>(null),cellMenuRef=ref<HTMLElement>()
const copyAsOpen=ref(false)
const copySubStyle=computed(()=>{const menu=cellMenu.value;if(!menu)return {};return {left:Math.max(4,menu.x+200+260>window.innerWidth?menu.x-258:menu.x+198)+'px',top:Math.max(4,Math.min(menu.y+34,window.innerHeight-168))+'px'}})
const copyFormats:[TableCopyFormat,string][]=[['insert','Insert 语句'],['update','Update 语句'],['tsv-data','制表符分隔值（数据）'],['tsv-fields','制表符分隔值（字段名称）'],['tsv-all','制表符分隔值（字段名称和数据）']]
const layoutKey='bajiu.table-layout.v1.'+JSON.stringify([props.profile.id,props.database,props.table])
const visibleColumns=computed(()=>columns.value.map((column,index)=>({column,index})).filter(item=>!hiddenColumns.value.has(item.column.name)))
const gridWidth=computed(()=>44+visibleColumns.value.reduce((sum,item)=>sum+columnWidth(item.column),0))
const selectedField=computed(()=>selectedColumn.value===null?undefined:columns.value[selectedColumn.value])
let releaseColumnResize:(()=>void)|undefined
function columnWidth(column:TableColumn){return columnWidths.value[column.name]??defaultColumnWidth(column)}
function restoreColumnLayout(){releaseColumnResize?.();const layout=readColumnLayout(layoutKey,columns.value);columnWidths.value=layout.widths;hiddenColumns.value=new Set(layout.hidden)}
function persistColumnLayout(){try{localStorage.setItem(layoutKey,JSON.stringify({version:1,widths:columnWidths.value,hidden:[...hiddenColumns.value]}));layoutWarning.value=''}catch{layoutWarning.value='列布局已应用，但当前环境无法保存，下次打开可能恢复默认。'}}
function resizeColumn(event:PointerEvent,column:TableColumn){
 if(event.button!==0)return;event.preventDefault();event.stopPropagation();releaseColumnResize?.();cellMenu.value=null
 const start=event.clientX,width=columnWidth(column),handle=event.currentTarget as HTMLElement;handle.setPointerCapture?.(event.pointerId)
 const move=(e:PointerEvent)=>{columnWidths.value[column.name]=clampColumnWidth(width+e.clientX-start)}
 const finish=()=>{document.removeEventListener('pointermove',move);document.removeEventListener('pointerup',finish);document.removeEventListener('pointercancel',finish);try{handle.releasePointerCapture?.(event.pointerId)}catch{}document.body.style.cursor='';releaseColumnResize=undefined;persistColumnLayout()}
 document.addEventListener('pointermove',move);document.addEventListener('pointerup',finish,{once:true});document.addEventListener('pointercancel',finish,{once:true});document.body.style.cursor='col-resize';releaseColumnResize=finish
}
function resizeColumnKeys(event:KeyboardEvent,column:TableColumn){if(!['ArrowLeft','ArrowRight'].includes(event.key))return;event.preventDefault();event.stopPropagation();columnWidths.value[column.name]=clampColumnWidth(columnWidth(column)+(event.key==='ArrowRight'?12:-12));persistColumnLayout()}
function resetColumnWidth(column?:TableColumn){if(column)delete columnWidths.value[column.name];else columnWidths.value=Object.create(null);persistColumnLayout()}
function setColumnWidth(column:TableColumn,event:Event){const input=event.target as HTMLInputElement,width=Number(input.value);if(Number.isFinite(width)){columnWidths.value[column.name]=clampColumnWidth(width);persistColumnLayout()}input.value=String(columnWidth(column))}
function toggleColumn(column:TableColumn){if(hiddenColumns.value.has(column.name))hiddenColumns.value.delete(column.name);else if(visibleColumns.value.length>1)hiddenColumns.value.add(column.name);persistColumnLayout()}
function allColumns(){hiddenColumns.value.clear();persistColumnLayout()}
function gridCellText(row:CellRow,column:number){return cellText(row,column).replace(/\r\n|\r|\n/g,' ')}
function toggleCellEditor(){finishCellEdit();if(!showCellEditor.value&&selectedColumn.value===null&&columns.value.length)selectedColumn.value=0;showCellEditor.value=!showCellEditor.value;showColumns.value=false}
function sideInput(event:Event){const index=selectedColumn.value,field=selectedField.value;if(index===null||!field||!editable(field))return;draft.value[index]={mode:'value',value:(event.target as HTMLTextAreaElement).value}}
function sideColumn(event:Event){finishCellEdit();selectedColumn.value=Number((event.target as HTMLSelectElement).value)}
function closeContext(){copyAsOpen.value=false;cellMenu.value=null}
useMenuDismissal(()=>cellMenu.value?cellMenuRef.value:undefined,closeContext)
function contextCell(event:MouseEvent,row:CellRow,column:number){
 event.preventDefault();event.stopPropagation();if(running.value)return
 const open=()=>{if(!currentRow(row)){if(row==='new')return;pick(row)}finishCellEdit();copyAsOpen.value=false;selectedColumn.value=column;cellMenu.value={row,column,x:Math.max(4,Math.min(event.clientX,window.innerWidth-210)),y:Math.max(4,Math.min(event.clientY,window.innerHeight-205))};void nextTick(()=>cellMenuRef.value?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus())}
 if(currentRow(row))open();else guard(open)
}
const menuWritable=computed(()=>{const m=cellMenu.value,c=m?columns.value[m.column]:undefined;return !!m&&currentRow(m.row)&&!!c&&editable(c)})
const menuUUID=computed(()=>{const m=cellMenu.value,c=m?columns.value[m.column]:undefined;return menuWritable.value&&!!c&&/char|text|json|binary|blob/i.test(c.type)})
function contextAction(action:'empty'|'null'|'uuid'|'refresh'){
 const menu=cellMenu.value;if(!menu)return
 const column=columns.value[menu.column],writable=menuWritable.value,canUUID=menuUUID.value;closeContext()
 if(action==='refresh'){refresh();return}if(!writable||!column)return
 if(action==='null'){if(column.nullable)draft.value[menu.column]={mode:'null',value:''}}
 else if(action==='empty')draft.value[menu.column]={mode:'value',value:''}
 else if(canUUID){const uuid=crypto.randomUUID(),length=Number(column.type.match(/\((\d+)\)/)?.[1]??36);draft.value[menu.column]={mode:'value',value:column.binary?uuid.replace(/-/g,'').toUpperCase():/^json/i.test(column.type)?JSON.stringify(uuid):length===32?uuid.replace(/-/g,''):uuid}}
}
function focusCopyTrigger(){cellMenuRef.value?.querySelector<HTMLButtonElement>('[data-copy-trigger]')?.focus()}
function openCopySubmenu(focus=false){copyAsOpen.value=true;if(focus)void nextTick(()=>cellMenuRef.value?.querySelector<HTMLButtonElement>('[data-cell-menu-copy]:not(:disabled)')?.focus())}
async function copyCell(format?:TableCopyFormat){
 const menu=cellMenu.value;if(!menu)return
 try{
  const cells=columns.value.map((_,i)=>({...cellValue(menu.row,i)})),originalRow=menu.row==='new'?undefined:currentRow(menu.row)?original.value:rows.value[menu.row]
  const text=format?copyTableRecord(format,props.database,props.table,columns.value,cells,keys.value,originalRow,visibleColumns.value.map(item=>item.index)):cellClipboardText(cells[menu.column]!)
  closeContext();await navigator.clipboard.writeText(text);if(!disposed)message.value=format?'已复制为'+(copyFormats.find(item=>item[0]===format)?.[1]??'指定格式'):'已复制单元格数据'
 }catch(e){closeContext();if(!disposed)error.value='复制失败：'+errorText(e)}
}
function contextKeys(event:KeyboardEvent){
 const inSub=event.target instanceof Element&&!!event.target.closest('.table-copy-submenu')
 if(event.key==='Escape'){event.preventDefault();event.stopPropagation();if(copyAsOpen.value){copyAsOpen.value=false;focusCopyTrigger()}else closeContext();return}
 if(event.key==='ArrowLeft'&&inSub){event.preventDefault();event.stopPropagation();copyAsOpen.value=false;focusCopyTrigger();return}
 if(event.key==='ArrowRight'&&event.target instanceof Element&&event.target.hasAttribute('data-copy-trigger')){event.preventDefault();openCopySubmenu(true);return}
 if(!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return
 event.preventDefault();const items=Array.from(cellMenuRef.value?.querySelectorAll<HTMLButtonElement>(inSub?'[data-cell-menu-copy]:not(:disabled)':'[data-cell-menu-main]:not(:disabled)')??[]),index=items.indexOf(document.activeElement as HTMLButtonElement)
 items[event.key==='Home'?0:event.key==='End'?items.length-1:(index+(event.key==='ArrowDown'?1:items.length-1))%items.length]?.focus()
}
let task='',disposed=false,initialized=false
async function admin(request:AdminRequest):Promise<AdminResult>{task=crypto.randomUUID();try{const result=await api.DatabaseAdmin(task,props.profile.id,props.database,request);if(disposed)throw new Error('标签页已关闭');return result}finally{task=''}}
async function request(sql:string):Promise<AdminResult>{const id=transactionID.value,result=await admin({operation:'query',sql,transactionId:id||undefined});if(id&&!result.transactionActive){transactionID.value='';transactionPending.value=false;uncertain.value=true;if(!result.error)throw new StatementError('事务已结束，请刷新数据')}if(result.error)throw new StatementError(result.error);return result}
async function beginTransaction(){guard(()=>perform(async()=>{const response=await admin({operation:'transaction-begin',name:props.table});if(!response.transactionId||!response.transactionActive)throw new Error(response.error||'事务未启动');transactionID.value=response.transactionId;transactionPending.value=false;transactionUncertain.value=false;await loadPage(page.value);message.value='事务已开始，保存的记录需提交后才生效。空闲 10 分钟自动结束并回滚。'}))}
async function commitTransaction(){if(!transactionID.value||dirty.value)return;await perform(async()=>{try{const response=await admin({operation:'transaction-commit',transactionId:transactionID.value});if(response.error)throw new Error(response.error)}catch(e){transactionUncertain.value=true;uncertain.value=true;throw new Error(errorText(e)+'；提交结果可能不确定，请结束事务并刷新核对。')}transactionID.value='';transactionPending.value=false;transactionUncertain.value=false;try{await loadPage(page.value);message.value='事务已提交'}catch(e){throw new Error('事务已提交，但刷新失败：'+errorText(e))}})}
function rollbackTransaction(){if(!transactionID.value)return;emit('confirm',{title:'回滚并结束事务？',body:'将放弃当前记录草稿，并请求回滚本表事务中尚未提交的操作。已提交的数据无法通过回滚撤销。',label:'回滚事务',action:()=>perform(async()=>{const response=await admin({operation:'transaction-rollback',transactionId:transactionID.value});if(response.error)throw new Error(response.error);transactionID.value='';transactionPending.value=false;transactionUncertain.value=false;isNew.value=false;original.value=undefined;draft.value=[];baseline.value='[]';await loadPage(page.value);message.value=response.completed?'事务已回滚':'事务已结束，请核对数据'})})}
async function perform(action:()=>Promise<void>){if(running.value||!props.connected)return;running.value=true;error.value='';message.value='';try{await action()}catch(e){if(!disposed)error.value=errorText(e)}finally{running.value=false}}
async function stop(){if(task)await api.CancelDatabaseTask(task).catch(e=>{error.value=errorText(e)})}
function pick(index:number){editingColumn.value=null;selectedColumn.value=null;cellBeforeEdit=undefined;selectedIndex.value=index;isNew.value=false;original.value=rows.value[index]?[...rows.value[index]!]:undefined;draft.value=original.value?makeDraft(columns.value,original.value):[];baseline.value=JSON.stringify(draft.value)}
function guard(action:()=>void|Promise<void>){if(running.value)return;if(dirty.value)emit('confirm',{title:'放弃当前记录的修改？',body:'当前记录尚未保存。继续后将丢弃这些修改，数据库中的数据不会因此被删除。',label:'放弃修改',action});else void action()}
function selectFromList(event:Event){const input=event.target as HTMLSelectElement,index=Number(input.value);input.value=String(selectedIndex.value);selectRecord(index)}
function selectRecord(index:number){if(index===selectedIndex.value&&!isNew.value)return;guard(()=>pick(index))}
function cellID(row:CellRow,column:number){return 'data-cell-'+instanceID+'-'+row+'-'+column}
function cellValue(row:CellRow,column:number):CellDraft{if(row==='new'||(!isNew.value&&row===selectedIndex.value))return draft.value[column]??{mode:'null',value:''};const value=rows.value[row]?.[column];return {mode:value===null?'null':'value',value:value??''}}
function cellText(row:CellRow,column:number){const cell=cellValue(row,column);return cell.mode==='null'?'NULL':cell.mode==='default'?columns.value[column]?.generated?'自动生成':'默认值':cell.value}
function currentRow(row:CellRow){return row==='new'?isNew.value:!isNew.value&&selectedIndex.value===row}
function selectCell(row:CellRow,column:number){
 if(running.value)return
 if(currentRow(row)){if(editingColumn.value!==column)finishCellEdit();selectedColumn.value=column;return}
 if(row!=='new')guard(()=>{pick(row);selectedColumn.value=column})
}
function startCellEdit(row:CellRow,column:number){
 const start=()=>{if(row!=='new'&&!currentRow(row))pick(row);const field=columns.value[column];if(!currentRow(row)||!field||!editable(field))return;if(editingColumn.value===column)return;finishCellEdit();selectedColumn.value=column;cellBeforeEdit={...draft.value[column]!};editingColumn.value=column;void nextTick(()=>{if(editingColumn.value!==column||!currentRow(row))return;const input=document.getElementById(cellID(row,column)+'-input') as HTMLTextAreaElement|null;input?.focus();input?.select()})}
 if(currentRow(row))start();else if(row!=='new')guard(start)
}
function finishCellEdit(column=editingColumn.value,cancel=false){if(column===null||editingColumn.value!==column)return;if(cancel&&cellBeforeEdit)draft.value[column]={...cellBeforeEdit};editingColumn.value=null;cellBeforeEdit=undefined}
function cellKeys(event:KeyboardEvent,column:number){if(event.isComposing)return;if(event.key==='Escape'){event.preventDefault();event.stopPropagation();finishCellEdit(column,true)}else if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing){event.preventDefault();finishCellEdit(column)}else return;void nextTick(()=>document.getElementById(cellID(isNew.value?'new':selectedIndex.value,column))?.focus())}
function inputCell(column:number,event:Event){if(editingColumn.value!==column||!draft.value[column])return;draft.value[column]={mode:'value',value:(event.target as HTMLTextAreaElement).value}}
async function loadPage(number:number){
 const response=await request(pageSQL(props.database,props.table,columns.value,keys.value,number,appliedFilter.value,sort.value,descending.value)),result=response.results[0]
 if(!result||result.columns.length!==columns.value.length||result.columns.some((name,i)=>name!==columns.value[i]?.name)||result.rows.some(row=>row.length!==columns.value.length))throw new Error('表结构已变化，请刷新表结构后重试')
 data.value=result;page.value=number;pageInput.value=number;uncertain.value=false;pick(result.rows.length?0:-1)
}
async function initialize(){
 const response=await request(metadataSQL(props.database,props.table));if(response.results.length!==4||response.results.some(r=>r.truncated))throw new Error('表结构信息不完整或超过当前显示上限，请在查询页查看')
 data.value=undefined;original.value=undefined;draft.value=[];baseline.value='[]';selectedIndex.value=-1;isNew.value=false
 columns.value=readColumns(response.results[0]);restoreColumnLayout();indexes.value=readIndexes(response.results[1]);foreignKeys.value=response.results[2];triggers.value=response.results[3];initialized=true
 if(sort.value&&!columns.value.some(c=>c.name===sort.value))sort.value='';filterSort.value=sort.value;filterDescending.value=descending.value
 if(appliedFilter.value.column&&!columns.value.some(c=>c.name===appliedFilter.value.column)){filter.value={column:'',operator:'contains',value:''};appliedFilter.value={...filter.value}}
 await loadPage(page.value);sessionChanged.value=false
}
function refresh(){guard(()=>perform(initialize))}
function navigate(number:number){guard(()=>perform(()=>loadPage(number)))}
function search(){guard(()=>perform(async()=>{const previous=appliedFilter.value,previousSort=sort.value,previousDescending=descending.value;appliedFilter.value={...filter.value};sort.value=filterSort.value;descending.value=filterDescending.value;try{await loadPage(1)}catch(e){appliedFilter.value=previous;sort.value=previousSort;descending.value=previousDescending;throw e}}))}
function resetSearch(){guard(()=>perform(async()=>{const previous=appliedFilter.value,previousSort=sort.value,previousDescending=descending.value;filter.value={column:'',operator:'contains',value:''};appliedFilter.value={...filter.value};filterSort.value='';filterDescending.value=false;sort.value='';descending.value=false;try{await loadPage(1)}catch(e){appliedFilter.value=previous;sort.value=previousSort;descending.value=previousDescending;throw e}}))}
function changeSort(name:string){guard(()=>perform(async()=>{const previous=sort.value,wasDescending=descending.value;descending.value=sort.value===name?!descending.value:false;sort.value=name;filterSort.value=name;filterDescending.value=descending.value;try{await loadPage(1)}catch(e){sort.value=previous;descending.value=wasDescending;throw e}}))}
function selectSection(value:TableSection){section.value=value;emit('section',value)}
function add(){if(!props.connected||!columns.value.length)return;guard(()=>{finishCellEdit();isNew.value=true;original.value=undefined;draft.value=makeDraft(columns.value,undefined);baseline.value=JSON.stringify(draft.value);selectedColumn.value=null;selectSection('data');uncertain.value=false;if(view.value==='grid')void nextTick(()=>document.getElementById('new-row-'+instanceID)?.scrollIntoView?.({block:'nearest'}))})}
function undo(){if(running.value)return;pick(rows.value.length?Math.max(0,selectedIndex.value):-1)}
function editable(column:TableColumn){return canWrite.value&&!column.generated&&!column.spatial}
async function mutate(sql:string,operation:'save'|'delete'){
 try{const response=await request(sql),affected=response.results[0]?.affected;if(affected!=='1')throw new StatementError(operation==='delete'?'记录已被修改或删除，请刷新后重试':'没有记录被修改：值可能与原值等价，或记录已变化，请刷新后重试')}
 catch(e){if(!(e instanceof StatementError)){uncertain.value=true;throw new Error(errorText(e)+'；暂不能确认写入结果，请先刷新核对，避免重复提交')}throw e}
 isNew.value=false;original.value=undefined;draft.value=[];baseline.value='[]';selectedIndex.value=-1;message.value=transactionID.value?(operation==='delete'?'记录已在事务中删除，尚未提交':'记录已保存到事务，尚未提交'):(operation==='delete'?'记录已删除':'记录已保存');if(transactionID.value)transactionPending.value=true
 try{await loadPage(page.value)}catch(e){throw new Error(message.value+'，但刷新失败：'+errorText(e))}
}
async function save(){if(!canWrite.value||!dirty.value)return;await perform(async()=>{await mutate(saveRowSQL(props.database,props.table,columns.value,keys.value,original.value,draft.value),'save')})}
function remove(){if(!canWrite.value||isNew.value||!original.value)return;let sql:string;try{sql=deleteRowSQL(props.database,props.table,columns.value,keys.value,original.value)}catch(e){error.value=errorText(e);return}emit('confirm',{title:'删除当前记录？',body:'将从“'+props.database+'.'+props.table+'”永久删除当前记录。外键和触发器会按数据库规则执行关联操作。',label:'删除记录',action:()=>perform(()=>mutate(sql,'delete'))})}
function recordLabel(row:(string|null)[],index:number){const values=keys.value.map(name=>row[columns.value.findIndex(c=>c.name===name)]).join(' / ');return '第 '+(index+1)+' 条'+(values?' · '+values.slice(0,80):'')}
const fkLabels=['约束','字段','引用库','引用表','引用字段','更新规则','删除规则']
watch(()=>dirty.value||!!transactionID.value,value=>emit('dirty',value),{immediate:true})
watch([view,section,()=>props.active],()=>{finishCellEdit();closeContext();showColumns.value=false})
watch(running,value=>{if(value)finishCellEdit()},{flush:'sync'})
watch(()=>props.section,value=>{section.value=value})
watch(()=>props.sessionKey,(value,old)=>{if(old&&value!==old){sessionChanged.value=true;transactionID.value='';transactionPending.value=false}})
watch(()=>props.connected,connected=>{if(connected&&!initialized)void perform(initialize)})
onMounted(()=>{window.addEventListener('scroll',closeContext,true);window.addEventListener('resize',closeContext);void perform(initialize)})
onUnmounted(()=>{disposed=true;releaseColumnResize?.();window.removeEventListener('scroll',closeContext,true);window.removeEventListener('resize',closeContext);if(task)void api.CancelDatabaseTask(task).catch(()=>{});if(transactionID.value)void api.DatabaseAdmin(crypto.randomUUID(),props.profile.id,props.database,{operation:'transaction-rollback',transactionId:transactionID.value}).catch(()=>{})})
</script>
<template><div class="table-data-editor">
 <nav class="table-section-tabs" aria-label="表内容"><button v-for="item in sections" :key="item[0]" :class="{active:section===item[0]}" @click="selectSection(item[0])">{{item[1]}}</button><span>{{profile.name}} / {{database}} / {{table}}</span></nav>
 <div class="table-data-toolbar" role="toolbar" aria-label="表数据操作"><template v-if="section==='data'">
 <button v-if="!transactionID" class="native-button" @click="beginTransaction" :disabled="running||!connected||!columns.length||profile.tunnelMode==='http'" :title="profile.tunnelMode==='http'?'PHP 中转暂不支持跨请求事务':'在独立会话中开始 InnoDB 事务'"><GitBranch :size="13"/>开始事务</button>
 <template v-else><button class="native-button" @click="commitTransaction" :disabled="running||dirty||transactionUncertain" title="先保存当前记录，再提交事务"><Check :size="13"/>提交事务</button><button class="native-button" @click="rollbackTransaction" :disabled="running"><RotateCcw :size="13"/>{{transactionUncertain?'结束事务':'回滚事务'}}</button><span class="table-transaction-state">{{transactionUncertain?'提交状态未确认':transactionPending?'事务中 · 未提交变更':'事务中'}}</span></template>
 <button class="native-button" @click="toggleCellEditor" :class="{pressed:showCellEditor}" :disabled="!columns.length"><PanelRight :size="13"/>单元格编辑器</button>
 <button class="native-button" @click="showFilters=!showFilters" :class="{pressed:showFilters}"><SlidersHorizontal :size="13"/>筛选 &amp; 排序</button>
 <button class="native-button" @click="showColumns=!showColumns;if(showColumns)showCellEditor=false" :class="{pressed:showColumns}" :disabled="!columns.length"><Columns3 :size="13"/>列</button>
 <button class="native-button" @click="add" :disabled="running||!connected||!columns.length||uncertain||sessionChanged"><Plus :size="13"/>新增</button><button class="native-button default-button" @click="save" :disabled="!canWrite||!dirty"><Save :size="13"/>保存</button><button class="native-button" @click="remove" :disabled="!canWrite||isNew"><Trash2 :size="13"/>删除</button><button class="native-button" @click="undo" :disabled="running||!dirty"><Undo2 :size="13"/>撤销修改</button></template><button class="native-button" @click="refresh" :disabled="running||!connected"><RefreshCw :size="13"/>刷新</button><button v-if="running" class="native-button" @click="stop"><Square :size="12"/>停止</button></div>
 <div v-if="showColumns&&section==='data'" class="table-column-panel"><div class="table-column-actions"><button class="native-button" @click="allColumns">显示全部列</button><button class="native-button" @click="resetColumnWidth()">恢复默认列宽</button></div><div class="table-column-list"><label v-for="column in columns" :key="column.name"><input type="checkbox" :checked="!hiddenColumns.has(column.name)" :disabled="!hiddenColumns.has(column.name)&&visibleColumns.length===1" @change="toggleColumn(column)"/><span :title="column.name+' · '+column.type">{{column.name}}</span><input type="number" min="72" max="1200" :value="columnWidth(column)" :aria-label="column.name+'列宽'" @change="setColumnWidth(column,$event)"/><span>px</span></label></div></div>
 <p v-if="layoutWarning" class="table-data-notice">{{layoutWarning}}</p>
 <p v-if="!connected" class="table-data-notice">连接已关闭。重新打开“{{profile.name}}”后可继续操作，当前内容保留。</p>
 <p v-if="sessionChanged&&connected" class="table-data-notice">连接会话已变化，请先点击“刷新”核对数据后再编辑。</p>
 <div v-if="section==='data'&&showFilters" class="table-data-filter" @keydown.enter.prevent="search"><select v-model="filter.column" aria-label="筛选字段" :disabled="running" @change="filter.operator='contains'"><option value="">全部字段</option><option v-for="c in columns" :key="c.name" :value="c.name">{{c.name}}</option></select><select v-model="filter.operator" aria-label="查询条件" :disabled="running||!filter.column"><option value="contains">包含</option><option value="equal">等于</option><option value="notEqual">不等于</option><option value="greater">大于</option><option value="less">小于</option><option value="null">为 NULL</option><option value="notNull">非 NULL</option></select><input v-model="filter.value" aria-label="查询内容" placeholder="输入查询条件" :disabled="running||['null','notNull'].includes(filter.operator)"/><select v-model="filterSort" aria-label="排序字段" :disabled="running"><option value="">默认排序</option><option v-for="column in columns" :key="column.name" :value="column.name">{{column.name}}</option></select><select v-model="filterDescending" aria-label="排序方向" :disabled="running"><option :value="false">升序</option><option :value="true">降序</option></select><button class="native-button" @click="search" :disabled="running||!connected||!columns.length"><Search :size="13"/>查询</button><button class="native-button" @click="resetSearch" :disabled="running||!connected">清空</button></div>
 <div class="table-data-feedback"><span v-if="running" role="status"><LoaderCircle :size="13" class="spin"/>正在处理…</span><p v-if="error" class="db-tools-error table-dismissible-feedback" role="alert"><span>{{error}}</span><button class="tiny-button" aria-label="关闭错误提示" title="关闭提示" @click="error=''"><X :size="14"/></button></p><p v-if="message" class="db-tools-message table-dismissible-feedback" role="status"><span>{{message}}</span><button class="tiny-button" aria-label="关闭操作提示" title="关闭提示" @click="message=''"><X :size="14"/></button></p><p v-if="section==='data'&&columns.length&&!keys.length" class="table-data-notice">此表没有可用的非空唯一键，已有记录仅供查看，可新增记录。</p><p v-if="dirty" class="table-data-dirty">{{isNew?'新增记录尚未保存':'当前记录有未保存的修改'}}</p></div>
 <div class="table-data-body"><div class="table-data-content">
  <template v-if="section==='data'">
   <template v-if="view==='form'">
    <div class="table-record-navigation"><button class="tiny-button" aria-label="上一条记录" :disabled="running||isNew||selectedIndex<=0" @click="selectRecord(selectedIndex-1)"><ChevronLeft :size="15"/></button><select :value="isNew?-1:selectedIndex" aria-label="选择当前记录" :disabled="running||isNew||!rows.length" @change="selectFromList"><option v-if="isNew||!rows.length" :value="-1">{{isNew?'新增记录':'暂无记录'}}</option><option v-for="(row,index) in rows" :key="index" :value="index">{{recordLabel(row,index)}}</option></select><button class="tiny-button" aria-label="下一条记录" :disabled="running||isNew||selectedIndex>=rows.length-1" @click="selectRecord(selectedIndex+1)"><ChevronRight :size="15"/></button><span>{{isNew?'新增':rows.length?selectedIndex+1:0}} / {{rows.length}}</span></div>
    <div v-if="hasRecord" class="table-record-form">
     <div v-for="(column,index) in columns" :key="column.name" class="table-record-field">
      <label :id="cellID(isNew?'new':selectedIndex,index)+'-label'"><strong>{{column.key==='PRI'?'◆ ':''}}{{column.name}}</strong><small>{{column.type}}{{column.binary?' · HEX':''}}{{column.generated?' · 自动生成':''}}{{column.spatial?' · 只读':''}}</small></label>
      <div :id="cellID(isNew?'new':selectedIndex,index)" class="table-record-value" :class="{'cell-selected':selectedColumn===index}" tabindex="0" :aria-labelledby="cellID(isNew?'new':selectedIndex,index)+'-label'" @click="selectCell(isNew?'new':selectedIndex,index)" @dblclick.stop="startCellEdit(isNew?'new':selectedIndex,index)" @contextmenu="contextCell($event,isNew?'new':selectedIndex,index)">
       <div v-if="editingColumn===index" class="table-cell-editor" @click.stop @dblclick.stop>
        <textarea :id="cellID(isNew?'new':selectedIndex,index)+'-input'" :value="draft[index]!.mode==='value'?draft[index]!.value:''" @input="inputCell(index,$event)" :aria-label="column.name" :readonly="!editable(column)" :placeholder="draft[index]!.mode==='null'?'NULL':draft[index]!.mode==='default'?column.defaultValue??'使用数据库默认值':''" rows="2" spellcheck="false" @blur="finishCellEdit(index)" @keydown="cellKeys($event,index)"/>
       </div>
       <div v-else class="table-cell-display" :class="{null:draft[index]!.mode==='null',default:draft[index]!.mode==='default'}">{{cellText(isNew?'new':selectedIndex,index)}}</div>
       <small v-if="column.comment">{{column.comment}}</small>
      </div>
     </div>
    </div><p v-else-if="!running" class="table-data-empty">没有记录，可点击“新增”添加数据。</p>
   </template>
   <div v-else class="table-data-grid" tabindex="0" aria-label="数据网格，可横向滚动" title="可用触控板或 Shift + 滚轮横向滚动"><table role="grid" :aria-label="table+'数据'" :style="{width:gridWidth+'px',minWidth:gridWidth+'px'}"><colgroup><col style="width:44px"/><col v-for="item in visibleColumns" :key="item.column.name" :style="{width:columnWidth(item.column)+'px'}"/></colgroup><thead><tr><th class="row-number">#</th><th v-for="item in visibleColumns" :key="item.column.name" :title="item.column.name+' · '+item.column.type"><button @click="changeSort(item.column.name)" :disabled="running">{{item.column.name}}<span v-if="sort===item.column.name">{{descending?' ▼':' ▲'}}</span><small>{{item.column.type}}{{item.column.binary?' · HEX':''}}</small></button><span class="table-column-resizer" role="separator" aria-orientation="vertical" :aria-label="'调整'+item.column.name+'列宽'" :aria-valuenow="columnWidth(item.column)" aria-valuemin="72" aria-valuemax="1200" tabindex="0" @pointerdown="resizeColumn($event,item.column)" @click.stop @dblclick.stop="resetColumnWidth(item.column)" @keydown="resizeColumnKeys($event,item.column)"></span></th></tr></thead><tbody>
    <tr v-for="(row,index) in rows" :key="index" :class="{selected:!isNew&&index===selectedIndex}">
     <td class="row-number" @click="selectRecord(index)">{{(page-1)*1000+index+1}}</td>
     <td v-for="{column,index:i} in visibleColumns" :id="cellID(index,i)" :key="i" :class="{'cell-selected':!isNew&&index===selectedIndex&&selectedColumn===i}" :tabindex="!isNew&&index===selectedIndex&&selectedColumn===i?0:-1" @click="selectCell(index,i)" @dblclick.stop="startCellEdit(index,i)" @contextmenu="contextCell($event,index,i)">
      <div v-if="hasRecord&&!isNew&&index===selectedIndex&&editingColumn===i&&columns[i]" class="grid-cell-editor" @click.stop @dblclick.stop>
       <textarea :id="cellID(index,i)+'-input'" :value="draft[i]!.mode==='value'?draft[i]!.value:''" @input="inputCell(i,$event)" :aria-label="columns[i]!.name" :readonly="!editable(columns[i]!)" :placeholder="draft[i]!.mode==='null'?'NULL':''" rows="1" spellcheck="false" @blur="finishCellEdit(i)" @keydown="cellKeys($event,i)"/>
      </div>
      <span v-else :class="{null:cellValue(index,i).mode==='null'}">{{gridCellText(index,i)}}</span>
     </td>
    </tr>
    <tr v-if="isNew&&hasRecord" :id="'new-row-'+instanceID" class="selected"><td class="row-number">新增</td><td v-for="{column,index:i} in visibleColumns" :id="cellID('new',i)" :key="column.name" :class="{'cell-selected':selectedColumn===i}" :tabindex="selectedColumn===i?0:-1" @click="selectCell('new',i)" @dblclick.stop="startCellEdit('new',i)" @contextmenu="contextCell($event,'new',i)">
     <div v-if="editingColumn===i" class="grid-cell-editor" @click.stop @dblclick.stop>
      <textarea :id="cellID('new',i)+'-input'" :value="draft[i]!.mode==='value'?draft[i]!.value:''" @input="inputCell(i,$event)" :aria-label="column.name" :readonly="!editable(column)" :placeholder="draft[i]!.mode==='default'?column.defaultValue??'使用数据库默认值':draft[i]!.mode==='null'?'NULL':''" rows="1" spellcheck="false" @blur="finishCellEdit(i)" @keydown="cellKeys($event,i)"/>
     </div><span v-else :class="{null:draft[i]!.mode==='null',default:draft[i]!.mode==='default'}">{{gridCellText('new',i)}}</span>
    </td></tr>
   </tbody></table><p v-if="!rows.length&&!isNew&&!running" class="table-data-empty">没有记录</p></div>
  </template>
  <div v-else class="table-metadata-content"><p class="table-data-note">仅展示当前账户可见的结构信息。</p>
   <table v-if="section==='fields'"><thead><tr><th>字段</th><th>类型</th><th>允许 NULL</th><th>默认值</th><th>键</th><th>附加属性</th><th>注释</th></tr></thead><tbody><tr v-for="c in columns" :key="c.name"><td>{{c.name}}</td><td>{{c.type}}</td><td>{{c.nullable?'是':'否'}}</td><td>{{c.defaultValue===null?'NULL / 无默认值':c.defaultValue}}</td><td>{{c.key}}</td><td>{{c.extra}}</td><td>{{c.comment}}</td></tr></tbody></table>
   <table v-else-if="section==='indexes'"><thead><tr><th>索引</th><th>类型</th><th>唯一</th><th>字段</th></tr></thead><tbody><tr v-for="index in indexes" :key="index.name"><td>{{index.name}}</td><td>{{index.type}}</td><td>{{index.unique?'是':'否'}}</td><td>{{index.parts.join(', ')}}</td></tr></tbody></table>
   <table v-else-if="section==='foreignKeys'"><thead><tr><th v-for="label in fkLabels" :key="label">{{label}}</th></tr></thead><tbody><tr v-for="(row,i) in foreignKeys?.rows??[]" :key="i"><td v-for="(value,n) in row" :key="n">{{value??'NULL'}}</td></tr></tbody></table>
   <div v-else-if="section==='triggers'" class="table-trigger-list"><section v-for="(row,i) in triggers?.rows??[]" :key="i"><h3>{{row[0]}} <small>{{row[1]}} {{row[2]}}</small></h3><pre>{{row[3]}}</pre></section></div>
   <p v-if="!running&&((section==='indexes'&&!indexes.length)||(section==='foreignKeys'&&!foreignKeys?.rows.length)||(section==='triggers'&&!triggers?.rows.length))" class="table-data-empty">没有可见的{{sections.find(item=>item[0]===section)?.[1]}}</p>
  </div>
 </div>
 <aside v-if="showCellEditor&&section==='data'" class="table-cell-sidebar" aria-label="单元格编辑器"><header><strong>单元格编辑器</strong><button class="tiny-button" @click="showCellEditor=false" aria-label="关闭单元格编辑器">×</button></header><select :value="selectedColumn??0" aria-label="编辑器字段" :disabled="running||!columns.length" @change="sideColumn"><option v-for="(column,i) in columns" :key="column.name" :value="i">{{column.name}}</option></select><p v-if="selectedField" class="table-data-note">{{selectedField.type}}{{selectedField.binary?' · HEX':''}}{{selectedField.generated?' · 只读生成列':''}}{{selectedField.spatial?' · 只读空间列':''}}</p><textarea v-if="selectedColumn!==null" :value="draft[selectedColumn]?.mode==='value'?draft[selectedColumn]!.value:''" :aria-label="(selectedField?.name??'')+'完整内容'" :readonly="!selectedField||!editable(selectedField)" :placeholder="draft[selectedColumn]?.mode==='null'?'NULL':draft[selectedColumn]?.mode==='default'?'使用数据库默认值':'请选择记录'" spellcheck="false" @input="sideInput" @contextmenu="hasRecord&&contextCell($event,isNew?'new':selectedIndex,selectedColumn)"/><p class="table-data-note">支持查看和编辑完整内容。修改仍需点击“保存”；右键可设置空字符串、NULL 或 UUID。</p></aside>
 </div>
 <div v-if="cellMenu" ref="cellMenuRef" class="table-cell-context" role="menu" :style="{left:cellMenu.x+'px',top:cellMenu.y+'px'}" @pointerdown.stop @click.stop @keydown="contextKeys">
  <button data-cell-menu-main role="menuitem" @mouseenter="copyAsOpen=false" @click="copyCell()">复制</button>
  <div class="table-copy-group" @mouseenter="openCopySubmenu()" @mouseleave="copyAsOpen=false">
   <button data-cell-menu-main data-copy-trigger role="menuitem" aria-haspopup="menu" :aria-expanded="copyAsOpen" @click="openCopySubmenu(true)">复制为<ChevronRight :size="12"/></button>
   <div v-if="copyAsOpen" class="table-copy-submenu" role="menu" aria-label="复制为" :style="copySubStyle"><button v-for="item in copyFormats" :key="item[0]" data-cell-menu-copy role="menuitem" :disabled="item[0]==='update'&&(cellMenu.row==='new'||!keys.length)" :title="item[0]==='update'&&!keys.length?'需要非空唯一键来定位记录':undefined" @click="copyCell(item[0])">{{item[1]}}</button></div>
  </div>
  <button data-cell-menu-main role="menuitem" :disabled="!menuWritable" @mouseenter="copyAsOpen=false" @click="contextAction('empty')">设置为空白字符串</button><button data-cell-menu-main role="menuitem" :disabled="!menuWritable||!columns[cellMenu.column]?.nullable" @mouseenter="copyAsOpen=false" @click="contextAction('null')">设置为 NULL</button><button data-cell-menu-main role="menuitem" :disabled="!menuUUID" @mouseenter="copyAsOpen=false" @click="contextAction('uuid')">生成 UUID</button><button data-cell-menu-main role="menuitem" :disabled="running||!connected" @mouseenter="copyAsOpen=false" @click="contextAction('refresh')">刷新</button>
 </div>
 <footer v-if="section==='data'" class="table-data-pagination"><button class="tiny-button" aria-label="第一页" :disabled="running||page===1||!connected" @click="navigate(1)"><ChevronsLeft :size="15"/></button><button class="tiny-button" aria-label="上一页" :disabled="running||page===1||!connected" @click="navigate(page-1)"><ChevronLeft :size="15"/></button><span>第</span><input v-model.number="pageInput" type="number" min="1" max="1000000" aria-label="页码" :disabled="running" @keydown.enter.prevent="navigate(Number(pageInput))"/><span>页</span><button class="native-button" @click="navigate(Number(pageInput))" :disabled="running||!connected">跳转</button><button class="tiny-button" aria-label="下一页" :disabled="running||!hasNext||!connected" @click="navigate(page+1)"><ChevronRight :size="15"/></button><span class="table-page-count">每页 1,000 条 · 本页 {{rows.length}} 条</span><div class="table-view-switch"><button :class="{active:view==='form'}" @click="view='form'" :aria-pressed="view==='form'" aria-label="表单视图" title="表单视图"><List :size="14"/></button><button :class="{active:view==='grid'}" @click="view='grid'" :aria-pressed="view==='grid'" aria-label="网格视图" title="网格视图"><LayoutGrid :size="14"/></button></div></footer>
</div></template>
