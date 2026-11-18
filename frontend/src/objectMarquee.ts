import {computed,onUnmounted,ref,watch,type Ref} from 'vue'
type Box={left:number;top:number;width:number;height:number}
type Options={viewport:Ref<HTMLElement|undefined>;enabled:()=>boolean;scope:()=>string;selection:()=>Iterable<string>;apply:(keys:string[])=>void;start?:()=>void}
export function useObjectMarquee(options:Options){
 const rectangle=ref<Box|null>(null),enabled=computed(options.enabled),scope=computed(options.scope)
 let release:(()=>void)|undefined
 watch(options.viewport,(root,_,cleanup)=>{
  if(!root)return
  let suppressClick=false,suppressDoubleClick=false,stop:((restore?:boolean)=>void)|undefined
  const bounds=()=>{const r=root.getBoundingClientRect();return {left:r.left+root.clientLeft,top:r.top+root.clientTop,right:r.left+root.clientLeft+root.clientWidth,bottom:r.top+root.clientTop+root.clientHeight}}
  const inside=(x:number,y:number)=>{const r=bounds();return x>=r.left&&x<r.right&&y>=r.top&&y<r.bottom}
  const blocked=(target:EventTarget|null)=>{if(!(target instanceof Element))return true;const button=target.closest('button');return !!target.closest('thead,input,textarea,select,a,[role="separator"],[contenteditable="true"]')||!!button&&!button.hasAttribute('data-object-key')}
  const click=(event:MouseEvent)=>{
   if(suppressClick){suppressClick=false;if(event.detail>0){event.preventDefault();event.stopImmediatePropagation();return}}
   if(enabled.value&&!blocked(event.target)&&!(event.target as Element).closest('[data-object-key]')&&inside(event.clientX,event.clientY))options.apply([])
  }
  const doubleClick=(event:MouseEvent)=>{if(suppressDoubleClick){suppressDoubleClick=false;event.preventDefault();event.stopImmediatePropagation()}}
  const down=(event:PointerEvent)=>{
   if(event.button!==0||event.isPrimary===false||event.pointerType==='touch'||!enabled.value||blocked(event.target)||!inside(event.clientX,event.clientY))return
   stop?.();suppressClick=false;suppressDoubleClick=false
   const id=event.pointerId,initial=new Set(options.selection()),context=scope.value,mode=event.ctrlKey||event.metaKey?'toggle':event.shiftKey?'add':'replace',origin=bounds(),startX=event.clientX-origin.left+root.scrollLeft,startY=event.clientY-origin.top+root.scrollTop
   let active=false,frame=0,x=event.clientX,y=event.clientY,last=new Set(initial),items:{key:string;left:number;right:number;top:number;bottom:number}[]=[]
   const valid=()=>enabled.value&&scope.value===context&&root.isConnected
   const edge=(value:number,start:number,end:number)=>value<start+24?-Math.min(18,Math.ceil((start+24-value)/2)):value>end-24?Math.min(18,Math.ceil((value-end+24)/2)):0
   const tick=()=>{
    frame=0;if(!active)return;if(!valid()){finish();return}
    const r=bounds(),header=root.querySelector('thead')?.getBoundingClientRect().height??0,oldTop=root.scrollTop,oldLeft=root.scrollLeft
    root.scrollTop=Math.max(0,root.scrollTop+edge(y,r.top+header,r.bottom));root.scrollLeft=Math.max(0,root.scrollLeft+edge(x,r.left,r.right))
    const endX=Math.min(r.right,Math.max(r.left,x))-r.left+root.scrollLeft,endY=Math.min(r.bottom,Math.max(r.top+header,y))-r.top+root.scrollTop
    const box={left:Math.min(startX,endX),top:Math.min(startY,endY),width:Math.abs(endX-startX),height:Math.abs(endY-startY)}
    rectangle.value=box
    const keys=mode==='replace'?new Set<string>():new Set(initial)
    for(const item of items){if(item.right>box.left&&item.left<box.left+Math.max(1,box.width)&&item.bottom>box.top&&item.top<box.top+Math.max(1,box.height)){if(mode==='toggle'&&initial.has(item.key))keys.delete(item.key);else keys.add(item.key)}}
    if(keys.size!==last.size||[...keys].some(key=>!last.has(key))){last=keys;options.apply([...keys])}
    if(root.scrollTop!==oldTop||root.scrollLeft!==oldLeft)schedule()
   }
   const schedule=()=>{if(active&&!frame)frame=requestAnimationFrame(tick)}
   const move=(e:PointerEvent)=>{
    if(e.pointerId!==id)return;x=e.clientX;y=e.clientY
    if(!active){if(Math.hypot(x-event.clientX,y-event.clientY)<5)return;if(!valid()){finish();return}
     active=true;options.start?.();const r=bounds()
     items=Array.from(root.querySelectorAll<HTMLElement>('[data-object-key]')).map(node=>{const box=node.getBoundingClientRect();return {key:node.dataset.objectKey!,left:box.left-r.left+root.scrollLeft,right:box.right-r.left+root.scrollLeft,top:box.top-r.top+root.scrollTop,bottom:box.bottom-r.top+root.scrollTop}})
     root.classList.add('is-marquee-selecting');try{root.setPointerCapture(id)}catch{/* Document listeners still complete the gesture. */}
    }
    e.preventDefault();schedule()
   }
   const finish=(restore=false)=>{
    const wasActive=active;active=false;if(frame){cancelAnimationFrame(frame);frame=0}
    document.removeEventListener('pointermove',move);document.removeEventListener('pointerup',up);document.removeEventListener('pointercancel',cancel);document.removeEventListener('keydown',key,true);window.removeEventListener('blur',cancel);window.removeEventListener('resize',cancel);root.removeEventListener('scroll',schedule)
    root.classList.remove('is-marquee-selecting');rectangle.value=null;stop=undefined;release=undefined
    try{if(root.hasPointerCapture(id))root.releasePointerCapture(id)}catch{/* The viewport may already be detached. */}
    if(wasActive){suppressClick=true;suppressDoubleClick=true;if(restore&&valid())options.apply([...initial])}
   }
   const up=(e:PointerEvent)=>{if(e.pointerId!==id)return;if(active){if(frame){cancelAnimationFrame(frame);frame=0}x=e.clientX;y=e.clientY;tick()}finish()}
   const cancel=()=>finish(true)
   const key=(e:KeyboardEvent)=>{if(e.key==='Escape'){e.preventDefault();e.stopImmediatePropagation();finish(true)}}
   stop=finish;release=finish;document.addEventListener('pointermove',move,{passive:false});document.addEventListener('pointerup',up);document.addEventListener('pointercancel',cancel);document.addEventListener('keydown',key,true);window.addEventListener('blur',cancel);window.addEventListener('resize',cancel);root.addEventListener('scroll',schedule,{passive:true})
  }
  root.addEventListener('pointerdown',down);root.addEventListener('click',click,true);root.addEventListener('contextmenu',click,true);root.addEventListener('dblclick',doubleClick,true)
  cleanup(()=>{stop?.();root.removeEventListener('pointerdown',down);root.removeEventListener('click',click,true);root.removeEventListener('contextmenu',click,true);root.removeEventListener('dblclick',doubleClick,true)})
 },{flush:'post'})
 watch([enabled,scope],()=>release?.(),{flush:'sync'})
 onUnmounted(()=>release?.())
 return rectangle
}
