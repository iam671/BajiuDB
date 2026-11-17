import {quoteIdentifier,sqlText} from './tableData'
import type {CellDraft,TableColumn} from './tableEditing'
export type TableCopyFormat='insert'|'update'|'tsv-data'|'tsv-fields'|'tsv-all'
export function cellClipboardText(cell:CellDraft){return cell.mode==='null'?'NULL':cell.mode==='default'?'DEFAULT':cell.value}
function columnLiteral(column:TableColumn,cell:CellDraft):string{
 if(cell.mode==='null')return 'NULL'
 if(cell.mode==='default')return 'DEFAULT'
 const value=cell.value
 if(column.spatial)throw new Error('空间字段暂不支持复制为 SQL，请使用 SQL 转储')
 if(column.binary){const hex=value.replace(/^0x/i,'');if(hex.length%2||!/^[0-9a-f]*$/i.test(hex))throw new Error(column.name+' 需要有效的十六进制内容');if(/^bit\b/i.test(column.type)){if(hex.length>16)throw new Error('BIT 值超过 64 位');return hex?BigInt('0x'+hex).toString():'0'}return "X'"+hex+"'"}
 if(/^(tinyint|smallint|mediumint|int|integer|bigint|decimal|numeric|float|double|real|year)\b/i.test(column.type)&&/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(value))return value
 // Quotes alone use standard doubled apostrophes. Backslashes and control
 // characters use UTF-8 hex literals, independent of NO_BACKSLASH_ESCAPES.
 if(/[\\\x00-\x1f\x7f]/.test(value))return sqlText(value)
 return "'"+value.replace(/'/g,"''")+"'"
}
const tsvCell=(value:string)=>/["\t\r\n]/.test(value)?'"'+value.replace(/"/g,'""')+'"':value
export function copyTableRecord(format:TableCopyFormat,database:string,table:string,columns:TableColumn[],cells:CellDraft[],keys:string[],original:(string|null)[]|undefined,visible:number[]){
 if(cells.length!==columns.length)throw new Error('记录字段不完整，请刷新后重试')
 if(format.startsWith('tsv-')){
  const fields=visible.map(i=>tsvCell(columns[i]?.name??'')).join('\t'),data=visible.map(i=>tsvCell(cellClipboardText(cells[i]!))).join('\t')
  return format==='tsv-fields'?fields:format==='tsv-data'?data:fields+'\r\n'+data
 }
 const name=quoteIdentifier(database)+'.'+quoteIdentifier(table),writable=columns.map((column,index)=>({column,index})).filter(item=>!item.column.generated)
 if(!writable.length)throw new Error('没有可复制的普通字段')
 if(format==='insert')return 'INSERT INTO '+name+' ('+writable.map(item=>quoteIdentifier(item.column.name)).join(', ')+') VALUES ('+writable.map(item=>columnLiteral(item.column,cells[item.index]!)).join(', ')+');'
 if(!original||original.length!==columns.length||!keys.length)throw new Error('复制为 UPDATE 需要已有记录及非空唯一键')
 const where=keys.map(key=>{const index=columns.findIndex(c=>c.name===key);if(index<0)throw new Error('唯一键字段不存在');const value=original[index]??null;return quoteIdentifier(key)+' <=> '+columnLiteral(columns[index]!,{mode:value===null?'null':'value',value:value??''})}).join(' AND ')
 return 'UPDATE '+name+' SET '+writable.map(item=>quoteIdentifier(item.column.name)+' = '+columnLiteral(item.column,cells[item.index]!)).join(', ')+' WHERE '+where+' LIMIT 1;'
}
