import type {TableColumn} from './tableEditing'
export const clampColumnWidth=(width:number)=>Math.max(72,Math.min(1200,Math.round(width)))
export function defaultColumnWidth(column:TableColumn){
 const t=column.type.toLowerCase();let width=180
 if(/^(tinyint|smallint|mediumint|int|year|bit)\b/.test(t))width=110
 else if(/^bigint\b/.test(t))width=172
 else if(/^(decimal|numeric|float|double|real)\b/.test(t))width=184
 else if(/^(datetime|timestamp)\b/.test(t))width=196
 else if(/^(date|time)\b/.test(t))width=140
 else if(/^(json|(?:tiny|medium|long)?text)\b/.test(t))width=300
 else if(column.binary)width=240
 else if(/^(var)?char\b/.test(t)){const length=Number(t.match(/\((\d+)\)/)?.[1]??32);width=Math.min(320,Math.max(140,length*8+20))}
 return clampColumnWidth(Math.max(width,Math.min(360,column.name.length*9+30)))
}
export function readColumnLayout(key:string,columns:TableColumn[]){
 const widths:Record<string,number>=Object.create(null),hidden:string[]=[]
 try{const raw=localStorage.getItem(key);if(!raw||raw.length>200000)return {widths,hidden};const value=JSON.parse(raw);if(value?.version!==1)return {widths,hidden};const savedHidden=new Set(Array.isArray(value.hidden)?value.hidden.filter((name:unknown)=>typeof name==='string').slice(0,1000):[]);for(const c of columns){const width=value.widths?.[c.name];if(Object.hasOwn(value.widths??{},c.name)&&typeof width==='number'&&Number.isFinite(width))widths[c.name]=clampColumnWidth(width);if(savedHidden.has(c.name))hidden.push(c.name)}if(hidden.length===columns.length)hidden.length=0}catch{/* Invalid layout data never blocks a database table. */}
 return {widths,hidden}
}
