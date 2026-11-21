import {createApp,nextTick,type App} from 'vue'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import Workspace from './App.vue'
import Picker from './ConnectionTypePicker.vue'
import {databaseEngines,loadRecentEngines,rememberEngine,recentEnginesKey} from './databaseEngines'
import {defaults,installNativeBridge} from './api'
let app:App|undefined,root:HTMLDivElement
async function settle(){for(let i=0;i<20;i++){await nextTick();await Promise.resolve()}}
beforeEach(()=>{localStorage.clear();root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}})})
afterEach(()=>{app?.unmount();root.remove();delete window.go;installNativeBridge(undefined);vi.restoreAllMocks();vi.unstubAllGlobals()})
function allCard(engine:string){return root.querySelector<HTMLButtonElement>('[aria-labelledby="all-types-title"] [data-engine="'+engine+'"]')!}
function input(selector:string,value:string){const node=root.querySelector<HTMLInputElement>(selector)!;node.value=value;node.dispatchEvent(new Event('input',{bubbles:true}))}
function workspace(){
 const save=vi.fn(async()=>({...defaults(),id:'new'})),test=vi.fn(),connect=vi.fn()
 window.go={main:{App:{ListConnections:vi.fn(async()=>[]),ListGroups:vi.fn(async()=>[{id:'group',name:'Group'}]),ListSessions:vi.fn(async()=>[]),ConfigPath:vi.fn(async()=>''),SaveConnection:save,TestConnection:test,Connect:connect} as never}}
 app=createApp(Workspace);app.mount(root);return {save,test,connect}
}
it.each(databaseEngines)('selects $label before opening its fixed configuration without connecting early',async(engine)=>{
 const calls=workspace();await settle();root.querySelector<HTMLButtonElement>('[aria-label="新建连接"]')!.click();await settle()
 expect(root.querySelector('.connection-type-dialog')).not.toBeNull();expect(root.querySelector('.connection-dialog')).toBeNull();expect(root.querySelectorAll('[aria-labelledby="all-types-title"] [data-engine]')).toHaveLength(8)
 expect(allCard(engine.value).querySelector('svg')?.getAttribute('data-database-engine')).toBe(engine.value)
 allCard(engine.value).click();await settle()
 expect(root.querySelector('.connection-type-dialog')).toBeNull();expect(root.querySelector('#connection-engine')).toBeNull();expect(root.querySelector('#connection-dialog-title')?.textContent).toContain(engine.label)
 if(engine.value!=='sqlite')expect(root.querySelector<HTMLInputElement>('#connection-port')?.value).toBe(String(engine.port))
 else expect(root.querySelector('#connection-file')).not.toBeNull()
 expect(calls.save).not.toHaveBeenCalled();expect(calls.test).not.toHaveBeenCalled();expect(calls.connect).not.toHaveBeenCalled()
 expect(loadRecentEngines()[0]).toBe(engine.value)
})
it('filters types by search, activates a result with Enter and shows a clear empty state',async()=>{
 const choose=vi.fn();app=createApp(Picker,{recent:['mysql','redis'],onChoose:choose});app.mount(root);await settle()
 expect(root.querySelectorAll('.recent-types button')).toHaveLength(2);input('[aria-label="搜索数据库类型"]','post');await settle();expect(root.querySelectorAll('[data-engine]')).toHaveLength(1)
 root.querySelector('input')!.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true,cancelable:true}));await settle();expect(choose).toHaveBeenLastCalledWith('postgresql')
 input('input','no-such-engine');await settle();expect(root.querySelector('[role=status]')?.textContent).toContain('没有匹配');expect(root.querySelectorAll('[data-engine]')).toHaveLength(0)
})
it('cancels with Escape without opening settings or a discard confirmation and preserves group context on the next selection',async()=>{
 workspace();await settle();root.querySelector<HTMLButtonElement>('[aria-label="新建连接"]')!.click();await settle();document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));await settle();expect(root.querySelector('[role=dialog]')).toBeNull();expect(root.querySelector('[role=alertdialog]')).toBeNull()
 root.querySelector('.tree-group')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();Array.from(root.querySelectorAll('.connection-context-menu button')).find(b=>b.textContent==='新建连接…')!.dispatchEvent(new MouseEvent('click',{bubbles:true}));await settle();allCard('redis').click();await settle()
 expect(root.querySelector<HTMLSelectElement>('#connection-group')?.value).toBe('group')
})
it('stores only known recent engine IDs, tolerates invalid or unavailable preference storage and deduplicates seed types',()=>{
 localStorage.setItem(recentEnginesKey,JSON.stringify({version:1,engines:['redis','invalid','redis',null,'mysql']}));expect(loadRecentEngines(['sqlite','mysql'])).toEqual(['redis','mysql','sqlite'])
 rememberEngine('postgresql');expect(JSON.parse(localStorage.getItem(recentEnginesKey)!)).toEqual({version:1,engines:['postgresql','redis','mysql']})
 localStorage.setItem(recentEnginesKey,'bad json');expect(loadRecentEngines(['mongodb'])).toEqual(['mongodb'])
 vi.spyOn(Storage.prototype,'setItem').mockImplementation(()=>{throw new Error('blocked')});expect(()=>rememberEngine('oracle')).not.toThrow()
})
