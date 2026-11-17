import {onMounted,onUnmounted} from 'vue'

export function useMenuDismissal(root:()=>HTMLElement|undefined,dismiss:()=>void){
 const outside=(event:Event)=>{
  const menu=root()
  if(menu&&!(event.target instanceof Node&&menu.contains(event.target)))dismiss()
 }
 onMounted(()=>{
  document.addEventListener('pointerdown',outside,true)
  document.addEventListener('contextmenu',outside,true)
  window.addEventListener('blur',dismiss)
 })
 onUnmounted(()=>{
  document.removeEventListener('pointerdown',outside,true)
  document.removeEventListener('contextmenu',outside,true)
  window.removeEventListener('blur',dismiss)
 })
}
