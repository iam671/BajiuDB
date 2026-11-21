import {createApp,nextTick,reactive,type App} from 'vue'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import ConnectionDialog from './ConnectionDialog.vue'
import Workspace from './App.vue'
import QueryEditor from './QueryEditor.vue'
import {defaults,installNativeBridge} from './api'
import {databaseEngines,newConnectionFor} from './databaseEngines'
let app:App|undefined,root:HTMLDivElement
async function settle(){for(let i=0;i<20;i++){await nextTick();await Promise.resolve()}}
function button(text:string){const b=Array.from(root.querySelectorAll('button')).find(b=>b.textContent?.trim()===text);if(!b)throw Error(text);return b}
beforeEach(()=>{root=document.createElement('div');document.body.append(root);localStorage.clear();vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}})})
afterEach(()=>{app?.unmount();root.remove();delete window.go;installNativeBridge(undefined);vi.unstubAllGlobals()})
it.each(databaseEngines)('opens $label settings with fixed matching defaults and no type dropdown',async(engine)=>{
 const model=reactive(newConnectionFor(engine.value)!),pick=vi.fn(async()=> 'test-only.db');window.go={main:{App:{SelectSQLiteFile:pick} as never}}
 app=createApp(ConnectionDialog,{model,busy:'',errors:{},result:null,error:'',editing:false,connectOnSave:false,fatal:false,groups:[]});app.mount(root);await settle()
  expect(root.querySelector('#connection-engine')).toBeNull();expect(root.querySelector('h2')?.textContent).toContain(engine.label)
  expect(model.engine).toBe(engine.value);expect(model.port).toBe(engine.port);expect(model.username).toBe(engine.user);expect(model.database).toBe(engine.database)
  if(engine.value==='sqlite'){expect(root.querySelector('#connection-host')).toBeNull();root.querySelector<HTMLButtonElement>('[aria-label="选择 SQLite 文件"]')!.click();await settle();expect(model.filePath).toBe('test-only.db')}
  else expect(root.querySelector('#connection-host')).not.toBeNull()
  if(!['mysql','mariadb'].includes(engine.value))expect(Array.from(root.querySelectorAll('[role=tab]')).map(t=>t.textContent)).not.toContain('SSH')
 if(engine.value==='mongodb'){button('数据库').click();await settle();expect(root.querySelector('#mongo-authdb')).not.toBeNull();expect(model.authDatabase).toBe('admin')}
})
it.each(['sqlite','postgresql','sqlserver','oracle','redis','mongodb'])('browses %s with its native preview instead of MySQL table editing and schema menus',async(engine)=>{
 const query=engine==='redis'?'["GET","key"]':engine==='mongodb'?'{"find":"items"}':'SELECT * FROM items';const execute=vi.fn(async()=>({results:[],completed:1,error:'',text:''}))
 window.go={main:{App:{ListConnections:vi.fn(async()=>[{...defaults(),engine,id:'qa',name:'QA'}]),ListGroups:vi.fn(async()=>[]),ListSessions:vi.fn(async()=>[{id:'qa',databases:['test'],connected:true}]),ConfigPath:vi.fn(async()=>''),ListObjects:vi.fn(async()=>({objects:[{name:'items',kind:'tables',query}],truncated:false})),DatabaseAdmin:execute} as never}}
 app=createApp(Workspace);app.mount(root);await settle();root.querySelector('.tree-connection')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();root.querySelector('.tree-database')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()
 root.querySelector('.object-table tbody tr')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();expect(root.querySelector('.connection-context-menu')?.textContent).not.toContain('设计表')
 root.querySelector('.object-table tbody tr')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();expect(root.querySelector<HTMLTextAreaElement>('[aria-label="SQL 编辑器"]')?.value).toBe(query);expect(root.querySelector('.table-data-editor')).toBeNull()
 button('执行（选中或全部）').click();await settle();expect(execute).toHaveBeenCalledWith(expect.any(String),'qa','test',{operation:'query',sql:query});expect(root.querySelector('.query-editable-result')).toBeNull()
})
it('keeps a PostgreSQL SELECT on the raw-query route without probing MySQL metadata',async()=>{
 const execute=vi.fn(async()=>({results:[{columns:['id'],rows:[['9007199254740993123']],affected:'',truncated:false}],completed:1,error:'',text:''}));window.go={main:{App:{DatabaseAdmin:execute} as never}}
 app=createApp(QueryEditor,{profile:{...defaults(),id:'qa',engine:'postgresql'},database:'postgres',initialSQL:'SELECT id FROM users',active:true,connected:true});app.mount(root);await settle();button('执行（选中或全部）').click();await settle()
 expect(execute).toHaveBeenCalledTimes(1);expect(root.textContent).toContain('9007199254740993123');expect(root.querySelector('.query-editable-result')).toBeNull()
})
