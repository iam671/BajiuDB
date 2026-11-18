<script setup lang="ts">
import {computed,nextTick,onMounted,onUnmounted,ref,shallowRef,watch} from 'vue'
import {Play,Square,Plug,Database,LoaderCircle} from '@lucide/vue'
import {api,errorText,type AdminResult,type Profile} from './api'
import TableDataEditor from './TableDataEditor.vue'
import {editableQuery,type EditableQuery} from './queryEditing'
import type {CellValueRequest} from './cellValueTools'
const props=defineProps<{profile:Profile;database:string;initialSQL:string;active:boolean;connected:boolean;sessionKey?:string}>()
type Confirmation={title:string;body:string;label:string;action:()=>void|Promise<void>}
const emit=defineEmits<{dirty:[value:boolean];confirm:[request:Confirmation];'quick-edit':[request:CellValueRequest]}>()
const sql=ref(props.initialSQL),editor=ref<HTMLTextAreaElement>(),executing=ref(false),starting=ref(false),childBusy=ref(false),childDirty=ref(false),stopping=ref(false),error=ref(''),message=ref(''),result=ref<AdminResult|null>(null)
const query=shallowRef<EditableQuery>(),queryID=ref(''),tableEditor=ref<InstanceType<typeof TableDataEditor>>()
const running=computed(()=>executing.value||starting.value||childBusy.value)
watch([sql,running,childDirty],()=>emit('dirty',sql.value!==props.initialSQL||running.value||childDirty.value),{immediate:true,flush:'sync'})
let task='',disposed=false,executedSQL=''
async function focusEditor(){await nextTick();if(props.active&&!disposed)editor.value?.focus()}
async function rawQuery(text:string){
 task=crypto.randomUUID();executing.value=true;stopping.value=false
 try{const response=await api.DatabaseAdmin(task,props.profile.id,props.database,{operation:'query',sql:text});if(disposed)return;result.value=response;if(response.error)error.value=response.error;else message.value+='已完成 '+response.completed+' 条语句'+(response.completed>100?'；仅显示前 100 个结果':'')}
 catch(e){if(!disposed)error.value=errorText(e)}finally{task='';executing.value=false;stopping.value=false}
}
async function run(text:string){
 if(disposed||!props.connected)return
 starting.value=true;query.value=undefined;childDirty.value=false;childBusy.value=false;error.value='';message.value='';result.value=null;executedSQL=text
 await nextTick();if(disposed)return
 const plan=editableQuery(text,props.database)
 if(plan){queryID.value=crypto.randomUUID();starting.value=true;query.value=plan;await nextTick();starting.value=false}
 else{starting.value=false;message.value='此查询结果仅供查看。可编辑结果需为包含直接字段的单表 SELECT。';await rawQuery(text)}
}
function execute(){
 if(running.value||!props.connected)return
 let text=sql.value;const input=editor.value;if(input&&input.selectionStart!==input.selectionEnd)text=text.slice(input.selectionStart,input.selectionEnd)
 if(!text.trim())return
 if(childDirty.value)emit('confirm',{title:'重新执行并放弃结果修改？',body:'将放弃未保存的数据草稿，并请求回滚查询结果中尚未提交的事务。',label:'重新执行',action:()=>run(text)})
 else void run(text)
}
async function unavailable(reason:string){query.value=undefined;childBusy.value=false;childDirty.value=false;message.value='只读结果：'+reason+'。';await rawQuery(executedSQL)}
async function cancel(){if(stopping.value)return;stopping.value=true;try{if(query.value)await tableEditor.value?.cancel();else if(task)await api.CancelDatabaseTask(task)}catch(e){if(!disposed)error.value=errorText(e)}finally{stopping.value=false}}
function keys(event:KeyboardEvent){if((event.ctrlKey||event.metaKey)&&event.key==='Enter'){event.preventDefault();void execute()}}
onMounted(()=>void focusEditor())
onUnmounted(()=>{disposed=true;if(task)void api.CancelDatabaseTask(task).catch(()=>{})})
</script>
<template><div class="query-editor-panel" @keydown="keys">
 <div class="query-editor-toolbar" role="toolbar" aria-label="查询操作">
  <button class="native-button default-button" :disabled="running||!connected||!sql.trim()" @click="execute"><Play :size="14"/>执行（选中或全部）</button>
  <button class="native-button" :disabled="!running||stopping" @click="cancel"><Square :size="12"/>{{stopping?'正在停止…':'停止'}}</button>
  <span class="query-editor-target"><Plug :size="13"/>{{profile.name}}<Database :size="13"/>{{database}}</span>
 </div>
 <p v-if="!connected" class="query-editor-disconnected" role="status">此查询的连接已关闭。重新打开“{{profile.name}}”后可继续执行，SQL 和已有结果仍保留。</p>
 <div class="query-editor-workspace" :class="{'has-editable-result':!!query}">
  <textarea ref="editor" v-model="sql" class="query-editor-input" aria-label="SQL 编辑器" spellcheck="false" :readonly="running" placeholder="在此输入 SQL，Ctrl+Enter 执行选中内容或全部语句。"/>
  <p v-if="!query" class="query-editor-note">SQL 批次每次执行使用独立会话，批次内事务须在同一次执行中完成。出错即停止，已提交操作不会自动回滚。<template v-if="profile.tunnelMode==='http'">PHP 中转需启用 BAJIU_TUNNEL_ALLOW_SQL=1；停止或关闭标签页只取消客户端等待，服务器可能仍继续执行。</template></p>
  <div class="query-editor-feedback" aria-live="polite"><span v-if="running" role="status"><LoaderCircle :size="13" class="spin"/>正在执行…</span><p v-if="error" class="db-tools-error" role="alert">{{error}}</p><p v-if="message" class="db-tools-message" role="status">{{message}}</p></div>
  <TableDataEditor v-if="query" ref="tableEditor" :key="queryID" class="query-editable-result" :profile="profile" :database="query.database" :table="query.table" section="data" :query="query" :active="active" :connected="connected" :sessionKey="sessionKey??''" @dirty="childDirty=$event" @busy="childBusy=$event" @query-unavailable="unavailable" @confirm="emit('confirm',$event)" @quick-edit="emit('quick-edit',$event)"/>
  <div v-else class="query-editor-results" :aria-busy="running">
   <section v-for="(r,index) in result?.results??[]" :key="index" class="query-result-set"><h3>结果 {{index+1}}<span v-if="!r.columns.length"> · 影响行数 {{r.affected}}</span><span v-if="r.truncated"> · 仅显示前 1,000 行</span></h3><div v-if="r.columns.length" class="query-result-table"><table><thead><tr><th v-for="(column,i) in r.columns" :key="i">{{column}}</th></tr></thead><tbody><tr v-for="(row,n) in r.rows" :key="n"><td v-for="(value,i) in row" :key="i" :class="{null:value===null}">{{value===null?'NULL':value}}</td></tr></tbody></table></div></section>
   <div v-if="!result&&!running" class="query-editor-empty">查询结果将显示在这里</div>
  </div>
 </div>
</div></template>
