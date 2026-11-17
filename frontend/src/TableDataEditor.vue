<script setup lang="ts">
import {computed,nextTick,onMounted,onUnmounted,ref,watch} from 'vue'
import {Plus,Save,Trash2,RefreshCw,Search,ChevronLeft,ChevronRight,ChevronsLeft,LayoutGrid,List,Undo2,LoaderCircle,Square} from '@lucide/vue'
import {api,errorText,type AdminResult,type Profile,type QueryResult} from './api'
import {deleteRowSQL,makeDraft,metadataSQL,pageSQL,readColumns,readIndexes,saveRowSQL,uniqueKey,type CellDraft,type TableColumn,type TableFilter,type TableIndex,type TableSection} from './tableEditing'
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
let task='',disposed=false,initialized=false
async function request(sql:string):Promise<AdminResult>{task=crypto.randomUUID();try{const result=await api.DatabaseAdmin(task,props.profile.id,props.database,{operation:'query',sql});if(disposed)throw new Error('标签页已关闭');if(result.error)throw new StatementError(result.error);return result}finally{task=''}}
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
function setCellMode(column:number,mode:'value'|'null'|'default'){const field=columns.value[column];if(editingColumn.value!==column||!field||!editable(field)||(mode==='null'&&!field.nullable)||(mode==='default'&&!isNew.value))return;draft.value[column]={mode,value:''};document.getElementById(cellID(isNew.value?'new':selectedIndex.value,column)+'-input')?.focus()}
async function loadPage(number:number){
 const response=await request(pageSQL(props.database,props.table,columns.value,keys.value,number,appliedFilter.value,sort.value,descending.value)),result=response.results[0]
 if(!result||result.columns.length!==columns.value.length||result.columns.some((name,i)=>name!==columns.value[i]?.name)||result.rows.some(row=>row.length!==columns.value.length))throw new Error('表结构已变化，请刷新表结构后重试')
 data.value=result;page.value=number;pageInput.value=number;uncertain.value=false;pick(result.rows.length?0:-1)
}
async function initialize(){
 const response=await request(metadataSQL(props.database,props.table));if(response.results.length!==4||response.results.some(r=>r.truncated))throw new Error('表结构信息不完整或超过当前显示上限，请在查询页查看')
 data.value=undefined;original.value=undefined;draft.value=[];baseline.value='[]';selectedIndex.value=-1;isNew.value=false
 columns.value=readColumns(response.results[0]);indexes.value=readIndexes(response.results[1]);foreignKeys.value=response.results[2];triggers.value=response.results[3];initialized=true
 if(sort.value&&!columns.value.some(c=>c.name===sort.value))sort.value=''
 if(appliedFilter.value.column&&!columns.value.some(c=>c.name===appliedFilter.value.column)){filter.value={column:'',operator:'contains',value:''};appliedFilter.value={...filter.value}}
 await loadPage(page.value);sessionChanged.value=false
}
function refresh(){guard(()=>perform(initialize))}
function navigate(number:number){guard(()=>perform(()=>loadPage(number)))}
function search(){guard(()=>perform(async()=>{const previous=appliedFilter.value;appliedFilter.value={...filter.value};try{await loadPage(1)}catch(e){appliedFilter.value=previous;throw e}}))}
function resetSearch(){guard(()=>perform(async()=>{const previous=appliedFilter.value;filter.value={column:'',operator:'contains',value:''};appliedFilter.value={...filter.value};try{await loadPage(1)}catch(e){appliedFilter.value=previous;throw e}}))}
function changeSort(name:string){guard(()=>perform(async()=>{const previous=sort.value,wasDescending=descending.value;descending.value=sort.value===name?!descending.value:false;sort.value=name;try{await loadPage(1)}catch(e){sort.value=previous;descending.value=wasDescending;throw e}}))}
function selectSection(value:TableSection){section.value=value;emit('section',value)}
function add(){if(!props.connected||!columns.value.length)return;guard(()=>{finishCellEdit();isNew.value=true;original.value=undefined;draft.value=makeDraft(columns.value,undefined);baseline.value=JSON.stringify(draft.value);selectedColumn.value=null;selectSection('data');uncertain.value=false;if(view.value==='grid')void nextTick(()=>document.getElementById('new-row-'+instanceID)?.scrollIntoView?.({block:'nearest'}))})}
function undo(){if(running.value)return;pick(rows.value.length?Math.max(0,selectedIndex.value):-1)}
function editable(column:TableColumn){return canWrite.value&&!column.generated&&!column.spatial}
async function mutate(sql:string,operation:'save'|'delete'){
 try{const response=await request(sql),affected=response.results[0]?.affected;if(affected!=='1')throw new StatementError(operation==='delete'?'记录已被修改或删除，请刷新后重试':'没有记录被修改：值可能与原值等价，或记录已变化，请刷新后重试')}
 catch(e){if(!(e instanceof StatementError)){uncertain.value=true;throw new Error(errorText(e)+'；暂不能确认写入结果，请先刷新核对，避免重复提交')}throw e}
 isNew.value=false;original.value=undefined;draft.value=[];baseline.value='[]';selectedIndex.value=-1;message.value=operation==='delete'?'记录已删除':'记录已保存'
 try{await loadPage(page.value)}catch(e){throw new Error(message.value+'，但刷新失败：'+errorText(e))}
}
async function save(){if(!canWrite.value||!dirty.value)return;await perform(async()=>{await mutate(saveRowSQL(props.database,props.table,columns.value,keys.value,original.value,draft.value),'save')})}
function remove(){if(!canWrite.value||isNew.value||!original.value)return;let sql:string;try{sql=deleteRowSQL(props.database,props.table,columns.value,keys.value,original.value)}catch(e){error.value=errorText(e);return}emit('confirm',{title:'删除当前记录？',body:'将从“'+props.database+'.'+props.table+'”永久删除当前记录。外键和触发器会按数据库规则执行关联操作。',label:'删除记录',action:()=>perform(()=>mutate(sql,'delete'))})}
function recordLabel(row:(string|null)[],index:number){const values=keys.value.map(name=>row[columns.value.findIndex(c=>c.name===name)]).join(' / ');return '第 '+(index+1)+' 条'+(values?' · '+values.slice(0,80):'')}
const fkLabels=['约束','字段','引用库','引用表','引用字段','更新规则','删除规则']
watch(dirty,value=>emit('dirty',value),{immediate:true})
watch([view,section,()=>props.active],()=>finishCellEdit())
watch(running,value=>{if(value)finishCellEdit()},{flush:'sync'})
watch(()=>props.section,value=>{section.value=value})
watch(()=>props.sessionKey,(value,old)=>{if(old&&value!==old)sessionChanged.value=true})
watch(()=>props.connected,connected=>{if(connected&&!initialized)void perform(initialize)})
onMounted(()=>void perform(initialize))
onUnmounted(()=>{disposed=true;if(task)void api.CancelDatabaseTask(task).catch(()=>{})})
</script>
<template><div class="table-data-editor">
 <nav class="table-section-tabs" aria-label="表内容"><button v-for="item in sections" :key="item[0]" :class="{active:section===item[0]}" @click="selectSection(item[0])">{{item[1]}}</button><span>{{profile.name}} / {{database}} / {{table}}</span></nav>
 <div class="table-data-toolbar" role="toolbar" aria-label="表数据操作"><template v-if="section==='data'"><button class="native-button" @click="add" :disabled="running||!connected||!columns.length||uncertain||sessionChanged"><Plus :size="13"/>新增</button><button class="native-button default-button" @click="save" :disabled="!canWrite||!dirty"><Save :size="13"/>保存</button><button class="native-button" @click="remove" :disabled="!canWrite||isNew"><Trash2 :size="13"/>删除</button><button class="native-button" @click="undo" :disabled="running||!dirty"><Undo2 :size="13"/>撤销修改</button></template><button class="native-button" @click="refresh" :disabled="running||!connected"><RefreshCw :size="13"/>刷新</button><button v-if="running" class="native-button" @click="stop"><Square :size="12"/>停止</button><div v-if="section==='data'" class="table-view-switch"><button :class="{active:view==='form'}" @click="view='form'" :aria-pressed="view==='form'"><List :size="13"/>表单视图</button><button :class="{active:view==='grid'}" @click="view='grid'" :aria-pressed="view==='grid'"><LayoutGrid :size="13"/>网格视图</button></div></div>
 <p v-if="!connected" class="table-data-notice">连接已关闭。重新打开“{{profile.name}}”后可继续操作，当前内容保留。</p>
 <p v-if="sessionChanged&&connected" class="table-data-notice">连接会话已变化，请先点击“刷新”核对数据后再编辑。</p>
 <div v-if="section==='data'" class="table-data-filter" @keydown.enter.prevent="search"><select v-model="filter.column" aria-label="筛选字段" :disabled="running" @change="filter.operator='contains'"><option value="">全部字段</option><option v-for="c in columns" :key="c.name" :value="c.name">{{c.name}}</option></select><select v-model="filter.operator" aria-label="查询条件" :disabled="running||!filter.column"><option value="contains">包含</option><option value="equal">等于</option><option value="notEqual">不等于</option><option value="greater">大于</option><option value="less">小于</option><option value="null">为 NULL</option><option value="notNull">非 NULL</option></select><input v-model="filter.value" aria-label="查询内容" placeholder="输入查询条件" :disabled="running||['null','notNull'].includes(filter.operator)"/><button class="native-button" @click="search" :disabled="running||!connected||!columns.length"><Search :size="13"/>查询</button><button class="native-button" @click="resetSearch" :disabled="running||!connected">清空</button></div>
 <div class="table-data-feedback"><span v-if="running" role="status"><LoaderCircle :size="13" class="spin"/>正在处理…</span><p v-if="error" class="db-tools-error" role="alert">{{error}}</p><p v-if="message" class="db-tools-message" role="status">{{message}}</p><p v-if="section==='data'&&columns.length&&!keys.length" class="table-data-notice">此表没有可用的非空唯一键，已有记录仅供查看，可新增记录。</p><p v-if="dirty" class="table-data-dirty">{{isNew?'新增记录尚未保存':'当前记录有未保存的修改'}}</p></div>
 <div class="table-data-content">
  <template v-if="section==='data'">
   <template v-if="view==='form'">
    <div class="table-record-navigation"><button class="tiny-button" aria-label="上一条记录" :disabled="running||isNew||selectedIndex<=0" @click="selectRecord(selectedIndex-1)"><ChevronLeft :size="15"/></button><select :value="isNew?-1:selectedIndex" aria-label="选择当前记录" :disabled="running||isNew||!rows.length" @change="selectFromList"><option v-if="isNew||!rows.length" :value="-1">{{isNew?'新增记录':'暂无记录'}}</option><option v-for="(row,index) in rows" :key="index" :value="index">{{recordLabel(row,index)}}</option></select><button class="tiny-button" aria-label="下一条记录" :disabled="running||isNew||selectedIndex>=rows.length-1" @click="selectRecord(selectedIndex+1)"><ChevronRight :size="15"/></button><span>{{isNew?'新增':rows.length?selectedIndex+1:0}} / {{rows.length}}</span></div>
    <div v-if="hasRecord" class="table-record-form">
     <div v-for="(column,index) in columns" :key="column.name" class="table-record-field">
      <label :id="cellID(isNew?'new':selectedIndex,index)+'-label'"><strong>{{column.key==='PRI'?'◆ ':''}}{{column.name}}</strong><small>{{column.type}}{{column.binary?' · HEX':''}}{{column.generated?' · 自动生成':''}}{{column.spatial?' · 只读':''}}</small></label>
      <div :id="cellID(isNew?'new':selectedIndex,index)" class="table-record-value" :class="{'cell-selected':selectedColumn===index}" tabindex="0" :aria-labelledby="cellID(isNew?'new':selectedIndex,index)+'-label'" @click="selectCell(isNew?'new':selectedIndex,index)" @dblclick.stop="startCellEdit(isNew?'new':selectedIndex,index)">
       <div v-if="editingColumn===index" class="table-cell-editor" @click.stop @dblclick.stop>
        <textarea :id="cellID(isNew?'new':selectedIndex,index)+'-input'" :value="draft[index]!.mode==='value'?draft[index]!.value:''" @input="inputCell(index,$event)" :aria-label="column.name" :readonly="!editable(column)" :placeholder="draft[index]!.mode==='null'?'NULL':draft[index]!.mode==='default'?column.defaultValue??'使用数据库默认值':''" rows="2" spellcheck="false" @blur="finishCellEdit(index)" @keydown="cellKeys($event,index)"/>
        <div class="cell-edit-actions"><button v-if="column.nullable" type="button" title="设置为 NULL" @mousedown.prevent @click="setCellMode(index,'null')">NULL</button><button type="button" title="设置为空字符串" @mousedown.prevent @click="setCellMode(index,'value')">空字符串</button><button v-if="isNew" type="button" title="使用数据库默认值" @mousedown.prevent @click="setCellMode(index,'default')">默认值</button></div>
       </div>
       <div v-else class="table-cell-display" :class="{null:draft[index]!.mode==='null',default:draft[index]!.mode==='default'}">{{cellText(isNew?'new':selectedIndex,index)}}</div>
       <small v-if="column.comment">{{column.comment}}</small>
      </div>
     </div>
    </div><p v-else-if="!running" class="table-data-empty">没有记录，可点击“新增”添加数据。</p>
   </template>
   <div v-else class="table-data-grid"><table role="grid" :aria-label="table+'数据'"><thead><tr><th class="row-number">#</th><th v-for="column in columns" :key="column.name"><button @click="changeSort(column.name)" :disabled="running">{{column.name}}<span v-if="sort===column.name">{{descending?' ▼':' ▲'}}</span><small>{{column.type}}{{column.binary?' · HEX':''}}</small></button></th></tr></thead><tbody>
    <tr v-for="(row,index) in rows" :key="index" :class="{selected:!isNew&&index===selectedIndex}">
     <td class="row-number" @click="selectRecord(index)">{{(page-1)*1000+index+1}}</td>
     <td v-for="(_value,i) in row" :id="cellID(index,i)" :key="i" :class="{'cell-selected':!isNew&&index===selectedIndex&&selectedColumn===i}" :tabindex="!isNew&&index===selectedIndex&&selectedColumn===i?0:-1" @click="selectCell(index,i)" @dblclick.stop="startCellEdit(index,i)">
      <div v-if="hasRecord&&!isNew&&index===selectedIndex&&editingColumn===i&&columns[i]" class="grid-cell-editor" @click.stop @dblclick.stop>
       <textarea :id="cellID(index,i)+'-input'" :value="draft[i]!.mode==='value'?draft[i]!.value:''" @input="inputCell(i,$event)" :aria-label="columns[i]!.name" :readonly="!editable(columns[i]!)" :placeholder="draft[i]!.mode==='null'?'NULL':''" rows="1" spellcheck="false" @blur="finishCellEdit(i)" @keydown="cellKeys($event,i)"/>
       <div class="cell-edit-actions"><button v-if="columns[i]!.nullable" type="button" title="设置为 NULL" @mousedown.prevent @click="setCellMode(i,'null')">NULL</button><button type="button" title="设置为空字符串" @mousedown.prevent @click="setCellMode(i,'value')">空字符串</button></div>
      </div>
      <span v-else :class="{null:cellValue(index,i).mode==='null'}">{{cellText(index,i)}}</span>
     </td>
    </tr>
    <tr v-if="isNew&&hasRecord" :id="'new-row-'+instanceID" class="selected"><td class="row-number">新增</td><td v-for="(column,i) in columns" :id="cellID('new',i)" :key="column.name" :class="{'cell-selected':selectedColumn===i}" :tabindex="selectedColumn===i?0:-1" @click="selectCell('new',i)" @dblclick.stop="startCellEdit('new',i)">
     <div v-if="editingColumn===i" class="grid-cell-editor" @click.stop @dblclick.stop>
      <textarea :id="cellID('new',i)+'-input'" :value="draft[i]!.mode==='value'?draft[i]!.value:''" @input="inputCell(i,$event)" :aria-label="column.name" :readonly="!editable(column)" :placeholder="draft[i]!.mode==='default'?column.defaultValue??'使用数据库默认值':draft[i]!.mode==='null'?'NULL':''" rows="1" spellcheck="false" @blur="finishCellEdit(i)" @keydown="cellKeys($event,i)"/>
      <div class="cell-edit-actions"><button v-if="column.nullable" type="button" title="设置为 NULL" @mousedown.prevent @click="setCellMode(i,'null')">NULL</button><button type="button" title="设置为空字符串" @mousedown.prevent @click="setCellMode(i,'value')">空字符串</button><button type="button" title="使用数据库默认值" @mousedown.prevent @click="setCellMode(i,'default')">默认值</button></div>
     </div><span v-else :class="{null:draft[i]!.mode==='null',default:draft[i]!.mode==='default'}">{{cellText('new',i)}}</span>
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
 <footer v-if="section==='data'" class="table-data-pagination"><button class="tiny-button" aria-label="第一页" :disabled="running||page===1||!connected" @click="navigate(1)"><ChevronsLeft :size="15"/></button><button class="tiny-button" aria-label="上一页" :disabled="running||page===1||!connected" @click="navigate(page-1)"><ChevronLeft :size="15"/></button><span>第</span><input v-model.number="pageInput" type="number" min="1" max="1000000" aria-label="页码" :disabled="running" @keydown.enter.prevent="navigate(Number(pageInput))"/><span>页</span><button class="native-button" @click="navigate(Number(pageInput))" :disabled="running||!connected">跳转</button><button class="tiny-button" aria-label="下一页" :disabled="running||!hasNext||!connected" @click="navigate(page+1)"><ChevronRight :size="15"/></button><span class="table-page-count">每页 1,000 条 · 本页 {{rows.length}} 条</span></footer>
 <p v-if="section==='data'" class="table-data-note table-edit-note">单击选中，双击单元格编辑；Enter 结束编辑，Shift+Enter 换行，Esc 取消当前单元格修改。修改后点击“保存”才写入数据库。停止或报错后请刷新核对；已提交的操作不会自动回滚。二进制值使用 HEX，空间列只读。</p>
</div></template>
