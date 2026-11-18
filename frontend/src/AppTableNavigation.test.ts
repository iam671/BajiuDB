import {createApp,nextTick,type App} from 'vue'
import {afterEach,expect,it,vi} from 'vitest'
import Workspace from './App.vue'
import {defaults} from './api'
vi.mock('./TableDataEditor.vue',()=>({default:{props:['table','database'],emits:['design'],data:()=>({draft:'pending'}),template:'<section><input aria-label="记录草稿" v-model="draft"/><span>{{database}}.{{table}}</span><button @click="$emit(\'design\')">打开当前表设计</button></section>'}}))
vi.mock('./TableDesigner.vue',()=>({default:{props:['table','database'],template:'<section class="qa-design">设计 {{database}}.{{table}}</section>'}}))
let app:App|undefined,root:HTMLDivElement
async function settle(){for(let i=0;i<20;i++){await Promise.resolve();await nextTick()}}
const event=(type:string)=>new MouseEvent(type,{bubbles:true,cancelable:true})
afterEach(()=>{app?.unmount();root.remove();delete window.go;vi.unstubAllGlobals()})
it('retains the active data tab and draft on left/right tree selection, including another database, and designs the displayed table',async()=>{
 localStorage.clear();vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}});root=document.createElement('div');document.body.append(root)
 window.go={main:{App:{ListConnections:vi.fn(async()=>[{...defaults(),id:'qa',name:'QA'}]),ListGroups:vi.fn(async()=>[]),ListSessions:vi.fn(async()=>[{id:'qa',databases:['one','two'],connected:true,connectedAt:'qa'}]),ConfigPath:vi.fn(async()=>''),ListObjects:vi.fn(async(_id,db)=>({objects:[{name:db==='one'?'first':'second',kind:'tables'}],truncated:false}))} as never}}
 app=createApp(Workspace);app.mount(root);await settle();root.querySelector('.tree-connection')!.dispatchEvent(event('dblclick'));await settle()
 const databases=root.querySelectorAll('.tree-database');databases[0]!.dispatchEvent(event('dblclick'));await settle();databases[0]!.nextElementSibling!.querySelector('.tree-category')!.dispatchEvent(event('dblclick'));await settle();databases[1]!.dispatchEvent(event('dblclick'));await settle();databases[1]!.nextElementSibling!.querySelector('.tree-category')!.dispatchEvent(event('dblclick'));await settle()
 const table=(name:string)=>Array.from(root.querySelectorAll('.tree-table')).find(n=>n.textContent?.trim()===name)!
 table('first').dispatchEvent(event('click'));await settle();table('first').dispatchEvent(event('dblclick'));await settle();const active=root.querySelector('[role="tab"][aria-selected="true"]')!.id,input=root.querySelector<HTMLInputElement>('[aria-label="记录草稿"]')!;input.value='keep this draft';input.dispatchEvent(new Event('input',{bubbles:true}));await settle()
 for(const type of ['click','contextmenu']){table('second').dispatchEvent(event(type));expect(root.querySelector('[role="tab"][aria-selected="true"]')!.id).toBe(active);await settle();expect(root.querySelector('[role="tab"][aria-selected="true"]')!.id).toBe(active);expect(input.value).toBe('keep this draft');expect(input.closest('[role="tabpanel"]')?.getAttribute('style')??'').not.toContain('display: none')}
 expect(root.querySelector('[role="menu"]')?.textContent).toContain('设计表');(Array.from(root.querySelectorAll('button')).find(b=>b.textContent==='打开当前表设计') as HTMLButtonElement).click();await settle();expect(root.querySelector('.qa-design')?.textContent).toBe('设计 one.first')
})
