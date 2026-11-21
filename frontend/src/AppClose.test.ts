import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import {createApp,nextTick,type App as VueApp} from 'vue'
import App from './App.vue'
import {defaults} from './api'
vi.mock('./TableDesigner.vue',()=>({default:{emits:['dirty'],template:'<button @click="$emit(\'dirty\',true)">修改设计草稿</button>'}}))
let app:VueApp|undefined,root:HTMLDivElement,quit:ReturnType<typeof vi.fn>
async function settle(){for(let i=0;i<16;i++){await Promise.resolve();await nextTick()}}
function button(text:string){return Array.from(root.querySelectorAll<HTMLButtonElement>('button')).find(button=>button.textContent?.trim()===text)!}
beforeEach(()=>{
 localStorage.clear();quit=vi.fn(async()=>{});root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class {observe(){}disconnect(){}})
 const profile={...defaults(),id:'qa',name:'QA'}
 window.go={main:{App:{ListConnections:vi.fn(async()=>[profile]),ListGroups:vi.fn(async()=>[]),ListSessions:vi.fn(async()=>[{id:'qa',databases:['qa_db'],connected:true,connectedAt:'qa-session'}]),ConfigPath:vi.fn(async()=>'qa.json'),ListObjects:vi.fn(async()=>({objects:[],truncated:false})),ConfirmQuitApp:quit,QuitApp:vi.fn(async()=>{})} as never}}
})
afterEach(()=>{app?.unmount();root.remove();delete window.go;vi.unstubAllGlobals()})
async function mount(){app=createApp(App);app.mount(root);await settle()}
function requestClose(){window.dispatchEvent(new Event('bajiu:main-close'))}
it('closes immediately when there is no draft',async()=>{await mount();requestClose();await settle();expect(quit).toHaveBeenCalledTimes(1);expect(root.querySelector('[role=alertdialog]')).toBeNull()})
it('asks once for dirty designs, preserves changes on cancel and quits only after acceptance',async()=>{
 await mount();root.querySelector('.tree-connection')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();root.querySelector('.tree-database')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();button('新建表').click();await settle();button('修改设计草稿').click();await settle()
 requestClose();requestClose();await settle();expect(quit).not.toHaveBeenCalled();expect(root.querySelectorAll('[role=alertdialog]')).toHaveLength(1);button('取消').click();await settle();expect(button('修改设计草稿')).toBeDefined();expect(quit).not.toHaveBeenCalled()
 requestClose();await settle();button('放弃修改并退出').click();await settle();expect(quit).toHaveBeenCalledTimes(1)
})
it('includes connection-form edits in the close check',async()=>{
 await mount();root.querySelector<HTMLButtonElement>('[aria-label="新建连接"]')!.click();await settle();root.querySelector<HTMLButtonElement>('.connection-type-card[data-engine="mysql"]')!.click();await settle();const input=root.querySelector<HTMLInputElement>('#connection-name')!;input.value='unsaved connection';input.dispatchEvent(new Event('input',{bubbles:true}));await settle();requestClose();await settle();expect(quit).not.toHaveBeenCalled();button('取消').click();await settle();expect(input.value).toBe('unsaved connection')
})
it('includes query edits and routes the File exit action through the same check',async()=>{
 await mount();root.querySelector('.tree-connection')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();root.querySelector('.tree-database')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();button('新建查询').click();await settle();const input=root.querySelector<HTMLTextAreaElement>('.query-editor-input')!;input.value='SELECT 1';input.dispatchEvent(new Event('input',{bubbles:true}));await settle();button('文件(F)').click();await settle();button('退出').click();await settle();expect(quit).not.toHaveBeenCalled();expect(root.querySelector('[role=alertdialog]')?.textContent).toContain('未保存')
})
