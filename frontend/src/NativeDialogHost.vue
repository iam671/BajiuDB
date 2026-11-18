<script setup lang="ts">
import {useTableResizing} from './tableResizing'
useTableResizing()
import {onMounted,onUnmounted,ref,watch} from 'vue'
import {api,type WindowDialog} from './api'
import ConnectionDialog from './ConnectionDialog.vue'
import GroupDialog from './GroupDialog.vue'
import ConfirmationDialog from './ConfirmationDialog.vue'
import AboutDialog from './AboutDialog.vue'
import DatabaseTools from './DatabaseTools.vue'
import IndexFieldsDialog from './IndexFieldsDialog.vue'
import CellValueDialog from './CellValueDialog.vue'
import TableNameDialog from './TableNameDialog.vue'
const id=new URLSearchParams(location.search).get('dialog')??''
const state=ref<WindowDialog>(),error=ref(''),tool=ref<InstanceType<typeof DatabaseTools>>()
let lastModel:Record<string,unknown>={},localVersion=0,applying=false,syncTimer:ReturnType<typeof setTimeout>|undefined,sendChain=Promise.resolve()
const dirtyFields=new Map<string,{version:number;value:unknown}>()
function apply(next:WindowDialog){
 if(next.id!==id||(next.revision??0)<(state.value?.revision??0))return
 const acknowledged=Number(next.payload.clientVersion??0)
 if(next.payload.model){next.payload.model={...next.payload.model};for(const[key,change]of dirtyFields){if(change.version>acknowledged)next.payload.model[key]=change.value;else dirtyFields.delete(key)}}
 localVersion=Math.max(localVersion,acknowledged);applying=true;state.value=next;lastModel=JSON.parse(JSON.stringify(next.payload.model??{}));applying=false
}
function changed(event:Event){apply((event as CustomEvent<WindowDialog>).detail)}
async function action(name:string,payload:any={}){clearTimeout(syncTimer);const snapshot=JSON.parse(JSON.stringify(payload));sendChain=sendChain.then(()=>api.SendWindowDialogAction({id,action:name,payload:snapshot})).catch(e=>{error.value=String(e)});await sendChain}
function formAction(name:string){void action(name,{model:state.value?.payload.model,clientVersion:localVersion})}
function closeRequested(){if(state.value?.kind==='database')tool.value?.requestClose();else formAction('cancel')}
function keydown(event:KeyboardEvent){if(event.key==='Escape'){event.preventDefault();closeRequested()}else if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='s'&&state.value?.kind==='connection'){event.preventDefault();formAction('save')}}
watch(()=>state.value?.payload.model,model=>{
 if(applying||!model)return
 const keys=Object.keys(model).filter(key=>JSON.stringify(model[key])!==JSON.stringify(lastModel[key]));if(!keys.length)return
 localVersion++;for(const key of keys)dirtyFields.set(key,{version:localVersion,value:model[key]});lastModel=JSON.parse(JSON.stringify(model))
 clearTimeout(syncTimer);syncTimer=setTimeout(()=>{syncTimer=undefined;void action('model',{changes:Object.fromEntries([...dirtyFields].map(([key,change])=>[key,change.value])),clientVersion:localVersion})},40)
},{deep:true,flush:'sync'})
onMounted(async()=>{document.documentElement.classList.add('native-dialog-document');window.addEventListener('bajiu:dialog-state',changed);window.addEventListener('bajiu:dialog-close',closeRequested);window.addEventListener('keydown',keydown);try{apply(await api.GetWindowDialog(id))}catch(e){error.value=String(e)}})
onUnmounted(()=>{clearTimeout(syncTimer);window.removeEventListener('bajiu:dialog-state',changed);window.removeEventListener('bajiu:dialog-close',closeRequested);window.removeEventListener('keydown',keydown)})
</script>
<template><div class="native-dialog-host">
 <p v-if="error" class="navigation-error" role="alert">{{error}}</p>
 <template v-if="state">
  <ConnectionDialog v-if="state.kind==='connection'" v-bind="state.payload" @save="formAction('save')" @cancel="formAction('cancel')" @test="formAction('test')" @password="formAction('password')" @forget="formAction('forget')" @choose-c-a="formAction('chooseCA')" @choose-ssh-key="formAction('chooseSSHKey')"/>
  <GroupDialog v-else-if="state.kind==='group'" v-bind="state.payload" @save="formAction('save')" @cancel="formAction('cancel')"/>
  <CellValueDialog v-else-if="state.kind==='cell-value'" v-bind="state.payload" @apply="cell=>action('apply',{cell})" @cancel="action('cancel')"/>
  <IndexFieldsDialog v-else-if="state.kind==='index-fields'" v-bind="state.payload" @apply="value=>action('apply',{value})" @cancel="action('cancel')"/>
  <TableNameDialog v-else-if="state.kind==='confirm'&&state.payload.namePrompt" :value="state.payload.value" @save="value=>action('accept',{value})" @cancel="action('cancel')"/>
  <ConfirmationDialog v-else-if="state.kind==='confirm'" v-bind="state.payload" @cancel="action('cancel')" @accept="action('accept')"/>
  <AboutDialog v-else-if="state.kind==='about'" @close="action('cancel')"/>
  <DatabaseTools v-else-if="state.kind==='database'" ref="tool" v-bind="state.payload" @close="action('cancel')" @changed="(operation,name)=>action('changed',{operation,name})"/>
 </template>
</div></template>
