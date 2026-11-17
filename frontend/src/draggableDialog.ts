import type {Directive} from 'vue'

const cleanups=new WeakMap<HTMLElement,()=>void>()
const controls='button,input,select,textarea,a,[contenteditable="true"],[role="button"]'

// Shared by every application dialog. Pointer capture keeps the drag active
// outside its title bar; offsets intentionally have no viewport boundary.
export const vDraggableDialog:Directive<HTMLElement>={
 mounted(dialog){
  const handle=dialog.querySelector<HTMLElement>(':scope > header')
  if(!handle)return
  handle.classList.add('dialog-drag-handle')
  handle.title='拖动标题栏移动，双击标题栏或 Alt+Home 居中'
  if(dialog.closest('.native-dialog-host')){
   const center=(event:KeyboardEvent)=>{if(event.altKey&&event.key==='Home'){event.preventDefault();void import('@wailsio/runtime').then(({Window})=>Window.Center())}}
   dialog.addEventListener('keydown',center);cleanups.set(dialog,()=>dialog.removeEventListener('keydown',center));return
  }
  let x=0,y=0,stopDrag:(()=>void)|undefined
  const render=()=>{dialog.style.transform=`translate(${x}px,${y}px)`}
  const blocked=(event:Event)=>event.target instanceof Element&&!!event.target.closest(controls)
  const center=()=>{stopDrag?.();x=0;y=0;render()}
  const doubleClick=(event:MouseEvent)=>{if(!blocked(event))center()}
  const key=(event:KeyboardEvent)=>{if(event.altKey&&event.key==='Home'){event.preventDefault();event.stopPropagation();center()}}
  const start=(event:PointerEvent)=>{
   if(event.button!==0||event.isPrimary===false||blocked(event)||dialog.closest('[inert]'))return
   event.preventDefault();stopDrag?.()
   const startX=x,startY=y,clientX=event.clientX,clientY=event.clientY,id=event.pointerId
   handle.setPointerCapture?.(id)
   const move=(e:PointerEvent)=>{if(e.pointerId===id){x=startX+e.clientX-clientX;y=startY+e.clientY-clientY;render()}}
   const stop=(e?:PointerEvent)=>{
    if(e&&e.pointerId!==id)return
    document.removeEventListener('pointermove',move);document.removeEventListener('pointerup',stop);document.removeEventListener('pointercancel',stop);handle.removeEventListener('lostpointercapture',stop)
    if(handle.isConnected&&handle.hasPointerCapture?.(id))handle.releasePointerCapture(id)
    stopDrag=undefined
   }
   stopDrag=stop;document.addEventListener('pointermove',move);document.addEventListener('pointerup',stop);document.addEventListener('pointercancel',stop);handle.addEventListener('lostpointercapture',stop)
  }
  render();handle.addEventListener('pointerdown',start);handle.addEventListener('dblclick',doubleClick);dialog.addEventListener('keydown',key)
  cleanups.set(dialog,()=>{stopDrag?.();handle.removeEventListener('pointerdown',start);handle.removeEventListener('dblclick',doubleClick);dialog.removeEventListener('keydown',key)})
 },
 beforeUnmount(dialog){cleanups.get(dialog)?.();cleanups.delete(dialog)},
}
