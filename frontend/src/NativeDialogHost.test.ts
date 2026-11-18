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
it('shows detailed errors and read-only SQL in the native dialog with a single close action',async()=>{
 const state:WindowDialog={id:'qa-native',kind:'confirm',title:'保存表结构失败',width:760,height:520,revision:1,payload:{title:'保存表结构失败',body:'保存未完成',label:'关闭',notice:true,details:'Error 1064 (42000): syntax error at line 2',sql:'ALTER TABLE `items` INVALID;'}}
 window.go!.main.App.GetWindowDialog=vi.fn(async()=>state);app=createApp(NativeDialogHost);app.mount(root);await settle()
 expect(root.querySelector('pre')?.textContent).toContain('1064 (42000)');expect(root.querySelector('textarea')?.readOnly).toBe(true);expect(root.querySelector('textarea')?.value).toBe(state.payload.sql);expect(root.querySelectorAll('button')).toHaveLength(1)
 ;(root.querySelector('button') as HTMLButtonElement).click();await settle();expect(send).toHaveBeenLastCalledWith({id:'qa-native',action:'cancel',payload:{}})
})
it('returns native index selections and prompted names to the owning designer',async()=>{
 const state:WindowDialog={id:'qa-native',kind:'index-fields',title:'选择索引字段',width:480,height:440,revision:1,payload:{fields:[{name:'title',type:'varchar',length:'80'}],value:'title(12) DESC',kind:'NORMAL',method:'',sqlMode:''}}
 window.go!.main.App.GetWindowDialog=vi.fn(async()=>state);app=createApp(NativeDialogHost);app.mount(root);await settle()
 ;(Array.from(root.querySelectorAll('button')).find(button=>button.textContent==='确定') as HTMLButtonElement).click();await settle();expect(send).toHaveBeenLastCalledWith({id:'qa-native',action:'apply',payload:{value:'`title`(12) DESC'}})
 window.dispatchEvent(new CustomEvent('bajiu:dialog-state',{detail:{...state,kind:'confirm',revision:2,payload:{namePrompt:true,value:''}}}));await settle();const input=root.querySelector<HTMLInputElement>('#new-table-name')!;input.value='notes';input.dispatchEvent(new Event('input',{bubbles:true}));await settle();root.querySelector('form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await settle();expect(send).toHaveBeenLastCalledWith({id:'qa-native',action:'accept',payload:{value:'notes'}})
})
it('returns typed date choices from the native editor without writing a database request',async()=>{
 const state:WindowDialog={id:'qa-native',kind:'cell-value',title:'编辑日期',width:380,height:460,payload:{column:{name:'birthday',type:'date',nullable:true,key:'',defaultValue:null,extra:'',comment:'',generated:false,binary:false,spatial:false},cell:{mode:'value',value:'2026-11-18'},isNew:false}}
 window.go!.main.App.GetWindowDialog=vi.fn(async()=>state);app=createApp(NativeDialogHost);app.mount(root);await settle();root.querySelector<HTMLButtonElement>('[aria-label="2026-11-19"]')!.click();await settle();(Array.from(root.querySelectorAll('button')).find(button=>button.textContent==='确定') as HTMLButtonElement).click();await settle();expect(send).toHaveBeenLastCalledWith({id:'qa-native',action:'apply',payload:{cell:{mode:'value',value:'2026-11-19'}}})
})
it('shows every table name in the native bulk confirmation and cancels without accepting',async()=>{
 const state:WindowDialog={id:'qa-native',kind:'confirm',title:'删除选中的 2 个表？',width:480,height:420,payload:{title:'删除选中的 2 个表？',body:'数据库 qa_db',label:'删除表',items:['alpha','odd`table']}}
 window.go!.main.App.GetWindowDialog=vi.fn(async()=>state);app=createApp(NativeDialogHost);app.mount(root);await settle();expect(Array.from(root.querySelectorAll('.confirmation-items li')).map(n=>n.textContent)).toEqual(['alpha','odd`table']);(Array.from(root.querySelectorAll('button')).find(b=>b.textContent==='取消') as HTMLButtonElement).click();await settle();expect(send).toHaveBeenLastCalledWith({id:'qa-native',action:'cancel',payload:{}})
})
