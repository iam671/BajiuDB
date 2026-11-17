import {createApp,nextTick,type App} from 'vue'
import {beforeEach,afterEach,it,expect,vi} from 'vitest'
import NativeDialogHost from './NativeDialogHost.vue'
import {defaults,type WindowDialog} from './api'
let app:App,root:HTMLDivElement,send:ReturnType<typeof vi.fn>
async function settle(){for(let i=0;i<10;i++){await Promise.resolve();await nextTick()}}
function snapshot(revision:number,version:number,name:string):WindowDialog{return {id:'qa-native',kind:'connection',title:'QA',width:580,height:580,revision,payload:{model:{...defaults(),name},clientVersion:version,busy:'',errors:{},result:null,error:'',editing:false,connectOnSave:false,fatal:false,groups:[]}}}
beforeEach(()=>{vi.useFakeTimers();history.replaceState({},'', '/?dialog=qa-native');root=document.createElement('div');document.body.append(root);send=vi.fn(async()=>{});window.go={main:{App:{GetWindowDialog:vi.fn(async()=>snapshot(1,0,'')),SendWindowDialogAction:send} as never}}})
afterEach(()=>{app?.unmount();root.remove();delete window.go;history.replaceState({},'','/');document.documentElement.classList.remove('native-dialog-document');vi.useRealTimers()})
it('preserves local typing when an older parent update arrives and sends only changed fields',async()=>{
 app=createApp(NativeDialogHost);app.mount(root);await settle();const input=root.querySelector<HTMLInputElement>('#connection-name')!;input.value='QA draft';input.dispatchEvent(new Event('input',{bubbles:true}));await settle()
 window.dispatchEvent(new CustomEvent('bajiu:dialog-state',{detail:snapshot(2,0,'')}));await settle();expect(input.value).toBe('QA draft')
 await vi.advanceTimersByTimeAsync(50);await settle();expect(send).toHaveBeenLastCalledWith({id:'qa-native',action:'model',payload:{changes:{name:'QA draft'},clientVersion:1}})
 window.dispatchEvent(new CustomEvent('bajiu:dialog-state',{detail:snapshot(3,1,'QA draft')}));await settle();input.value='QA draft 2';input.dispatchEvent(new Event('input',{bubbles:true}));await settle();window.dispatchEvent(new CustomEvent('bajiu:dialog-state',{detail:snapshot(4,1,'QA draft')}));await settle();expect(input.value).toBe('QA draft 2')
})
it('forwards the native close request with the latest form instead of silently discarding it',async()=>{
 app=createApp(NativeDialogHost);app.mount(root);await settle();const input=root.querySelector<HTMLInputElement>('#connection-name')!;input.value='unsaved QA';input.dispatchEvent(new Event('input',{bubbles:true}));window.dispatchEvent(new CustomEvent('bajiu:dialog-close',{detail:'qa-native'}));await settle();expect(send).toHaveBeenLastCalledWith(expect.objectContaining({action:'cancel',payload:expect.objectContaining({model:expect.objectContaining({name:'unsaved QA'})})}))
})
