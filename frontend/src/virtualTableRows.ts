import {computed,nextTick,onUnmounted,ref,watch,type Ref} from 'vue'

// A fixed row height keeps native table scroll geometry stable as rows recycle.
export const gridRowHeight=36
const overscan=12
type RowWindow={start:number;end:number;indices:number[];before:number;after:number}

export function useVirtualTableRows(count:Readonly<Ref<number>>,enabled:()=>boolean){
 const viewport=ref<HTMLElement>(),header=ref<HTMLElement>()
 const scrollTop=ref(0),height=ref(0),headerHeight=ref(0)
 let frame=0
 const windowRows=computed<RowWindow>(previous=>{
  const total=count.value
  const first=Math.min(Math.max(0,total-1),Math.floor(scrollTop.value/gridRowHeight))
  const visible=Math.max(gridRowHeight,height.value-headerHeight.value)
  const start=Math.max(0,first-overscan)
  const end=Math.min(total,Math.max(first+1,Math.ceil((scrollTop.value+visible)/gridRowHeight))+overscan)
  const before=start*gridRowHeight,after=(total-end)*gridRowHeight
  // Scrolling within the same row (or horizontally) needs no Vue DOM update.
  if(previous&&previous.start===start&&previous.end===end&&previous.after===after)return previous
  return {start,end,indices:Array.from({length:end-start},(_,i)=>start+i),before,after}
 })
 function measure(){
  const element=viewport.value
  if(!element||!enabled()||!element.clientHeight)return
  scrollTop.value=element.scrollTop
  height.value=element.clientHeight
  headerHeight.value=header.value?.offsetHeight??0
 }
 function schedule(){if(!frame)frame=requestAnimationFrame(()=>{frame=0;measure()})}
 function scrollToRow(index:number){
  const top=Math.max(0,Math.min(index,Math.max(0,count.value-1)))*gridRowHeight
  scrollTop.value=top
  if(viewport.value){viewport.value.scrollTop=top;measure()}
 }
 watch([viewport,header],([element,head],_,cleanup)=>{
  if(!element)return
  element.scrollTop=scrollTop.value
  element.addEventListener('scroll',schedule,{passive:true})
  const observer=new ResizeObserver(schedule)
  observer.observe(element)
  if(head)observer.observe(head)
  measure()
  cleanup(()=>{observer.disconnect();element.removeEventListener('scroll',schedule)})
 },{flush:'post'})
 watch(enabled,active=>{if(active)void nextTick(measure)},{flush:'post'})
 watch(count,()=>{void nextTick(measure)},{flush:'post'})
 onUnmounted(()=>{if(frame)cancelAnimationFrame(frame)})
 return {viewport,header,windowRows,scrollToRow}
}
