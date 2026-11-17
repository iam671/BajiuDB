import type {TableColumn} from './tableEditing'
export const clampColumnWidth=(width:number)=>Math.max(72,Math.min(1200,Math.round(width)))
export function defaultColumnWidth(column:TableColumn){
 return /^(tinyint|smallint|mediumint|int|integer|bigint|decimal|numeric|float|double|real|bit|bool|boolean|year)\b/i.test(column.type)?100:150
}
export function readColumnLayout(key:string,columns:TableColumn[]){
 const widths:Record<string,number>=Object.create(null),hidden:string[]=[]
 try{const raw=localStorage.getItem(key);if(!raw||raw.length>200000)return {widths,hidden};const value=JSON.parse(raw);if(value?.version!==1)return {widths,hidden};const savedHidden=new Set(Array.isArray(value.hidden)?value.hidden.filter((name:unknown)=>typeof name==='string').slice(0,1000):[]);for(const c of columns){const width=value.widths?.[c.name];if(Object.hasOwn(value.widths??{},c.name)&&typeof width==='number'&&Number.isFinite(width))widths[c.name]=clampColumnWidth(width);if(savedHidden.has(c.name))hidden.push(c.name)}if(hidden.length===columns.length)hidden.length=0}catch{/* Invalid layout data never blocks a database table. */}
 return {widths,hidden}
}
