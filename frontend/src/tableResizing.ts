import {onMounted,onUnmounted} from 'vue'

type TableState={table:HTMLTableElement;heads:HTMLTableCellElement[];signature:string;resizeObserver:ResizeObserver|undefined;dispose:()=>void;initialize:()=>void}
const minimum=24,maximum=1800
const clamp=(value:number)=>Math.max(minimum,Math.min(maximum,Math.round(value)))
// Install once per application document, including native tool windows.
export function installTableResizing(root:Document=document){
 const states=new Map<HTMLTableElement,TableState>()
 let frame=0,disposed=false,stopDrag:(()=>void)|undefined
 function schedule(){if(!disposed&&!frame)frame=requestAnimationFrame(()=>{frame=0;scan()})}
 function setup(table:HTMLTableElement,heads:HTMLTableCellElement[],signature:string){
  const originalWidth=table.style.width,originalMin=table.style.minWidth,originalLayout=table.style.tableLayout
  let ready=false,widths:number[]=[],defaults:number[]=[],cols:HTMLTableColElement[]=[],ownedGroup:HTMLTableColElement|undefined
  const cleanups:(()=>void)[]=[],oldColStyles:string[]=[],positions=heads.map(h=>h.style.position)
  const key='bajiu.table-columns.v1.'+JSON.stringify([table.dataset.columnLayoutKey??table.getAttribute('aria-label')??table.className,signature])
  function apply(){table.style.tableLayout='fixed';table.style.width=widths.reduce((a,b)=>a+b,0)+'px';table.style.minWidth=table.style.width;cols.forEach((col,i)=>col.style.width=widths[i]+'px');heads.forEach((head,i)=>{const handle=head.querySelector('[data-table-resize-handle]');handle?.setAttribute('aria-valuenow',String(widths[i]))})}
  function persist(){try{localStorage.setItem(key,JSON.stringify({version:1,widths}))}catch{/* Layout remains usable when storage is unavailable. */}}
  function setWidth(index:number,value:number){widths[index]=clamp(value);apply()}
  function initialize(){
   if(ready||!table.isConnected||!table.getClientRects().length||!heads.some(h=>h.getBoundingClientRect().width>0))return
   defaults=heads.map(h=>clamp(h.getBoundingClientRect().width));widths=[...defaults]
   try{const saved=JSON.parse(localStorage.getItem(key)??'null');if(saved?.version===1&&Array.isArray(saved.widths)&&saved.widths.length===heads.length&&saved.widths.every((v:unknown)=>typeof v==='number'&&Number.isFinite(v)))widths=saved.widths.map(clamp)}catch{}
   const group=Array.from(table.children).find(c=>c.tagName==='COLGROUP') as HTMLTableColElement|undefined
   if(group&&group.children.length===heads.length&&Array.from(group.children).every(c=>(c as HTMLTableColElement).span<=1))cols=Array.from(group.children) as HTMLTableColElement[]
   else{ownedGroup=root.createElement('colgroup');cols=heads.map(()=>root.createElement('col'));cols.forEach(c=>ownedGroup!.append(c));table.insertBefore(ownedGroup,table.tHead??table.firstChild)}
   oldColStyles.push(...cols.map(c=>c.getAttribute('style')??''));ready=true
   heads.forEach((head,index)=>{
    if(getComputedStyle(head).position==='static')head.style.position='relative'
    const label=head.textContent?.trim()||'第 '+(index+1)+' 列',handle=root.createElement('span')
    handle.className='table-header-resizer';handle.dataset.tableResizeHandle='';handle.tabIndex=0;handle.role='separator';handle.setAttribute('aria-orientation','vertical');handle.setAttribute('aria-label','调整'+label+'列宽');handle.setAttribute('aria-valuemin',String(minimum));handle.setAttribute('aria-valuemax',String(maximum));handle.title='拖动调整列宽，双击恢复默认'
    const click=(event:Event)=>event.stopPropagation()
    const reset=(event:Event)=>{event.preventDefault();event.stopPropagation();setWidth(index,defaults[index]!);persist()}
    const keys=(event:KeyboardEvent)=>{if(!['ArrowLeft','ArrowRight','Home'].includes(event.key))return;event.preventDefault();event.stopPropagation();setWidth(index,event.key==='Home'?defaults[index]!:widths[index]!+(event.key==='ArrowRight'?1:-1)*(event.shiftKey?24:8));persist()}
    const down=(event:PointerEvent)=>{
     if(event.button!==0)return;event.preventDefault();event.stopPropagation();stopDrag?.()
     const start=event.clientX,width=widths[index]!,cursor=root.body.style.cursor,select=root.body.style.userSelect
     handle.setPointerCapture?.(event.pointerId);root.body.style.cursor='col-resize';root.body.style.userSelect='none'
     const move=(e:PointerEvent)=>setWidth(index,width+e.clientX-start)
     const finish=()=>{root.removeEventListener('pointermove',move);root.removeEventListener('pointerup',finish);root.removeEventListener('pointercancel',finish);window.removeEventListener('blur',finish);try{handle.releasePointerCapture?.(event.pointerId)}catch{}root.body.style.cursor=cursor;root.body.style.userSelect=select;stopDrag=undefined;persist()}
     root.addEventListener('pointermove',move);root.addEventListener('pointerup',finish,{once:true});root.addEventListener('pointercancel',finish,{once:true});window.addEventListener('blur',finish,{once:true});stopDrag=finish
    }
    handle.addEventListener('pointerdown',down);handle.addEventListener('click',click);handle.addEventListener('dblclick',reset);handle.addEventListener('keydown',keys);head.append(handle)
    cleanups.push(()=>{handle.removeEventListener('pointerdown',down);handle.removeEventListener('click',click);handle.removeEventListener('dblclick',reset);handle.removeEventListener('keydown',keys);handle.remove();head.style.position=positions[index]!})
   })
   apply()
  }
  const resizeObserver=typeof ResizeObserver==='function'?new ResizeObserver(initialize):undefined;resizeObserver?.observe(table)
  const state:TableState={table,heads,signature,resizeObserver,initialize,dispose:()=>{stopDrag?.();resizeObserver?.disconnect();cleanups.forEach(cleanup=>cleanup());ownedGroup?.remove();if(!ownedGroup)cols.forEach((col,i)=>{const style=oldColStyles[i];if(style)col.setAttribute('style',style);else col.removeAttribute('style')});table.style.width=originalWidth;table.style.minWidth=originalMin;table.style.tableLayout=originalLayout}}
  states.set(table,state);initialize()
 }
 function scan(){
  for(const [table,state] of states)if(!table.isConnected){state.dispose();states.delete(table)}
  for(const table of Array.from(root.querySelectorAll<HTMLTableElement>('table'))){
   // The data grid already has type-based widths, drag handles and per-table caching.
   if(table.closest('.table-data-grid')||!table.tHead)continue
   const heads=Array.from(table.tHead.rows).at(-1)?.cells;if(!heads?.length)continue
   const cells=Array.from(heads);if(cells.some(h=>h.colSpan!==1||h.rowSpan!==1))continue
   const signature=cells.map(h=>h.textContent?.trim()??'').join('\u001f'),existing=states.get(table)
   if(existing&&existing.signature===signature&&existing.heads.every((h,i)=>h===cells[i])){existing.initialize();continue}
   existing?.dispose();setup(table,cells,signature)
  }
 }
 const observer=new MutationObserver(records=>{if(records.some(record=>{
  if(record.target instanceof Element&&record.target.closest('thead'))return true
  return [...Array.from(record.addedNodes),...Array.from(record.removedNodes)].some(node=>node instanceof Element&&!node.matches('[data-table-resize-handle]')&&(node.matches('table,thead,th')||!!node.querySelector('table,thead,th')))
 }))schedule()})
 observer.observe(root.body,{subtree:true,childList:true});scan()
 return ()=>{disposed=true;observer.disconnect();if(frame)cancelAnimationFrame(frame);stopDrag?.();for(const state of states.values())state.dispose();states.clear()}
}
export function useTableResizing(){let cleanup:(()=>void)|undefined;onMounted(()=>{cleanup=installTableResizing()});onUnmounted(()=>cleanup?.())}
