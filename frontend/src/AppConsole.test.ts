import {createApp,nextTick,type App} from 'vue'
import {afterEach,expect,it,vi} from 'vitest'
import Workspace from './App.vue'
import {defaults} from './api'
let app:App|undefined,root:HTMLDivElement
async function settle(){for(let i=0;i<20;i++){await nextTick();await Promise.resolve()}}
const mouse=(type:string)=>new MouseEvent(type,{bubbles:true,cancelable:true})
afterEach(()=>{app?.unmount();root.remove();delete window.go;vi.unstubAllGlobals()})
it('opens independent closable console tabs from database menus and defaults each to that database',async()=>{
 vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}});localStorage.clear();root=document.createElement('div');document.body.append(root)
 const admin=vi.fn(async(_task,_id,db)=>({completed:1,error:'',text:'',results:[{columns:['DATABASE()'],rows:[[db]],affected:'0',truncated:false}]}))
 window.go={main:{App:{ListConnections:vi.fn(async()=>[{...defaults(),id:'qa',name:'QA'}]),ListGroups:vi.fn(async()=>[]),ListSessions:vi.fn(async()=>[{id:'qa',databases:['first_db','second_db'],connected:true,connectedAt:'qa'}]),ConfigPath:vi.fn(async()=>''),ListObjects:vi.fn(async()=>({objects:[],truncated:false})),DatabaseAdmin:admin,CancelDatabaseTask:vi.fn(async()=>{})} as never}}
 app=createApp(Workspace);app.mount(root);await settle();root.querySelector('.tree-connection')!.dispatchEvent(mouse('dblclick'));await settle()
 for(const db of ['first_db','second_db']){Array.from(root.querySelectorAll('.tree-database')).find(n=>n.textContent?.trim()===db)!.dispatchEvent(mouse('contextmenu'));await settle();(Array.from(root.querySelectorAll('[role="menu"] button')).find(b=>b.textContent?.trim()==='命令行界面') as HTMLButtonElement).click();await settle();expect(root.querySelector('.db-tools-window')).toBeNull();const panel=Array.from(root.querySelectorAll<HTMLElement>('.sql-console')).find(p=>p.style.display!=='none')!;expect(panel.textContent).toContain(db);const input=panel.querySelector('textarea')!;input.value='SELECT DATABASE();';input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true,cancelable:true}));await settle();expect(admin.mock.calls.at(-1)?.[2]).toBe(db)}
 expect(root.querySelectorAll('.sql-console')).toHaveLength(2);expect(root.querySelectorAll('.query-tab-close')).toHaveLength(2)
 const first=root.querySelectorAll<HTMLElement>('.sql-console')[0]!;expect(first.textContent).toContain('first_db');expect(first.textContent).not.toContain('second_db');expect(first.querySelectorAll('pre')).toHaveLength(2);
 (root.querySelectorAll('.query-tab-close')[1] as HTMLButtonElement).click();await settle();expect(root.querySelectorAll('.sql-console')).toHaveLength(1)
 const draft=root.querySelector<HTMLTextAreaElement>('.sql-console textarea')!;draft.value='SELECT pending';draft.dispatchEvent(new Event('input',{bubbles:true}));await settle();(root.querySelector('.query-tab-close') as HTMLButtonElement).click();await settle();expect(root.querySelector('[role=alertdialog]')?.textContent).toContain('尚未执行');(Array.from(root.querySelectorAll('button')).find(b=>b.textContent==='取消') as HTMLButtonElement).click();await settle();expect(draft.value).toBe('SELECT pending')
})
