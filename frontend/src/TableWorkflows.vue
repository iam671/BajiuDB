<script setup lang="ts">
import {computed,onMounted,onUnmounted,ref} from 'vue'
import {api,errorText,type AdminRequest,type AdminResult,type Profile} from './api'
import {csvText,generatedSQL,importSQL,parseCSV,quoteIdentifier,suggestedExpression} from './tableData'
const props=defineProps<{profile:Profile;database:string;table?:string;mode:'createTable'|'import'|'export'|'generate'}>()
const emit=defineEmits<{close:[];changed:[operation:string,name:string]}>()
const table=ref(props.table??''),tables=ref<string[]>([]),running=ref(false),error=ref(''),message=ref(''),file=ref(''),rows=ref<string[][]>([]),mapping=ref<string[]>([]),columns=ref<(string|null)[][]>([]),preview=ref(''),stage=ref(false),protectFormulas=ref(true),count=ref(10),start=ref(1),fields=ref<{name:string;expression:string;selected:boolean}[]>([])
const definition=ref('`id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n`name` VARCHAR(255) NOT NULL,\nPRIMARY KEY (`id`)')
let task='',disposed=false
const writable=computed(()=>columns.value.filter(c=>!/auto_increment|(?:VIRTUAL|STORED) GENERATED/i.test(c[6]??'')))
const createSQL=computed(()=>'CREATE TABLE '+quoteIdentifier(table.value)+' (\n'+definition.value+'\n) ENGINE=InnoDB;')
async function request(request:AdminRequest):Promise<AdminResult>{task=crypto.randomUUID();try{const result=await api.DatabaseAdmin(task,props.profile.id,props.database,request);if(result.error)throw new Error(result.error);return result}finally{task=''}}
async function perform(action:()=>Promise<void>){if(running.value)return;running.value=true;error.value='';message.value='';try{await action()}catch(e){if(!disposed)error.value=errorText(e)}finally{running.value=false}}
async function cancel(){if(task)await api.CancelDatabaseTask(task).catch(e=>{error.value=errorText(e)})}
function close(){if(running.value){void cancel();return}emit('close')}
defineExpose({requestClose:close})
async function columnsForTable(){stage.value=false;preview.value='';if(!table.value)return;const result=await request({operation:'query',sql:'SHOW FULL COLUMNS FROM '+quoteIdentifier(table.value)});columns.value=result.results[0]?.rows??[];mapping.value=(rows.value[0]??[]).map(name=>writable.value.some(c=>c[0]===name)?name:'');fields.value=writable.value.map(c=>({name:c[0]??'',expression:suggestedExpression(c[1]??'',c[0]??''),selected:c[3]==='NO'&&c[5]===null}))}
async function chooseTable(){await perform(columnsForTable)}
async function chooseCSV(){await perform(async()=>{const chosen=await api.OpenCSVFile();if(!chosen.path)return;const parsed=parseCSV(chosen.text);rows.value=parsed;file.value=chosen.path;mapping.value=parsed[0]!.map(name=>writable.value.some(c=>c[0]===name)?name:'');stage.value=false;preview.value=''})}
function buildPreview(){error.value='';message.value='';try{preview.value=props.mode==='import'?importSQL(table.value,rows.value,mapping.value):generatedSQL(table.value,Number(count.value),Number(start.value),fields.value.filter(f=>f.selected));stage.value=true}catch(e){error.value=errorText(e)}}
async function write(){await perform(async()=>{
 if(props.mode==='createTable'){
  if(!table.value.trim()||table.value.trim()!==table.value||Array.from(table.value).length>64||/[\x00\r\n./\\]/.test(table.value))throw new Error('请输入有效的表名（1–64 字符）')
  if(!definition.value.trim()||/[;]|--|\/\*/.test(definition.value))throw new Error('表定义只能包含字段和约束，不能包含其他 SQL 语句')
 }
 await request({operation:'query',sql:props.mode==='createTable'?createSQL.value:preview.value});stage.value=false;preview.value='';message.value=props.mode==='createTable'?'已创建表 '+table.value:'已执行完成，请核对服务器数据';emit('changed','table',table.value)
})}
async function exportData(){await perform(async()=>{if(!columns.value.length)throw new Error('请先成功读取表字段后再导出');if(columns.value.some(c=>/^(?:(?:tiny|medium|long)?blob|(?:var)?binary|bit|geometry|point|linestring|polygon|multipoint|multilinestring|multipolygon|geometrycollection)\b/i.test(c[1]??'')))throw new Error('CSV 向导暂不导出二进制或空间字段，请使用“转储 SQL 文件”保留原始数据。');const result=await request({operation:'query',sql:'SELECT * FROM '+quoteIdentifier(table.value)+' LIMIT 1001'});const data=result.results[0];if(!data)throw new Error('服务器没有返回结果');if(data.truncated||data.rows.length>1000)throw new Error('当前 CSV 向导最多导出 1,000 行；此表已超过上限。请使用数据库“转储 SQL 文件”导出完整数据。');const path=await api.SaveDatabaseReport(table.value+'.csv',csvText(data,protectFormulas.value));if(path)message.value='已导出 '+data.rows.length+' 行：'+path})}
onMounted(()=>void perform(async()=>{if(props.mode==='createTable'){table.value='';return}const list=await api.ListObjects(props.profile.id,props.database,'tables');tables.value=list.objects.map(o=>o.name);if(!table.value)table.value=tables.value[0]??'';if(list.truncated&&!props.table)throw new Error('表列表超过 10,000 项，请从左侧选择目标表再打开向导');await columnsForTable()}))
onUnmounted(()=>{disposed=true;if(task)void api.CancelDatabaseTask(task)})
</script>
<template><div class="table-workflow">
 <div class="db-tools-body">
  <div class="db-schema-form"><label for="workflow-table">{{mode==='createTable'?'表名':'目标表'}}</label><input v-if="mode==='createTable'" id="workflow-table" v-model="table" maxlength="64" :disabled="running"/><select v-else id="workflow-table" v-model="table" :disabled="running" @change="chooseTable"><option v-for="name in tables" :key="name">{{name}}</option></select></div>
  <template v-if="mode==='createTable'"><label for="table-definition">字段与约束（SQL）</label><textarea id="table-definition" v-model="definition" class="db-sql-editor" :disabled="running" spellcheck="false"/><p class="db-tools-note">填写 CREATE TABLE 括号内的字段和约束。确认下方 SQL 后创建。</p><pre class="db-sql-preview">{{createSQL}}</pre></template>
  <template v-else-if="mode==='import'"><div class="db-tools-toolbar"><button class="native-button" :disabled="running||!table" @click="chooseCSV">选择 CSV 文件</button><span class="db-file-path">{{file}}</span></div><p class="db-tools-note">UTF-8 CSV，首行为字段名，最多 8 MB / 10,000 行。支持引号、逗号和多行内容。\N 代表 NULL，空字段代表空字符串。只追加数据，不覆盖已有行；先检查字段映射和 SQL 预览。</p>
   <table v-if="rows.length" aria-label="CSV 字段映射"><thead><tr><th>CSV 列</th><th>写入字段</th><th>首行预览</th></tr></thead><tbody><tr v-for="(header,index) in rows[0]" :key="index"><td>{{header}}</td><td><select v-model="mapping[index]" :disabled="running" @change="stage=false"><option value="">跳过</option><option v-for="c in writable" :key="c[0]??''" :value="c[0]">{{c[0]}} · {{c[1]}}</option></select></td><td>{{rows[1]?.[index]}}</td></tr></tbody></table><p v-if="rows.length">共 {{rows.length-1}} 行数据。</p>
   <button class="native-button" :disabled="running||!rows.length||!table" @click="buildPreview">预览导入 SQL</button>
  </template>
  <template v-else-if="mode==='export'"><p>导出所选表的 UTF-8 CSV 文件，包含字段名；\N 代表 NULL。最多 1,000 行，超限会停止并提示，不会保存不完整文件。</p><label class="native-check"><input v-model="protectFormulas" type="checkbox" :disabled="running"/>防止电子表格公式执行（为 =、+、-、@ 等开头的值添加单引号）</label><p class="db-tools-note">防护会改变这些文本值；需要原始值用于再导入时可取消勾选。完整数据库备份请使用“转储 SQL 文件”。</p></template>
  <template v-else-if="mode==='generate'"><p class="db-tools-note">在目标表追加测试数据，不删除原数据。自动排除自增和生成列；<code v-pre>{{n}}</code> 替换为序号。表达式会作为 SQL 执行，唯一键、外键和字段约束仍由 MySQL 检查。</p><div class="db-tools-toolbar"><label>生成行数<input v-model.number="count" type="number" min="1" max="1000" :disabled="running" @input="stage=false"/></label><label>起始序号<input v-model.number="start" type="number" min="1" max="1000000000" :disabled="running" @input="stage=false"/></label></div><table aria-label="生成字段"><thead><tr><th>包含</th><th>字段</th><th>SQL 表达式</th></tr></thead><tbody><tr v-for="field in fields" :key="field.name"><td><input v-model="field.selected" type="checkbox" :disabled="running" @change="stage=false"/></td><td>{{field.name}}</td><td><input v-model="field.expression" :aria-label="field.name+'表达式'" :disabled="running" placeholder="填写 SQL 表达式" @input="stage=false"/></td></tr></tbody></table><button class="native-button" :disabled="running||!table" @click="buildPreview">预览生成 SQL</button></template>
  <template v-if="stage"><p class="db-tools-note">请核对 SQL。事务仅对支持事务的表生效；出错或停止后非事务表可能保留已写入的数据。</p><textarea class="db-sql-editor" :value="preview" readonly aria-label="写入 SQL 预览"/></template>
  <p v-if="running" role="status">正在处理… <button class="native-button" @click="cancel">停止</button></p><p v-if="error" class="db-tools-error" role="alert">{{error}}</p><p v-if="message" class="db-tools-message" role="status">{{message}}</p>
 </div>
 <footer><button v-if="mode==='createTable'" class="native-button default-button" :disabled="running||!table||!definition.trim()" @click="write">创建表</button><button v-else-if="mode==='export'" class="native-button default-button" :disabled="running||!table" @click="exportData">导出 CSV</button><button v-else class="native-button default-button" :disabled="running||!stage||!preview" @click="write">{{mode==='import'?'确认导入':'确认生成'}}</button><button class="native-button" @click="close">{{running?'停止操作':'关闭'}}</button></footer>
</div></template>
