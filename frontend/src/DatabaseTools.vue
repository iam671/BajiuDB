<script setup lang="ts">
import {vDraggableDialog} from './draggableDialog'
import {computed,nextTick,onMounted,onUnmounted,ref} from 'vue'
import {X,Play,Square,FolderOpen,Save,Database,LoaderCircle} from '@lucide/vue'
import {api,errorText,type AdminRequest,type AdminResult,type Profile} from './api'
import {dictionaryHTML,modelSVG} from './databaseReports'
import {quoteIdentifier} from './tableData'
import SearchableSelect from './SearchableSelect.vue'
import TableWorkflows from './TableWorkflows.vue'
export type DatabaseToolMode='create'|'alter'|'drop'|'query'|'console'|'run'|'dump'|'dictionary'|'model'|'createTable'|'import'|'export'|'generate'|'openTable'|'designTable'
const props=defineProps<{profile:Profile;database:string;mode:DatabaseToolMode;table?:string;initialSQL?:string}>()
const emit=defineEmits<{close:[];changed:[operation:string,name:string]}>()
const names:Record<DatabaseToolMode,string>={create:'新建数据库',alter:'编辑数据库',drop:'删除数据库',query:'新建查询',console:'命令行界面',run:'运行 SQL 文件',dump:'转储 SQL 文件',dictionary:'数据字典',model:'逆向数据库到模型',createTable:'新建表',import:'导入向导',export:'导出向导',generate:'数据生成',openTable:'打开表',designTable:'设计表'}
const running=ref(false),error=ref(''),message=ref(''),sql=ref(props.initialSQL??''),file=ref(''),statements=ref(0),includeData=ref(true),name=ref(props.mode==='create'?'':props.database),confirmName=ref(''),charset=ref('utf8mb4'),collation=ref(''),catalog=ref<(string|null)[][]>([]),result=ref<AdminResult|null>(null),svg=ref(''),zoom=ref(1),consoleLines=ref<string[]>([]),history:string[]=[],historyIndex=ref(0),editor=ref<HTMLTextAreaElement>(),windowRef=ref<HTMLElement>()
const workflow=ref<InstanceType<typeof TableWorkflows>>()
const workflowMode=computed(()=>['createTable','import','export','generate'].includes(props.mode)?props.mode as 'createTable'|'import'|'export'|'generate':null)
const currentTableDDL=ref(''),alterClause=ref('')
const alterSQL=computed(()=>'ALTER TABLE '+quoteIdentifier(props.table??'')+' '+alterClause.value.trim()+';')
let task=''
const charsets=computed(()=>[...new Set(catalog.value.map(r=>r[1]??''))].filter(Boolean).sort())
const collations=computed(()=>catalog.value.filter(r=>r[1]===charset.value).map(r=>r[0]??''))
function selectCharset(){if(!charsets.value.includes(charset.value))return;if(collations.value.includes(collation.value))return;collation.value=collations.value.find(c=>c===charset.value+'_general_ci')??collations.value[0]??''}
async function request(request:AdminRequest){task=crypto.randomUUID();try{return await api.DatabaseAdmin(task,props.profile.id,props.database,request)}finally{task=''}}
async function perform(action:()=>Promise<void>){if(running.value)return;running.value=true;error.value='';message.value='';try{await action()}catch(e){error.value=errorText(e)}finally{running.value=false}}
async function cancel(){if(task)try{await api.CancelDatabaseTask(task)}catch(e){error.value=errorText(e)}}
function close(){if(workflowMode.value){workflow.value?.requestClose();return}if(running.value){void cancel();return}emit('close')}
defineExpose({requestClose:close})
function trap(event:KeyboardEvent){const nodes=Array.from(windowRef.value?.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]')??[]).filter(el=>el.getClientRects().length);if(event.shiftKey&&document.activeElement===nodes[0]){event.preventDefault();nodes.at(-1)?.focus()}else if(!event.shiftKey&&document.activeElement===nodes.at(-1)){event.preventDefault();nodes[0]?.focus()}}
async function loadFile(){await perform(async()=>{const chosen=await api.OpenSQLFile();if(!chosen.path)return;file.value=chosen.path;sql.value=chosen.text;statements.value=chosen.statements;result.value=null})}
async function execute(){
 const command=sql.value.trim();if(!command)return
 if(props.mode==='console'){
  if(command==='clear'){consoleLines.value=[];sql.value='';return}if(command==='exit'){close();return}if(command==='help'){consoleLines.value.push('help · clear · exit；输入 SQL，Ctrl+Enter 执行。每次执行为独立会话，事务须在同一批次内完成。');sql.value='';return}
 }
 await perform(async()=>{let text=sql.value;if(props.mode==='query'&&editor.value){const a=editor.value.selectionStart,b=editor.value.selectionEnd;if(a!==b)text=text.slice(a,b)}
  const response=await request({operation:'query',sql:text});result.value=response
  if(props.mode==='console'){consoleLines.value.push('mysql> '+text,...response.results.map(r=>r.columns.length?[r.columns.join('\t'),...r.rows.map(row=>row.map(c=>c??'NULL').join('\t')),r.truncated?'（最多显示 1,000 行）':''].join('\n'):'影响行数：'+r.affected),response.error||'已完成 '+response.completed+' 条语句');history.push(text);historyIndex.value=history.length;sql.value=''}
  if(response.error)error.value=response.error;else message.value='已完成 '+response.completed+' 条语句'+(response.completed>100?'；仅显示前 100 个结果':'')
 })
}
function consoleHistory(event:KeyboardEvent){if(props.mode!=='console'||event.key!=='ArrowUp'||sql.value.includes('\n'))return;event.preventDefault();historyIndex.value=Math.max(0,historyIndex.value-1);sql.value=history[historyIndex.value]??''}
async function saveReport(extension:string,text:string){const path=await api.SaveDatabaseReport(props.database+'.'+extension,text);if(path)message.value='已保存：'+path}
async function saveSchema(){await perform(async()=>{const response=await request({operation:props.mode,name:name.value,charset:charset.value,collation:collation.value});if(response.error)throw new Error(response.error);emit('changed',props.mode,name.value);emit('close')})}
async function applyTableDesign(){if(!props.table||!alterClause.value.trim())return;await perform(async()=>{const response=await request({operation:'query',sql:alterSQL.value});if(response.error)throw new Error(response.error);alterClause.value='';message.value='表结构已更新';emit('changed','table',props.table!);try{currentTableDDL.value=await api.TableDDL(props.profile.id,props.database,props.table!)}catch(e){currentTableDDL.value='';message.value='修改已执行，请刷新表列表确认最新结构。'+errorText(e)}})}
async function dump(){await perform(async()=>{const response=await request({operation:'dump',includeData:includeData.value});result.value=response;await saveReport('sql',response.text)})}
onMounted(async()=>{
 await nextTick();(editor.value??Array.from(windowRef.value?.querySelectorAll<HTMLElement>('input,select,textarea,button')??[]).find(el=>el.getClientRects().length))?.focus()
 if(props.mode==='openTable'&&props.table){sql.value='SELECT * FROM '+quoteIdentifier(props.table)+' LIMIT 1001;';await execute()}
 if(props.mode==='designTable'&&props.table)await perform(async()=>{currentTableDDL.value=await api.TableDDL(props.profile.id,props.database,props.table!)})
 if(props.mode==='create'||props.mode==='alter')await perform(async()=>{catalog.value=(await request({operation:'catalog'})).results[0]?.rows??[];selectCharset();if(props.mode==='alter'){const schema=(await request({operation:'schema'})).results[0]?.rows[0];if(schema){charset.value=schema[0]??'';collation.value=schema[1]??''}}})
 if(props.mode==='dictionary'||props.mode==='model')await perform(async()=>{result.value=await request({operation:props.mode});if(props.mode==='dictionary'&&props.table)result.value.results.forEach(r=>{r.rows=r.rows.filter(row=>row[0]===props.table)});if(props.mode==='model')svg.value=modelSVG(props.database,result.value)})
})
onUnmounted(()=>{if(task)void api.CancelDatabaseTask(task)})
</script>
<template>
 <div class="db-tools-overlay" @keydown.esc.stop.prevent="close" @keydown.tab="trap"><section v-draggable-dialog ref="windowRef" class="db-tools-window" :class="{'db-tools-compact':['create','alter','drop'].includes(mode),'db-tools-drop':mode==='drop'}" role="dialog" aria-modal="true" :aria-label="names[mode]">
  <header><Database :size="16"/><h2>{{names[mode]}} · {{profile.name}}<template v-if="mode!=='create'"> / {{database}}</template></h2><button class="tiny-button" aria-label="关闭数据库工具" @click="close"><X :size="17"/></button></header>
  <TableWorkflows v-if="workflowMode" ref="workflow" :profile="profile" :database="database" :table="table" :mode="workflowMode" @close="emit('close')" @changed="(operation,name)=>emit('changed',operation,name)"/>
  <div v-else class="db-tools-body">
   <template v-if="mode==='create'||mode==='alter'"><p class="db-tools-note">{{mode==='alter'?'修改数据库默认字符集和排序规则，不会重命名数据库或转换已有表的数据。':'创建一个空数据库，权限由当前 MySQL 账户决定。'}}</p><div class="db-schema-form"><label for="schema-name">数据库名</label><input id="schema-name" v-model="name" :disabled="mode==='alter'||running" maxlength="64"/><label for="schema-charset">字符集</label><SearchableSelect label="字符集" id="schema-charset" v-model="charset" :options="charsets" @change="selectCharset" :disabled="running"/><label for="schema-collation">排序规则</label><SearchableSelect label="排序规则" id="schema-collation" v-model="collation" :options="collations" :disabled="running"/></div></template>
   <template v-else-if="mode==='drop'"><p class="db-danger">删除数据库将永久删除其中的表、数据、视图和其他对象，无法通过 Bajiu DB 撤销。请先完成备份。</p><label for="drop-confirm">输入数据库名 <strong>{{database}}</strong> 确认删除</label><input id="drop-confirm" v-model="confirmName" :disabled="running" autocomplete="off"/></template>
   <template v-else-if="mode==='designTable'"><p class="db-tools-note">{{table}} · 当前表定义</p><pre class="db-sql-preview">{{currentTableDDL||'正在读取表定义…'}}</pre><label for="table-alter-clause">结构修改（ALTER TABLE 子句）</label><textarea id="table-alter-clause" class="db-sql-editor" v-model="alterClause" :disabled="running" spellcheck="false" placeholder="例如：ADD COLUMN `remark` VARCHAR(255) NULL"/><p class="db-tools-note">填写增加/修改/删除字段、索引等修改子句。请检查下方完整 SQL 后点击“应用修改”；结构修改会影响真实表，已提交操作无法自动撤销。</p><pre v-if="alterClause.trim()" class="db-sql-preview">{{alterSQL}}</pre></template>
   <template v-else-if="['query','console','run','openTable'].includes(mode)">
    <div class="db-tools-toolbar"><button v-if="mode==='run'" class="native-button" @click="loadFile" :disabled="running"><FolderOpen :size="14"/>选择 SQL 文件</button><button class="native-button default-button" @click="execute" :disabled="running||!sql.trim()"><Play :size="14"/>{{mode==='openTable'?'刷新':mode==='run'?'执行文件':'执行'}}<span v-if="mode==='query'">（选中或全部）</span></button><span v-if="file" class="db-file-path">{{file}} · {{statements}} 条语句</span></div>
    <p v-if="mode==='openTable'" class="db-tools-note">{{table}} · 显示前 1,000 行数据。点击“刷新”重新读取。</p>
    <p v-else class="db-tools-note">SQL 可修改数据。每次执行为独立会话，事务须在同一批次内完成；出错即停止，已提交操作不会自动回滚。Ctrl+Enter 执行。<template v-if="mode==='run'">先检查下方 SQL 预览，再点击执行文件。仅支持 UTF-8、8 MB 内文件。</template><template v-if="profile.tunnelMode==='http'">PHP 中转停止仅取消客户端等待，服务器可能仍继续执行。SQL 功能需在网站设置 BAJIU_TUNNEL_ALLOW_SQL=1。</template></p>
    <pre v-if="mode==='console'" class="db-console-output">{{consoleLines.join('\n\n')||'Bajiu DB SQL 命令行 · help 查看帮助'}}</pre>
    <textarea v-if="mode!=='openTable'" ref="editor" class="db-sql-editor" v-model="sql" :readonly="mode==='run'||running" :aria-label="mode==='console'?'SQL 命令':'SQL 编辑器'" spellcheck="false" @keydown.ctrl.enter.prevent="execute" @keydown="consoleHistory"/>
    <div v-if="result&&mode!=='console'" class="db-query-results"><section v-for="(r,index) in result.results" :key="index"><h3>结果 {{index+1}}<span v-if="!r.columns.length"> · 影响行数 {{r.affected}}</span><span v-if="r.truncated"> · 仅显示前 1,000 行</span></h3><table v-if="r.columns.length"><thead><tr><th v-for="(c,i) in r.columns" :key="i">{{c}}</th></tr></thead><tbody><tr v-for="(row,n) in r.rows" :key="n"><td v-for="(value,i) in row" :key="i" :class="{null:value===null}">{{value===null?'NULL':value}}</td></tr></tbody></table></section></div>
   </template>
   <template v-else-if="mode==='dump'"><p>导出当前数据库的表和视图结构，可包含表数据。</p><label class="native-check"><input type="checkbox" v-model="includeData" :disabled="running"/>包含表数据</label><p class="db-tools-note">不包含触发器、存储过程、函数和事件。使用 InnoDB 一致性快照；并发结构变更、非事务表不保证一致。文件最多 64 MB；PHP 中转还受网站超时限制。请恢复到相同名称的空库，不自动覆盖已有对象。</p><button class="native-button default-button" @click="dump" :disabled="running"><Save :size="14"/>转储并保存 SQL 文件</button></template>
   <template v-else-if="mode==='dictionary'&&result"><div class="db-tools-toolbar"><button class="native-button" :disabled="running" @click="perform(()=>saveReport('html',dictionaryHTML(database,result!)))"><Save :size="14"/>导出 HTML 字典</button></div><section v-for="table in result.results[0]?.rows" :key="table[0]??''" class="db-dictionary-table"><h3>{{table[0]}} <small>{{table[1]}} · {{table[2]}}</small></h3><p>{{table[3]}}</p><table><thead><tr><th>字段</th><th>类型</th><th>允许空值</th><th>默认值</th><th>键</th><th>附加属性</th><th>注释</th></tr></thead><tbody><tr v-for="column in result.results[1]?.rows.filter(c=>c[0]===table[0])" :key="column[1]??''"><td v-for="(value,i) in column.slice(1)" :key="i">{{value===null?'NULL':value}}</td></tr></tbody></table></section></template>
   <template v-else-if="mode==='model'&&svg"><div class="db-tools-toolbar"><button class="native-button" @click="perform(()=>saveReport('svg',svg))" :disabled="running"><Save :size="14"/>保存 SVG 模型</button><label>缩放 <input type="range" min="0.3" max="2" step="0.1" v-model.number="zoom"/> {{Math.round(zoom*100)}}%</label></div><p class="db-tools-note">◆ 表示主键，连线来自服务器声明的外键；不推测未声明的关系。最多显示 200 个表/视图，跨库外键不绘制到当前模型。</p><div class="db-model-canvas"><div :style="{zoom}" v-html="svg"></div></div></template>
   <p v-if="running" role="status" class="db-tools-status"><LoaderCircle :size="15" class="spin"/>正在处理…<button class="native-button" @click="cancel"><Square :size="12"/>停止</button></p><p v-if="error" class="db-tools-error" role="alert">{{error}}</p><p v-if="message" class="db-tools-message" role="status">{{message}}</p>
  </div>
  <footer v-if="!workflowMode"><button v-if="mode==='designTable'" class="native-button default-button" @click="applyTableDesign" :disabled="running||!currentTableDDL||!alterClause.trim()">应用修改</button><button v-if="mode==='create'||mode==='alter'" class="native-button default-button" @click="saveSchema" :disabled="running||!name||!collation">保存</button><button v-if="mode==='drop'" class="native-button db-delete-button" @click="saveSchema" :disabled="running||confirmName!==database">删除数据库</button><button class="native-button" @click="close">{{running?'停止操作':'关闭'}}</button></footer>
 </section></div>
</template>
