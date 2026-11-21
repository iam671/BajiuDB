<script setup lang="ts">
import {computed,onMounted,onUnmounted,ref,watch} from 'vue'
import {api,errorText,type AdminRequest,type AdminResult,type Profile,type TransferFormat,type TransferOptions} from './api'
import {generatedSQL,importSQL,quoteIdentifier,suggestedExpression} from './tableData'
import SearchableSelect from './SearchableSelect.vue'
const props=defineProps<{profile:Profile;database:string;table?:string;mode:'createTable'|'import'|'export'|'generate'}>()
const emit=defineEmits<{close:[];changed:[operation:string,name:string]}>()
const table=ref(props.table??''),tables=ref<string[]>([]),running=ref(false),error=ref(''),message=ref(''),file=ref(''),rows=ref<(string|null)[][]>([]),mapping=ref<string[]>([]),columns=ref<(string|null)[][]>([]),preview=ref(''),stage=ref(false),protectFormulas=ref(true),count=ref(10),start=ref(1),fields=ref<{name:string;expression:string;selected:boolean}[]>([])
const format=ref('csv'),formats=ref<TransferFormat[]>([]),separator=ref('tab'),sourceTables=ref<string[]>([]),sourceTable=ref(''),connectionString=ref(''),exportTable=ref(props.table??'data')
const selectedFormat=computed(()=>formats.value.find(item=>item.id===format.value)),formatAvailable=computed(()=>selectedFormat.value?.available===true),isODBC=computed(()=>format.value==='odbc'),isTransfer=computed(()=>props.mode==='import'||props.mode==='export')
const definition=ref('`id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n`name` VARCHAR(255) NOT NULL,\nPRIMARY KEY (`id`)')
let task='',disposed=false
const writable=computed(()=>columns.value.filter(c=>!/auto_increment|(?:VIRTUAL|STORED) GENERATED/i.test(c[6]??'')))
const createSQL=computed(()=>'CREATE TABLE '+quoteIdentifier(table.value)+' (\n'+definition.value+'\n) ENGINE=InnoDB;')
function invalidatePreview(){stage.value=false;preview.value=''}
function clearSource(){rows.value=[];mapping.value=[];invalidatePreview()}
watch(format,()=>{file.value='';sourceTables.value=[];sourceTable.value='';connectionString.value='';clearSource()})
watch([separator,sourceTable],clearSource)
watch(connectionString,()=>{sourceTables.value=[];sourceTable.value='';clearSource()})
function options(exporting=false):TransferOptions{return {format:format.value,path:file.value,table:exporting?exportTable.value:sourceTable.value,delimiter:separator.value,connectionString:connectionString.value,protectFormulas:protectFormulas.value}}
async function transfer<T>(action:(id:string)=>Promise<T>){if(disposed)throw new Error('窗口已关闭');task=crypto.randomUUID();try{return await action(task)}finally{task=''}}
async function request(request:AdminRequest):Promise<AdminResult>{return transfer(async id=>{const result=await api.DatabaseAdmin(id,props.profile.id,props.database,request);if(result.error)throw new Error(result.error);return result})}
async function perform(action:()=>Promise<void>){if(running.value)return;running.value=true;error.value='';message.value='';try{await action()}catch(e){if(!disposed)error.value=errorText(e)}finally{running.value=false}}
async function cancel(){if(task)await api.CancelDatabaseTask(task).catch(e=>{error.value=errorText(e)})}
function close(){if(running.value){void cancel();return}emit('close')}
defineExpose({requestClose:close})
async function columnsForTable(){invalidatePreview();columns.value=[];if(!table.value)return;const result=await request({operation:'query',sql:'SHOW FULL COLUMNS FROM '+quoteIdentifier(table.value)});columns.value=result.results[0]?.rows??[];mapping.value=(rows.value[0]??[]).map(name=>writable.value.some(c=>c[0]===name)?name??'':'');fields.value=writable.value.map(c=>({name:c[0]??'',expression:suggestedExpression(c[1]??'',c[0]??''),selected:c[3]==='NO'&&c[5]===null}));exportTable.value=table.value}
async function chooseTable(){await perform(columnsForTable)}
async function loadSourceRows(){clearSource();const data=await transfer(id=>api.ReadTransferSource(id,options()));if(disposed)return;rows.value=[data.columns,...data.rows];mapping.value=data.columns.map(name=>writable.value.some(c=>c[0]===name)?name:'')}
async function chooseSource(){await perform(async()=>{if(!formatAvailable.value)throw new Error('当前格式不可用：'+(selectedFormat.value?.note??''));if(!isODBC.value){const chosen=await api.SelectTransferFile(format.value);if(!chosen)return;file.value=chosen}clearSource();sourceTables.value=[];sourceTable.value='';const list=await transfer(id=>api.InspectTransferSource(id,options()));sourceTables.value=list;sourceTable.value=list[0]??'';if(list.length===1)await loadSourceRows();if(!list.length)throw new Error('来源中没有可导入的表或工作表')})}
function buildPreview(){error.value='';message.value='';try{if(props.mode==='import'&&rows.value.length<2)throw new Error('来源没有可导入的数据行');preview.value=props.mode==='import'?importSQL(table.value,rows.value,mapping.value,false,new Map(columns.value.map(c=>[c[0]??'',c[1]??'']))):generatedSQL(table.value,Number(count.value),Number(start.value),fields.value.filter(f=>f.selected));stage.value=true}catch(e){invalidatePreview();error.value=errorText(e)}}
async function write(){await perform(async()=>{
 if(props.mode==='createTable'){
  if(!table.value.trim()||table.value.trim()!==table.value||Array.from(table.value).length>64||/[\x00\r\n./\\]/.test(table.value))throw new Error('请输入有效的表名（1–64 字符）')
  if(!definition.value.trim()||/[;]|--|\/\*/.test(definition.value))throw new Error('表定义只能包含字段和约束，不能包含其他 SQL 语句')
 }else if(!stage.value||!preview.value)throw new Error('请先生成并检查 SQL 预览')
 await request({operation:'query',sql:props.mode==='createTable'?createSQL.value:preview.value});invalidatePreview();message.value=props.mode==='createTable'?'已创建表 '+table.value:'已执行完成，请核对服务器数据';emit('changed','table',table.value)
})}
async function exportData(){await perform(async()=>{if(!formatAvailable.value)throw new Error('当前格式不可用：'+(selectedFormat.value?.note??''));if(!columns.value.length)throw new Error('请先成功读取表字段后再导出');const result=await transfer(id=>api.ExportTransferTable(id,props.profile.id,props.database,table.value,options(true)));if(result.path)message.value='已导出 '+result.rows.toLocaleString()+' 行：'+result.path})}
onMounted(()=>void perform(async()=>{if(props.mode==='createTable'){table.value='';return}const [list,catalog]=await Promise.all([api.ListObjects(props.profile.id,props.database,'tables'),isTransfer.value?api.TransferFormats():Promise.resolve([])]);tables.value=list.objects.map(o=>o.name);formats.value=catalog;if(!table.value)table.value=tables.value[0]??'';if(list.truncated&&!props.table)throw new Error('表列表超过 10,000 项，请从左侧选择目标表再打开向导');await columnsForTable()}))
onUnmounted(()=>{disposed=true;connectionString.value='';if(task)void api.CancelDatabaseTask(task)})
</script>
<template><div class="table-workflow">
 <div class="db-tools-body">
  <div class="db-schema-form"><label for="workflow-table">{{mode==='createTable'?'表名':mode==='export'?'来源表':'目标表'}}</label><input v-if="mode==='createTable'" id="workflow-table" v-model="table" maxlength="64" :disabled="running"/><SearchableSelect v-else id="workflow-table" label="MySQL 表" v-model="table" :options="tables" :disabled="running" :allowCustom="false" @change="chooseTable"/>
   <template v-if="isTransfer"><label for="transfer-format">{{mode==='import'?'来源格式':'导出格式'}}</label><SearchableSelect id="transfer-format" label="数据格式" v-model="format" :options="formats.map(item=>({value:item.id,label:item.label+(item.available?'':'（需要驱动）')}))" :allowCustom="false" :disabled="running"/>
    <template v-if="format==='txt'"><label for="transfer-delimiter">分隔符</label><SearchableSelect id="transfer-delimiter" label="文本分隔符" v-model="separator" :options="[{value:'tab',label:'制表符'},{value:',',label:'逗号'},{value:';',label:'分号'},{value:'|',label:'竖线'}]" :allowCustom="false" :disabled="running"/></template>
    <template v-if="isODBC"><label for="transfer-connection">ODBC 连接字符串</label><input id="transfer-connection" v-model="connectionString" type="password" autocomplete="off" :disabled="running" placeholder="DSN=...;UID=...;PWD=..."/></template>
    <template v-if="mode==='export'&&['db','xls','mdb','accdb','odbc'].includes(format)"><label for="transfer-target-table">{{format==='xls'?'工作表名':'导出新表名'}}</label><input id="transfer-target-table" v-model="exportTable" :disabled="running" maxlength="64"/></template>
   </template>
  </div>
  <p v-if="isTransfer&&selectedFormat" class="db-tools-note" :class="{'db-tools-error':!formatAvailable}">{{selectedFormat.note}}</p>
  <template v-if="mode==='createTable'"><label for="table-definition">字段与约束（SQL）</label><textarea id="table-definition" v-model="definition" class="db-sql-editor" :disabled="running" spellcheck="false"/><p class="db-tools-note">填写 CREATE TABLE 括号内的字段和约束。确认下方 SQL 后创建。</p><pre class="db-sql-preview">{{createSQL}}</pre></template>
  <template v-else-if="mode==='import'"><div class="db-tools-toolbar"><button class="native-button" :disabled="running||!table||!formatAvailable||(isODBC&&!connectionString)" @click="chooseSource">{{isODBC?'连接 ODBC 数据源':'选择 '+format.toUpperCase()+' 文件'}}</button><span class="db-file-path">{{file}}</span></div>
   <div v-if="sourceTables.length" class="transfer-source-row"><label for="transfer-source-table">来源表 / 工作表</label><SearchableSelect id="transfer-source-table" label="来源表或工作表" v-model="sourceTable" :options="sourceTables" :disabled="running" :allowCustom="false"/><button class="native-button" :disabled="running||!sourceTable" @click="perform(loadSourceRows)">读取数据</button></div>
   <p class="db-tools-note">文件最多 64 MB；读取最多 10,000 行 / 200,000 单元格 / 8 MB 数据，超限停止。文本和 Excel 首行为字段名，\N 表示 NULL，与 NULL 标记相同的字面值前多加一个反斜杠。二进制字段使用 0xHEX，空间字段使用 SRID=数字;WKB=0xHEX，映射到对应类型后还原。只追加数据，先检查字段映射和 SQL 预览。</p>
   <table v-if="rows.length" aria-label="导入字段映射"><thead><tr><th>来源字段</th><th>写入字段</th><th>首行预览</th></tr></thead><tbody><tr v-for="(header,index) in rows[0]" :key="index"><td>{{header}}</td><td><SearchableSelect :id="'transfer-map-'+index" :label="'映射'+header" v-model="mapping[index]!" :disabled="running" :allowCustom="false" :options="[{value:'',label:'跳过'},...writable.map(c=>({value:c[0]??'',label:c[0]+' · '+c[1]}))]" @change="invalidatePreview"/></td><td>{{rows[1]?.[index]===null?'NULL':rows[1]?.[index]}}</td></tr></tbody></table><p v-if="rows.length">共 {{rows.length-1}} 行数据。</p>
   <button class="native-button" :disabled="running||rows.length<2||!table" @click="buildPreview">预览导入 SQL</button>
  </template>
  <template v-else-if="mode==='export'"><p>按所选格式导出全部记录，不设导出总行数或总大小上限，后台逐行写入，可随时停止。XLSX 超过单张工作表容量会自动分表；其它格式受各自容量约束。只导出字段和数据，不复制索引、约束或触发器；数据库/Excel 格式以文本保留大数原文。二进制字段导出为 0xHEX，空间字段导出为 SRID=数字;WKB=0xHEX；导回对应类型字段时还原。</p><label v-if="format==='csv'||format==='txt'" class="native-check"><input v-model="protectFormulas" type="checkbox" :disabled="running"/>防止电子表格公式执行（为 =、+、-、@ 等开头的值添加单引号）</label><p class="db-tools-note">文本公式防护会改变这些值，重新导入时可取消勾选。ODBC 只创建新表，需允许创建/写入；停止或失败后请核对目标表，驱动的结构操作未必可回滚。连接字符串只在当前窗口内存使用。PHP 隧道使用有序分批读取，导出期间请避免修改来源数据。完整备份请使用“转储 SQL 文件”。</p></template>
  <template v-else-if="mode==='generate'"><p class="db-tools-note">在目标表追加测试数据，不删除原数据。自动排除自增和生成列；<code v-pre>{{n}}</code> 替换为序号。表达式会作为 SQL 执行，唯一键、外键和字段约束仍由 MySQL 检查。</p><div class="db-tools-toolbar"><label>生成行数<input v-model.number="count" type="number" min="1" max="1000" :disabled="running" @input="stage=false"/></label><label>起始序号<input v-model.number="start" type="number" min="1" max="1000000000" :disabled="running" @input="stage=false"/></label></div><table aria-label="生成字段"><thead><tr><th>包含</th><th>字段</th><th>SQL 表达式</th></tr></thead><tbody><tr v-for="field in fields" :key="field.name"><td><input v-model="field.selected" type="checkbox" :disabled="running" @change="stage=false"/></td><td>{{field.name}}</td><td><input v-model="field.expression" :aria-label="field.name+'表达式'" :disabled="running" placeholder="填写 SQL 表达式" @input="stage=false"/></td></tr></tbody></table><button class="native-button" :disabled="running||!table" @click="buildPreview">预览生成 SQL</button></template>
  <template v-if="stage"><p class="db-tools-note">请核对 SQL。事务仅对支持事务的表生效；出错或停止后非事务表可能保留已写入的数据。</p><textarea class="db-sql-editor" :value="preview" readonly aria-label="写入 SQL 预览"/></template>
  <p v-if="running" role="status">正在处理… <button class="native-button" @click="cancel">停止</button></p><p v-if="error" class="db-tools-error" role="alert">{{error}}</p><p v-if="message" class="db-tools-message" role="status">{{message}}</p>
 </div>
 <footer><button v-if="mode==='createTable'" class="native-button default-button" :disabled="running||!table||!definition.trim()" @click="write">创建表</button><button v-else-if="mode==='export'" class="native-button default-button" :disabled="running||!table||!formatAvailable||(isODBC&&(!connectionString||!exportTable))" @click="exportData">导出 {{format.toUpperCase()}}</button><button v-else class="native-button default-button" :disabled="running||!stage||!preview" @click="write">{{mode==='import'?'确认导入':'确认生成'}}</button><button class="native-button" @click="close">{{running?'停止操作':'关闭'}}</button></footer>
</div></template>
