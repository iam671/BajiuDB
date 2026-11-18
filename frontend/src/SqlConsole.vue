<script setup lang="ts">
import {computed,nextTick,onMounted,onUnmounted,ref,watch} from 'vue'
import {Square} from '@lucide/vue'
import {api,errorText,type Profile} from './api'
import {quoteIdentifier} from './tableData'
import {consoleComplete,consoleResult,consoleUse} from './sqlConsole'
const consoleID='sql-console-'+crypto.randomUUID()
const props=defineProps<{profile:Profile;database:string;active:boolean;connected:boolean;sessionKey:string}>()
const emit=defineEmits<{dirty:[value:boolean];close:[];'database-change':[database:string]}>()
const database=ref(props.database),input=ref(''),running=ref(false),stopping=ref(false),editor=ref<HTMLTextAreaElement>(),viewport=ref<HTMLElement>()
const entries=ref<{id:number;text:string;error:boolean}[]>([]),discarded=ref(false),history:string[]=[],historyIndex=ref(0)
const inputRows=computed(()=>Math.max(1,Math.min(12,input.value.split('\n').length)))
let sequence=0,task='',disposed=false,historyDraft=''
watch([input,running],()=>emit('dirty',!!input.value.trim()||running.value),{immediate:true,flush:'sync'})
async function focus(){await nextTick();if(!disposed&&props.active)editor.value?.focus({preventScroll:true})}
async function bottom(){await nextTick();if(viewport.value&&props.active)viewport.value.scrollTop=viewport.value.scrollHeight}
function append(text:string,error=false){entries.value.push({id:++sequence,text,error});let size=entries.value.reduce((n,e)=>n+e.text.length,0);while(entries.value.length>100||size>2000000&&entries.value.length>1){size-=entries.value.shift()!.text.length;discarded.value=true}void bottom()}
function remember(text:string){if(history.at(-1)!==text)history.push(text);while(history.length>100||history.reduce((n,s)=>n+s.length,0)>1000000)history.shift();historyIndex.value=history.length;historyDraft=''}
async function execute(){
 if(running.value||!props.connected||!input.value.trim())return
 const sql=input.value.trim(),command=sql.replace(/;\s*$/,'').toLowerCase()
 if(sql.length>1000000){append('输入超过 1 MB，请使用“运行 SQL 文件”处理大型脚本。',true);return}
 if(command==='\\c'){input.value='';void focus();return}
 if(command==='clear'){entries.value=[];discarded.value=false;input.value='';void focus();return}
 if(['exit','quit','\\q'].includes(command)){input.value='';emit('close');return}
 append('mysql> '+sql.replace(/\n/g,'\n    -> '));remember(sql);input.value=''
 if(command==='help'||command==='\\h'){append('输入 SQL，以分号结束后 Enter 执行；Ctrl+Enter 可直接执行。Shift+Enter 换行，↑/↓ 历史记录，Ctrl+C 停止，Ctrl+L 清屏。\nUSE 数据库名; / \\u 数据库名 切换本标签的默认数据库。clear 清屏，exit 关闭标签。\n每次提交使用独立会话；事务及会话变量须在同一批次内使用，已提交操作不会自动回滚。'+(props.profile.tunnelMode==='http'?'\nPHP 中转停止仅取消客户端等待，服务器操作可能仍继续。':''));void focus();return}
 const target=consoleUse(sql),requestDatabase=target??database.value,requestSQL=target?'USE '+quoteIdentifier(target)+';':sql
 task=crypto.randomUUID();running.value=true;stopping.value=false
 try{
  const response=await api.DatabaseAdmin(task,props.profile.id,requestDatabase,{operation:'query',sql:requestSQL})
  if(disposed)return
  for(const result of response.results)append(consoleResult(result))
  if(response.error)append(response.error,true)
  else if(target&&response.completed===1){database.value=target;emit('database-change',target);append('Database changed · '+target)}
  if(response.completed>response.results.length)append('已执行 '+response.completed+' 条语句；仅保留前 '+response.results.length+' 个结果。')
 }catch(e){if(!disposed)append(errorText(e),true)}finally{task='';running.value=false;stopping.value=false;if(!disposed)void focus()}
}
async function stop(){if(!task||stopping.value)return;stopping.value=true;try{await api.CancelDatabaseTask(task)}catch(e){if(!disposed)append(errorText(e),true)}finally{stopping.value=false}}
function keys(event:KeyboardEvent){
 if(event.isComposing)return
 if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='l'){event.preventDefault();entries.value=[];discarded.value=false;return}
 if(event.ctrlKey&&event.key.toLowerCase()==='c'&&!window.getSelection()?.toString()&&(running.value||event.target===editor.value&&editor.value?.selectionStart===editor.value?.selectionEnd)){event.preventDefault();if(running.value)void stop();else{input.value='';append('^C')}return}
 if(event.key==='Enter'&&!event.shiftKey){if(event.ctrlKey||event.metaKey||consoleComplete(input.value)||/^(?:help|clear|exit|quit|\\[hquc])(?:\s|;|$)/i.test(input.value.trim())){event.preventDefault();void execute()}return}
 if(event.target===editor.value&&['ArrowUp','ArrowDown'].includes(event.key)&&!event.shiftKey&&!event.ctrlKey&&!event.metaKey){const node=editor.value;if(!node||node.selectionStart!==node.selectionEnd)return;const up=event.key==='ArrowUp';if(input.value.includes('\n')&&(up?node.selectionStart>0:node.selectionStart<input.value.length))return;event.preventDefault();if(historyIndex.value===history.length)historyDraft=input.value;historyIndex.value=Math.max(0,Math.min(history.length,historyIndex.value+(up?-1:1)));input.value=historyIndex.value===history.length?historyDraft:history[historyIndex.value]??'';void nextTick(()=>node.setSelectionRange(node.value.length,node.value.length))}
}
watch(()=>props.active,value=>{if(value){void focus();void bottom()}})
watch(()=>props.connected,value=>{if(!value){if(task)void stop();append('连接已关闭；重新连接后可继续，输入和历史仍保留。',true)}})
watch(()=>props.sessionKey,(value,old)=>{if(old&&value&&old!==value)append('连接已重新打开，后续命令使用新的数据库会话。')})
onMounted(()=>void focus())
onUnmounted(()=>{disposed=true;if(task)void api.CancelDatabaseTask(task).catch(()=>{})})
</script>
<template><section class="sql-console" :aria-label="profile.name+' 命令行界面'" @keydown="keys">
 <div class="sql-console-context"><span>{{profile.name}} / <strong>{{database}}</strong></span><button v-if="running" class="link-button" :disabled="stopping" @click="stop"><Square :size="11"/>{{stopping?'正在停止…':'停止'}}</button></div>
 <div ref="viewport" class="sql-console-viewport"><p v-if="discarded" class="sql-console-retention">较早输出已移除；历史记录仅保存在当前标签内存中。</p><pre v-for="entry in entries" :key="entry.id" :class="{'sql-console-error':entry.error}" :role="entry.error?'alert':undefined">{{entry.text}}</pre>
  <div class="sql-console-prompt"><label :for="consoleID">mysql&gt;</label><textarea :id="consoleID" ref="editor" v-model="input" :rows="inputRows" :readonly="running||!connected" aria-label="SQL 命令" :aria-busy="running" spellcheck="false" autocomplete="off" autocapitalize="off" wrap="off"/></div>
  <p v-if="!connected" class="sql-console-offline" role="status">连接已关闭</p>
 </div>
</section></template>
