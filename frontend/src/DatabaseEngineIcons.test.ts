import {createApp,h,nextTick,reactive,type App} from 'vue'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import Workspace from './App.vue'
import Icon from './DatabaseEngineIcon.vue'
import {databaseEngines} from './databaseEngines'
import {defaults,installNativeBridge} from './api'
let app:App|undefined,root:HTMLDivElement
async function settle(){for(let i=0;i<20;i++){await nextTick();await Promise.resolve()}}
beforeEach(()=>{localStorage.clear();root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}})})
afterEach(()=>{app?.unmount();root.remove();delete window.go;installNativeBridge(undefined);vi.unstubAllGlobals()})
it('keeps engine identity consistent across the connection tree, both list views, information and status',async()=>{
 const profiles=databaseEngines.map(e=>({...defaults(),engine:e.value,id:e.value,name:e.label}))
 const sessions=profiles.filter((_,i)=>i%2===0).map(p=>({id:p.id,connected:true,databases:[]}))
 window.go={main:{App:{ListConnections:vi.fn(async()=>profiles),ListGroups:vi.fn(async()=>[]),ListSessions:vi.fn(async()=>sessions),ConfigPath:vi.fn(async()=> '')} as never}}
 app=createApp(Workspace);app.mount(root);await settle()
 const icons=(selector:string)=>Array.from(root.querySelectorAll(selector)).map(el=>el.getAttribute('data-database-engine'))
 expect(icons('.tree-connection .database-engine-icon')).toEqual(profiles.map(p=>p.engine))
 expect(icons('.object-table .database-engine-icon')).toEqual(profiles.map(p=>p.engine))
 root.querySelector<HTMLButtonElement>('[aria-label="图标视图"]')!.click();await settle()
 expect(icons('.object-icon-grid .database-engine-icon')).toEqual(profiles.map(p=>p.engine))
 for(const [index,profile] of profiles.entries()){
  const row=root.querySelectorAll<HTMLButtonElement>('.tree-connection')[index]
  const state=index%2===0?'is-connected':'is-disconnected'
  expect(row.querySelector('svg.database-engine-icon')?.classList.contains(state)).toBe(true)
  row.click();await settle()
  expect(icons('.information-heading .database-engine-icon')).toEqual([profile.engine])
  expect(icons('.status-context .database-engine-icon')).toEqual([profile.engine])
 }
})
it('supports legacy MySQL, a safe unknown type and accessible live connection state',async()=>{
 const props=reactive<{engine?:string;state?:'connected'|'disconnected';labelled:boolean}>({labelled:false})
 app=createApp({render:()=>h(Icon,props)});app.mount(root);await settle()
 expect(root.querySelector('svg')?.getAttribute('data-database-engine')).toBe('mysql')
 expect(root.querySelector('svg')?.getAttribute('aria-hidden')).toBe('true')
 props.engine='REDIS';props.labelled=true;props.state='disconnected';await settle()
 expect(root.querySelector('[role=img]')?.getAttribute('aria-label')).toBe('Redis · 未连接')
 props.state='connected';await settle()
 expect(root.querySelector('[role=img]')?.getAttribute('aria-label')).toBe('Redis · 已连接')
 expect(root.querySelector('svg')?.getAttribute('data-database-engine')).toBe('redis')
 props.engine='future-engine';await settle()
 expect(root.querySelector('svg')?.getAttribute('data-database-engine')).toBe('unknown')
 expect(root.querySelector('[role=img]')?.getAttribute('aria-label')).toBe('数据库 · 已连接')
})
