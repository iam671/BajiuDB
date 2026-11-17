import {onUnmounted,watch,type ComputedRef} from 'vue'
import {api,hasNativeWindows,type WindowDialog,type DialogAction} from './api'
type Definition=Omit<WindowDialog,'id'> & {key:string}
export function useNativeDialogs(definitions:ComputedRef<Definition[]>,onAction:(key:string,action:DialogAction)=>void,onError:(message:string)=>void){
 const entries=new Map<string,{id:string;serialized:string}>();let chain=Promise.resolve(),disposed=false
 function received(event:Event){const action=(event as CustomEvent<DialogAction>).detail;for(const[key,entry]of entries)if(entry.id===action.id){onAction(key,action);break}}
 if(hasNativeWindows()){
  window.addEventListener('bajiu:dialog-action',received)
  watch(definitions,()=>{chain=chain.then(async()=>{
   if(disposed)return
   const current=definitions.value,keys=new Set(current.map(d=>d.key))
   for(const[key,entry]of entries){if(!keys.has(key)){entries.delete(key);await api.CloseWindowDialog(entry.id)}}
   for(const{key,...definition}of current){const serialized=JSON.stringify(definition),entry=entries.get(key);if(entry){if(entry.serialized!==serialized){entry.serialized=serialized;await api.UpdateWindowDialog({...definition,id:entry.id})}}else{const id=crypto.randomUUID();entries.set(key,{id,serialized});try{await api.OpenWindowDialog({...definition,id});if(disposed){entries.delete(key);await api.CloseWindowDialog(id)}}catch(e){entries.delete(key);throw e}}}
  }).catch(e=>onError(String(e)))},{immediate:true,deep:true})
 }
 onUnmounted(()=>{disposed=true;window.removeEventListener('bajiu:dialog-action',received);for(const entry of entries.values())void api.CloseWindowDialog(entry.id);entries.clear()})
}
