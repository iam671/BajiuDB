import type {TableColumn} from './tableEditing'
import {quoteIdentifier as qi} from './tableData'

type Token={text:string;value:string;kind:'name'|'word'|'string'|'number'|'symbol'}
export interface EditableQuery{database:string;table:string;alias:string;projection:{name:string;label:string}[]|null;where:string;references:string[];order:{name:string;descending:boolean}[];offset:string;limit?:string}
// Deliberately small grammar. Anything uncertain follows the ordinary read-only SQL path.
export function editableQuery(sql:string,database:string):EditableQuery|undefined{
 try{return parse(sql,database)}catch{return undefined}
}
function parse(sql:string,database:string):EditableQuery{
 if(sql.length>1_000_000)throw Error('query limit')
 const tokens:Token[]=[];let p=0
 while(p<sql.length){
  if(/\s/.test(sql[p]!)){p++;continue}
  const start=p,c=sql[p]!
  if(sql.slice(p,p+2)==='--')throw Error('comment')
  if(c==='`'||c==="'"){
   p++;let value='',closed=false
   while(p<sql.length){const char=sql[p++]!;if(char==='\\'||char==='\0')throw Error('SQL mode');if(char===c){if(sql[p]===c){value+=c;p++}else{closed=true;break}}else value+=char}
   if(!closed)throw Error('quote');tokens.push({text:sql.slice(start,p),value,kind:c==='`'?'name':'string'})
  }else{
   const match=sql.slice(p).match(/^(?:[A-Za-z_][A-Za-z0-9_$]*|[0-9]+(?:\.[0-9]+)?|<=>|<=|>=|<>|!=|[.*,();=<>+\-])/)
   if(!match)throw Error('token');p+=match[0].length;const text=match[0];tokens.push({text,value:text,kind:/^[A-Za-z_]/.test(text)?'word':/^\d/.test(text)?'number':'symbol'})
  }
  if(tokens.length>20000)throw Error('token limit')
 }
 if(tokens.at(-1)?.text===';')tokens.pop()
 if(tokens.some(t=>t.text===';'))throw Error('multiple statements')
 const words=tokens.filter(t=>t.kind==='word').map(t=>t.value.toUpperCase())
 if(words.filter(w=>w==='SELECT').length!==1||words.filter(w=>w==='FROM').length!==1||words.some(w=>['DISTINCT','DISTINCTROW','ALL','UNION','JOIN','GROUP','HAVING','WITH','OVER','INTO','FOR','LOCK','SQL_CALC_FOUND_ROWS','SQL_NO_CACHE','HIGH_PRIORITY','STRAIGHT_JOIN'].includes(w)))throw Error('derived query')
 let i=0
 const word=(value:string)=>tokens[i]?.kind==='word'&&tokens[i]!.value.toUpperCase()===value
 const take=(value:string)=>{if(!word(value))throw Error(value);i++}
 const ident=()=>{const t=tokens[i++];if(!t||!['word','name'].includes(t.kind)||t.kind==='word'&&['NULL','TRUE','FALSE','CURRENT_DATE','CURRENT_TIME','CURRENT_TIMESTAMP','CURRENT_USER','CURRENT_ROLE','LOCALTIME','LOCALTIMESTAMP','UTC_DATE','UTC_TIME','UTC_TIMESTAMP','SESSION_USER','SYSTEM_USER'].includes(t.value.toUpperCase()))throw Error('identifier');return t.value}
 const dotName=()=>{const parts=[ident()];while(tokens[i]?.text==='.'){i++;parts.push(tokens[i]?.text==='*'?(i++,'*'):ident());if(parts.length>3)throw Error('qualified name')}return parts}
 take('SELECT')
 const fields:{parts:string[];label:string}[]=[]
 while(!word('FROM')){
  let parts:string[]
  if(tokens[i]?.text==='*'){i++;parts=['*']}else parts=dotName()
  let label=parts.at(-1)!
  if(word('AS')){i++;label=ident()}else if(tokens[i]&&['word','name'].includes(tokens[i]!.kind)&&!word('FROM'))label=ident()
  fields.push({parts,label});if(tokens[i]?.text!==',')break;i++
 }
 take('FROM');const source=dotName();if(source.length>2||source.includes('*'))throw Error('source')
 const table=source.at(-1)!,db=source.length===2?source[0]!:database
 let alias=''
 if(word('AS')){i++;alias=ident()}else if(tokens[i]&&['word','name'].includes(tokens[i]!.kind)&&!['WHERE','ORDER','LIMIT'].some(word))alias=ident()
 const resolve=(parts:string[])=>{if(parts.length===3&&(parts[0]!==db||parts[1]!==table||alias))throw Error('qualifier');if(parts.length===2&&parts[0]!== (alias||table))throw Error('qualifier');return parts.at(-1)!}
 let projection:EditableQuery['projection']=fields.map(f=>({name:resolve(f.parts),label:f.label}))
 if(projection.some(f=>f.name==='*')){if(projection.length!==1||fields[0]?.label!=='*')throw Error('wildcard mix');projection=null}
 else if(new Set(projection.map(f=>f.name)).size!==projection.length||new Set(projection.map(f=>f.label)).size!==projection.length)throw Error('duplicate projection')
 let where='';const references:string[]=[]
 if(word('WHERE')){
  i++;const begin=i;let depth=0
  const operators=['AND','OR','NOT','IS','NULL','TRUE','FALSE','LIKE','IN','BETWEEN','REGEXP','RLIKE','BINARY','ESCAPE']
  while(i<tokens.length&&!((word('ORDER')||word('LIMIT'))&&depth===0)){
   const token=tokens[i]!
   if(token.text==='('){if(++depth>64)throw Error('depth')}
   else if(token.text===')'){if(--depth<0)throw Error('parentheses')}
   else if(token.kind==='name'||token.kind==='word'){
    if(token.kind==='word'&&operators.includes(token.value.toUpperCase())){i++;continue}
    const parts=dotName();if(tokens[i]?.text==='(')throw Error('function');references.push(resolve(parts));continue
   }
   i++
  }
  if(depth||begin===i)throw Error('where');where=tokens.slice(begin,i).map(t=>t.text).join(' ')
 }
 const order:EditableQuery['order']=[]
 if(word('ORDER')){i++;take('BY');do{if(tokens[i]?.text===',')i++;let name:string
  if(tokens[i]?.kind==='number'){const n=Number(tokens[i++]!.value);if(!projection||!Number.isSafeInteger(n)||n<1||n>projection.length)throw Error('ordinal');name=projection[n-1]!.name}
  else{const parts=dotName();name=parts.length===1?projection?.find(f=>f.label===parts[0])?.name??resolve(parts):resolve(parts)}
  let descending=false;if(word('ASC')||word('DESC')){descending=word('DESC');i++}order.push({name,descending})
 }while(tokens[i]?.text===',')}
 const integer=()=>{const t=tokens[i++];if(t?.kind!=='number'||!/^\d+$/.test(t.value)||BigInt(t.value)>18446744073709551615n)throw Error('limit');return BigInt(t.value).toString()}
 let offset='0',limit:string|undefined
 if(word('LIMIT')){i++;limit=integer();if(tokens[i]?.text===','){i++;offset=limit;limit=integer()}else if(word('OFFSET')){i++;offset=integer()}}
 if(i!==tokens.length)throw Error('unsupported SQL')
 if(!table||!fields.length)throw Error('table')
 return {database:db,table,alias,projection,where,references,order,offset,limit}
}
export function queryProjection(plan:EditableQuery,columns:TableColumn[]){
 const fields=plan.projection??columns.filter(c=>!/\bINVISIBLE\b/i.test(c.extra)).map(c=>({name:c.name,label:c.name}))
 if([...fields.map(f=>f.name),...plan.references,...plan.order.map(o=>o.name)].some(name=>!columns.some(c=>c.name===name)))throw Error('查询字段无法映射到源表')
 return fields.map(f=>({index:columns.findIndex(c=>c.name===f.name),label:f.label}))
}
export function queryPageScope(plan:EditableQuery,page:number){
 const start=BigInt(page-1)*1000n,remaining=plan.limit===undefined?1001n:BigInt(plan.limit)-start
 if(remaining<0n)throw Error('页码超出查询 LIMIT 范围')
 const offset=BigInt(plan.offset)+start;if(offset>18446744073709551615n)throw Error('查询偏移量过大')
 return {alias:plan.alias?' AS '+qi(plan.alias):'',where:plan.where,order:plan.order.map(o=>qi(o.name)+(o.descending?' DESC':' ASC')).join(','),limit:(remaining<1001n?remaining:1001n).toString(),offset:offset.toString()}
}
