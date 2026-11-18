import type {QueryResult} from './api'
// Only determines whether Enter submits. Actual SQL parsing/validation stays on the server.
export function consoleComplete(sql:string){
 let quote='',line=false,block=false,last=''
 for(let i=0;i<sql.length;i++){const c=sql[i]!,next=sql[i+1]
  if(line){if(c==='\n')line=false;continue}if(block){if(c==='*'&&next==='/'){block=false;i++}continue}
  if(quote){if(c==='\\'&&quote!=='`'){i++;continue}if(c===quote){if(next===quote)i++;else quote=''}continue}
  if(c==='#'||c==='-'&&next==='-'&&/\s/.test(sql[i+2]??'')){line=true;if(c==='-')i++;continue}
  if(c==='/'&&next==='*'){block=true;i++;continue}
  if(c==="'"||c==='"'||c==='`'){quote=c;last='value';continue}
  if(!/\s/.test(c))last=c
 }
 return !quote&&!block&&last===';'
}
export function consoleUse(sql:string){
 const match=sql.trim().match(/^(?:USE\s+|\\u\s+)(?:`((?:``|[^`])+)`|([\p{L}\p{N}_$-]+))\s*;?\s*$/iu)
 return match?(match[1]?.replace(/``/g,'`')??match[2]):undefined
}
const clean=(value:string|null)=>value===null?'NULL':value.replace(/[\x00-\x1f\x7f]/g,c=>c==='\n'?'\\n':c==='\r'?'\\r':c==='\t'?'\\t':'\\x'+c.charCodeAt(0).toString(16).padStart(2,'0'))
const width=(value:string)=>Array.from(value).reduce((n,c)=>n+(/[\u1100-\u115f\u2e80-\ua4cf\uac00-\ud7af\uf900-\ufaff\ufe10-\ufe19\ufe30-\ufe6f\uff00-\uff60\uffe0-\uffe6]/u.test(c)?2:1),0)
function preview(value:string|null){let text='',n=0;for(const c of value??'NULL'){const escaped=clean(c),w=width(escaped);if(n+w>48)return {text:text+'…',clipped:true};text+=escaped;n+=w}return {text,clipped:false}}
export function consoleResult(result:QueryResult){
 if(!result.columns.length)return 'Query OK, '+result.affected+' row(s) affected'
 let clipped=false
 const cell=(value:string|null)=>{const result=preview(value);clipped ||= result.clipped;return result.text}
 const count=Math.min(40,result.columns.length),names=result.columns.slice(0,count).map(cell),rows=result.rows.map(row=>row.slice(0,count).map(cell)),widths=names.map((name,i)=>Math.max(width(name),...rows.map(row=>width(row[i]??'')))),border='+'+widths.map(n=>'-'.repeat(n+2)).join('+')+'+'
 const line=(row:string[])=>'|'+widths.map((n,i)=>' '+(row[i]??'')+' '.repeat(Math.max(0,n-width(row[i]??'')))).join(' |')+' |'
 const output=[border,line(names),border];let size=output.join('\n').length,shown=0
 for(const row of rows){const text=line(row);if(size+text.length>200000)break;output.push(text);size+=text.length+1;shown++}
 output.push(border,shown+' row(s) in set')
 if(result.truncated||shown<rows.length||count<result.columns.length)output.push('显示已限制：最多 1,000 行、40 列及 200 KB 输出，请缩小查询范围。')
 if(clipped)output.push('长单元格已缩略显示；可在查询标签或表数据页查看完整值。')
 return output.join('\n')
}
