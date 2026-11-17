import type {QueryResult} from './api'
import {quoteIdentifier as qi,sqlText} from './tableData'
export type TableSection='data'|'fields'|'indexes'|'foreignKeys'|'triggers'
export interface TableColumn{name:string;type:string;nullable:boolean;key:string;defaultValue:string|null;extra:string;comment:string;generated:boolean;binary:boolean;spatial:boolean}
export interface TableIndex{name:string;unique:boolean;type:string;columns:string[];parts:string[]}
export interface CellDraft{mode:'value'|'null'|'default';value:string}
export interface TableFilter{column:string;operator:'contains'|'equal'|'notEqual'|'greater'|'less'|'null'|'notNull';value:string}
export function resultRecords(result:QueryResult|undefined){return (result?.rows??[]).map(row=>Object.fromEntries((result?.columns??[]).map((name,i)=>[name,row[i]??null])))}
export function readColumns(result:QueryResult|undefined):TableColumn[]{return resultRecords(result).map(r=>{const type=r.Type??'',spatial=/^(geometry|point|linestring|polygon|multipoint|multilinestring|multipolygon|geometrycollection)\b/i.test(type);return {name:r.Field??'',type,nullable:r.Null==='YES',key:r.Key??'',defaultValue:r.Default??null,extra:r.Extra??'',comment:r.Comment??'',generated:/(?:VIRTUAL|STORED) GENERATED/i.test(r.Extra??''),binary:spatial||/^(?:(?:tiny|medium|long)?blob|(?:var)?binary|bit)\b/i.test(type),spatial}})}
export function readIndexes(result:QueryResult|undefined):TableIndex[]{const groups=new Map<string,TableIndex>();for(const r of resultRecords(result).sort((a,b)=>(a.Key_name??'').localeCompare(b.Key_name??'')||Number(a.Seq_in_index)-Number(b.Seq_in_index))){const name=r.Key_name??'';let index=groups.get(name);if(!index){index={name,unique:r.Non_unique==='0',type:r.Index_type??'',columns:[],parts:[]};groups.set(name,index)}if(r.Column_name)index.columns.push(r.Column_name);index.parts.push((r.Column_name??r.Expression??'表达式')+(r.Sub_part?' ('+r.Sub_part+')':''))}return [...groups.values()]}
export function uniqueKey(columns:TableColumn[],indexes:TableIndex[]){return [...indexes].sort((a,b)=>Number(b.name==='PRIMARY')-Number(a.name==='PRIMARY')).find(index=>index.unique&&index.columns.length===index.parts.length&&index.columns.length>0&&index.columns.every(name=>columns.some(c=>c.name===name&&!c.nullable&&!c.spatial)))?.columns??[]}
const tableName=(database:string,table:string)=>qi(database)+'.'+qi(table)
export function metadataSQL(database:string,table:string){const db=sqlText(database),name=sqlText(table);return [
 'SHOW FULL COLUMNS FROM '+tableName(database,table),
 'SHOW INDEX FROM '+tableName(database,table),
 'SELECT k.CONSTRAINT_NAME,k.COLUMN_NAME,k.REFERENCED_TABLE_SCHEMA,k.REFERENCED_TABLE_NAME,k.REFERENCED_COLUMN_NAME,r.UPDATE_RULE,r.DELETE_RULE FROM information_schema.KEY_COLUMN_USAGE k LEFT JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND r.TABLE_NAME=k.TABLE_NAME WHERE k.TABLE_SCHEMA='+db+' AND k.TABLE_NAME='+name+' AND k.REFERENCED_TABLE_NAME IS NOT NULL ORDER BY k.CONSTRAINT_NAME,k.ORDINAL_POSITION',
 'SELECT TRIGGER_NAME,ACTION_TIMING,EVENT_MANIPULATION,ACTION_STATEMENT FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA='+db+' AND EVENT_OBJECT_TABLE='+name+' ORDER BY TRIGGER_NAME',
].join(';\n')+';'}
function readExpression(column:TableColumn){const bit=column.type.match(/^bit(?:\((\d+)\))?/i);if(bit){const width=Math.min(8,Math.max(1,Math.ceil(Number(bit[1]??1)/8)))*2;return 'LPAD(HEX('+qi(column.name)+'),'+width+",'0')"}return column.binary?'HEX('+qi(column.name)+')':'CONVERT('+qi(column.name)+' USING utf8mb4)'}
export function pageSQL(database:string,table:string,columns:TableColumn[],keys:string[],page:number,filter:TableFilter,sort:string,descending:boolean){
 if(!Number.isSafeInteger(page)||page<1||page>1000000)throw new Error('页码须为 1–1,000,000')
 if(!columns.length)throw new Error('请先读取表字段')
 const selected=filter.column?columns.filter(c=>c.name===filter.column):columns
 if(!selected.length)throw new Error('筛选字段不存在，请刷新表结构')
 let where=''
 if(filter.operator==='null'||filter.operator==='notNull'){if(!filter.column)throw new Error('请先选择筛选字段');where=qi(selected[0]!.name)+(filter.operator==='null'?' IS NULL':' IS NOT NULL')}
 else if(filter.value!==''||(filter.column&&filter.operator!=='contains')){const operators:Record<string,string>={equal:'=',notEqual:'<>',greater:'>',less:'<'};where=selected.map(column=>filter.operator==='contains'?'LOCATE('+sqlText(filter.value)+','+readExpression(column)+')>0':qi(column.name)+(operators[filter.operator]??'=')+literal(column,filter.value)).map(text=>'('+text+')').join(' OR ')}
 const order=sort?columns.filter(c=>c.name===sort).map(c=>c.name):keys
 return 'SELECT '+columns.map(c=>readExpression(c)+' AS '+qi(c.name)).join(',')+' FROM '+tableName(database,table)+(where?' WHERE '+where:'')+(order.length?' ORDER BY '+order.map(name=>qi(name)+(descending?' DESC':' ASC')).join(','):'')+' LIMIT 1001 OFFSET '+((page-1)*1000)+';'
}
function literal(column:TableColumn,value:string|null){if(value===null)return 'NULL';if(!column.binary)return sqlText(value);const hex=value.replace(/^0x/i,'');if(hex.length%2||!/^[a-f0-9]*$/i.test(hex))throw new Error(column.name+' 需要偶数位十六进制内容');if(/^bit\b/i.test(column.type)){if(hex.length>16)throw new Error(column.name+' 的 BIT 值最多为 8 字节');return hex?BigInt('0x'+hex).toString():'0'}return "X'"+hex+"'"}
export function makeDraft(columns:TableColumn[],row:(string|null)[]|undefined):CellDraft[]{return columns.map((c,i)=>row?{mode:row[i]===null?'null':'value',value:row[i]??''}:{mode:c.generated||/auto_increment/i.test(c.extra)||c.defaultValue!==null?'default':c.nullable?'null':'value',value:''})}
function rowPredicate(columns:TableColumn[],keys:string[],original:(string|null)[]){
 if(!keys.length)throw new Error('此表没有可用的非空唯一键，编辑和删除已禁用；仍可新增记录')
 if(original.length!==columns.length)throw new Error('记录字段不完整，请刷新后重试')
 const indexed=keys.map(name=>{const i=columns.findIndex(c=>c.name===name);if(i<0)throw new Error('唯一键字段已变化，请刷新');return qi(name)+' <=> '+literal(columns[i]!,original[i]??null)})
 const snapshot=columns.map((c,i)=>'CAST('+readExpression(c)+' AS BINARY) <=> CAST('+(original[i]===null?'NULL':sqlText(original[i]??''))+' AS BINARY)')
 return [...indexed,...snapshot].map(part=>'('+part+')').join(' AND ')
}
export function saveRowSQL(database:string,table:string,columns:TableColumn[],keys:string[],original:(string|null)[]|undefined,draft:CellDraft[]){
 if(draft.length!==columns.length)throw new Error('字段映射不完整，请刷新')
 const writable=columns.map((column,i)=>({column,i,cell:draft[i]!})).filter(({column})=>!column.generated&&!column.spatial)
 for(const {column,cell} of writable)if(cell.mode==='null'&&!column.nullable)throw new Error(column.name+' 不允许 NULL')
 if(!original){const values=writable.filter(({cell})=>cell.mode!=='default');return 'INSERT INTO '+tableName(database,table)+' ('+values.map(({column})=>qi(column.name)).join(',')+') VALUES ('+values.map(({column,cell})=>literal(column,cell.mode==='null'?null:cell.value)).join(',')+');'}
 const changes=writable.filter(({cell,i})=>cell.mode!=='default'&&(cell.mode==='null'?null:cell.value)!==original[i])
 if(!changes.length)throw new Error('没有需要保存的修改')
 return 'UPDATE '+tableName(database,table)+' SET '+changes.map(({column,cell})=>qi(column.name)+'='+literal(column,cell.mode==='null'?null:cell.value)).join(',')+' WHERE '+rowPredicate(columns,keys,original)+' LIMIT 1;'
}
export function deleteRowSQL(database:string,table:string,columns:TableColumn[],keys:string[],original:(string|null)[]){return 'DELETE FROM '+tableName(database,table)+' WHERE '+rowPredicate(columns,keys,original)+' LIMIT 1;'}
