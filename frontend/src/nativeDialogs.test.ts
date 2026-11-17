import {createApp,nextTick,type App} from 'vue'
import {afterEach,it,expect,vi} from 'vitest'
import Workspace from './App.vue'
import {installNativeBridge,type WindowDialog,type DialogAction} from './api'
let app:App|undefined,root:HTMLDivElement|undefined
async function settle(){for(let i=0;i<25;i++){await Promise.resolve();await nextTick()}}
function button(text:string,scope:ParentNode=root!){const found=Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find(b=>b.textContent?.trim()===text);if(!found)throw Error(text);return found}
afterEach(()=>{app?.unmount();root?.remove();app=undefined;root=undefined;installNativeBridge(undefined);vi.restoreAllMocks()})
it('creates native connection and confirmation windows, synchronises edits and closes the right windows',async()=>{
 const opened:WindowDialog[]=[],updated:WindowDialog[]=[],closed:string[]=[]
 installNativeBridge(async(name,...args)=>{switch(name.split('.').at(-1)){
  case 'ListConnections':case 'ListGroups':case 'ListSessions':return []
  case 'ConfigPath':return 'qa-only-config'
  case 'OpenWindowDialog':opened.push(JSON.parse(JSON.stringify(args[0])));return
  case 'UpdateWindowDialog':updated.push(JSON.parse(JSON.stringify(args[0])));return
  case 'CloseWindowDialog':closed.push(String(args[0]));return
  default:throw Error('Unexpected call '+name)
 }})
 root=document.createElement('div');document.body.append(root);app=createApp(Workspace);app.mount(root);await settle();root.querySelector<HTMLButtonElement>('[aria-label="新建连接"]')!.click();await settle();expect(opened).toHaveLength(1);expect(opened[0].kind).toBe('connection');expect(root.querySelector('.connection-dialog')).toBeNull()
 const action=(data:DialogAction)=>window.dispatchEvent(new CustomEvent('bajiu:dialog-action',{detail:data}))
 action({id:opened[0].id,action:'model',payload:{changes:{name:'native QA'},clientVersion:1}});await settle();expect(updated.at(-1)?.payload.model.name).toBe('native QA')
 action({id:opened[0].id,action:'cancel',payload:{clientVersion:1}});await settle();expect(opened[1].kind).toBe('confirm');expect(root.querySelector('[role="alertdialog"]')).toBeNull()
 action({id:opened[1].id,action:'cancel',payload:{}});await settle();expect(closed).toContain(opened[1].id);expect(closed).not.toContain(opened[0].id)
 action({id:opened[0].id,action:'cancel',payload:{clientVersion:1}});await settle();expect(opened[2].id).not.toBe(opened[1].id);action({id:opened[2].id,action:'accept',payload:{}});await settle();expect(closed).toContain(opened[0].id);expect(closed).toContain(opened[2].id)
 button('帮助(H)').click();await settle();button('关于 Bajiu DB…',root.querySelector('.menu-popup')!).click();await settle();expect(opened.at(-1)?.kind).toBe('about')
})
