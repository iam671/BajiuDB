import type {CellDraft,TableColumn} from './tableEditing'
export type CellValuePayload={column:TableColumn;cell:CellDraft;isNew:boolean}
export type CellValueRequest=CellValuePayload&{apply:(cell:CellDraft)=>void}
export type QuickKind='date'|'datetime'|'time'|'year'|'enum'|'set'|'boolean'|'number'|'json'|'binary'|'text'
export function quickKind(column:TableColumn):QuickKind{
 const type=column.type.toLowerCase()
 if(/^date\b/.test(type))return 'date'
 if(/^(datetime|timestamp)\b/.test(type))return 'datetime'
 if(/^time\b/.test(type))return 'time'
 if(/^year\b/.test(type))return 'year'
 if(/^enum\(/.test(type))return 'enum'
 if(/^set\(/.test(type))return 'set'
 if(/^(?:tinyint\(1\)|bit\(1\)|bool(?:ean)?\b)/.test(type))return 'boolean'
 if(column.binary)return 'binary'
 if(/^(?:(?:tiny|small|medium|big)?int|integer|decimal|numeric|float|double|real)\b/.test(type))return 'number'
 if(/^json\b/.test(type))return 'json'
 return 'text'
}
export function enumValues(type:string):string[]{
 const values:string[]=[];const body=type.slice(type.indexOf('(')+1,-1);let i=0
 while(i<body.length){while(/\s/.test(body[i]??''))i++;if(body[i++]!=="'")return [];let value='',closed=false
  while(i<body.length){const c=body[i++]!;if(c==='\\'){const next=body[i++];if(next===undefined)return [];value+=({n:'\n',r:'\r',t:'\t','0':'\0',b:'\b',Z:'\x1a'} as Record<string,string>)[next]??next}else if(c==="'"){if(body[i]==="'"){value+="'";i++}else{closed=true;break}}else value+=c}
  if(!closed)return [];values.push(value);while(/\s/.test(body[i]??''))i++;if(i<body.length&&body[i++]!==',')return []
 }
 return values
}
export const pad=(value:number)=>String(value).padStart(2,'0')
export function localDate(date=new Date()){return date.getFullYear()+'-'+pad(date.getMonth()+1)+'-'+pad(date.getDate())}
export function localTime(type:string,date=new Date()){const precision=Math.min(6,Number(type.match(/\((\d)\)/)?.[1]??0));return pad(date.getHours())+':'+pad(date.getMinutes())+':'+pad(date.getSeconds())+(precision?'.'+String(date.getMilliseconds()).padStart(3,'0').padEnd(6,'0').slice(0,precision):'')}
export function validDate(value:string){const match=value.match(/^(\d{4})-(\d{2})-(\d{2})$/);if(!match)return false;const y=Number(match[1]),m=Number(match[2]),d=Number(match[3]),date=new Date(y,m-1,d);return y>=1000&&y<=9999&&date.getFullYear()===y&&date.getMonth()===m-1&&date.getDate()===d}
export function validTime(value:string,type:string,clock=false){const m=value.match(/^(-?)(\d{1,3}):([0-5]\d):([0-5]\d)(?:\.(\d{1,6}))?$/);return !!m&&(!clock||!m[1])&&Number(m[2])<=(clock?23:838)&&(m[5]?.length??0)<=Number(type.match(/\((\d)\)/)?.[1]??0)}
// Exact decimal arithmetic: never route database values through a JS Number.
export function stepNumber(value:string,delta:1|-1){
 const m=value.match(/^([+-]?)(\d+)(?:\.(\d*))?$/);if(!m||value.length>10000)throw Error('请先输入普通十进制数值')
 const fraction=m[3]??'',scale=10n**BigInt(fraction.length),base=BigInt(m[2]!+fraction)*(m[1]==='-'?-1n:1n)+BigInt(delta)*scale
 const digits=(base<0n?-base:base).toString().padStart(fraction.length+1,'0')
 return (base<0n?'-':'')+(fraction.length?digits.slice(0,-fraction.length)+'.'+digits.slice(-fraction.length):digits)
}
export function formatJSON(value:string){
 if(value.length>300000)throw Error('JSON 过大，请直接编辑原始内容')
 JSON.parse(value) // Validate only. Serialising parsed numbers would corrupt large integers.
 let out='',depth=0,quoted=false,escape=false
 for(const c of value){if(quoted){out+=c;if(escape)escape=false;else if(c==='\\')escape=true;else if(c==='"')quoted=false;continue}
  if(c==='"'){quoted=true;out+=c}else if(/\s/.test(c))continue
  else if(c==='{'||c==='['){if(++depth>64)throw Error('JSON 嵌套过深，请直接编辑原始内容');out+=c+'\n'+'  '.repeat(depth)}
  else if(c==='}'||c===']'){depth--;out+='\n'+'  '.repeat(depth)+c}
  else if(c===',')out+=',\n'+'  '.repeat(depth)
  else if(c===':')out+=': ';else out+=c
 }
 return out
}
export function cellValueError(payload:CellValuePayload):string{
 const {column,cell,isNew}=payload;if(cell.mode==='null')return column.nullable?'':'此字段不允许 NULL'
 if(cell.mode==='default')return isNew?'':'已有记录不能使用新增默认值'
 const kind=quickKind(column),value=cell.value
 if(kind==='date'&&!validDate(value))return '请输入有效日期，格式 YYYY-MM-DD'
 if(kind==='datetime'){const parts=value.split(' ');if(parts.length!==2||!validDate(parts[0]!)||!validTime(parts[1]!,column.type,true))return '请输入有效日期时间，小数秒精度须与字段一致'}
 if(kind==='time'&&!validTime(value,column.type))return '请输入 HH:MM:SS，可使用负数及最多 838 小时，小数秒精度须与字段一致'
 if(kind==='year'&&!/^(?:0|0000|19\d{2}|20\d{2}|21[0-4]\d|215[0-5])$/.test(value))return 'YEAR 范围为 1901–2155 或 0000'
 if(kind==='year'&&value==='1900')return 'YEAR 范围为 1901–2155 或 0000'
 if(kind==='enum'&&!enumValues(column.type).includes(value))return '请选择或输入枚举中的值'
 if(kind==='set'&&value.split(',').filter(Boolean).some(v=>!enumValues(column.type).includes(v)))return 'SET 包含未定义的选项'
 // TINYINT(1) is a display width, not a server constraint to just 0/1.
 if(kind==='boolean'&&!(column.binary?/^(?:00|01)$/:/^[+-]?\d+$/).test(value))return column.binary?'请选择 0 或 1':'请输入整数，可快捷选择 0 或 1'
 if(kind==='number'&&!/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:e[+-]?\d+)?$/i.test(value))return '请输入有效数值'
 if(kind==='json'){try{JSON.parse(value)}catch(e){return 'JSON 格式错误：'+String(e)}}
 if(kind==='binary'&&!/^(?:[0-9a-f]{2})*$/i.test(value))return 'HEX 内容必须为偶数位十六进制字符'
 return ''
}
