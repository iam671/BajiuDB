import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, nextTick, type App as VueApp } from 'vue'
import App from './App.vue'
import { defaults, type Profile, type ConnectionGroup } from './api'

let app:VueApp, root:HTMLDivElement
let records:Profile[]
let catalog:ConnectionGroup[]
let bridge:Record<string,ReturnType<typeof vi.fn>>
const savedProfile:Profile={...defaults(),id:'test-profile',revision:1,name:'开发服务器',hasPassword:true,createdAt:'2026-11-15T00:00:00Z',updatedAt:'2026-11-15T00:00:00Z'}
async function settle(){for(let i=0;i<5;i++){await Promise.resolve();await nextTick()}}
function button(text:string,scope:ParentNode=root){const found=Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find(b=>b.textContent?.trim()===text);if(!found)throw new Error('Button not found: '+text);return found}
async function fill(selector:string,value:string){const input=root.querySelector<HTMLInputElement>(selector);if(!input)throw new Error('Input not found: '+selector);input.value=value;input.dispatchEvent(new Event('input',{bubbles:true}));await settle()}
async function key(selector:string,key:string){root.querySelector(selector)!.dispatchEvent(new KeyboardEvent('keydown',{key,bubbles:true,cancelable:true}));await settle()}
async function mount(){app=createApp(App);app.mount(root);await settle()}
function createDialog(){root.querySelector<HTMLButtonElement>('[aria-label="新建连接"]')!.click()}
function dragData(){const values=new Map<string,string>();return {effectAllowed:'none',dropEffect:'none',get types(){return [...values.keys()]},setData(type:string,value:string){values.set(type,value)},getData(type:string){return values.get(type)??''}}}
function drag(element:Element,type:string,data:ReturnType<typeof dragData>){const event=new Event(type,{bubbles:true,cancelable:true});Object.defineProperty(event,'dataTransfer',{value:data});element.dispatchEvent(event);return event}
async function doubleClick(element:HTMLElement){element.click();await settle();element.click();element.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()}

beforeEach(()=>{
 localStorage.clear();records=[];catalog=[]
 bridge={ListConnections:vi.fn(async()=>records),ListSessions:vi.fn(async()=>[]),ConfigPath:vi.fn(async()=>'test-config.json'),SaveConnection:vi.fn(async(input)=>{const p={...savedProfile,...input,id:'test-profile',revision:1};for(const field of ['password','sshPassword','sshKeyPassphrase','httpToken','passwordAction','sshPasswordAction','sshKeyPassphraseAction','httpTokenAction','rememberPassword'])delete p[field];records=[p];return p}),DeleteConnection:vi.fn(async()=>{}),TestConnection:vi.fn(async()=>({success:true,message:'ok',serverVersion:'8.0-test',latencyMs:1})),Connect:vi.fn(async()=>({id:'test-profile',connected:true,serverVersion:'8.0-test',currentUser:'test@localhost',databases:['test_db'],latencyMs:1,connectedAt:'2026-11-16T00:00:00Z'})),Disconnect:vi.fn(async()=>{}),RefreshDatabases:vi.fn(),ListObjects:vi.fn(async()=>({objects:[{name:'customers',kind:'tables',engine:'InnoDB',rows:'8',dataLength:'16384',indexLength:'0',autoIncrement:'9',comment:'客户',createdAt:'',modifiedAt:''}],truncated:false})),SelectCAFile:vi.fn(async()=>'test-ca.pem'),QuitApp:vi.fn()}
 window.go={main:{App:bridge as never}}
 bridge.ListGroups=vi.fn(async()=>catalog)
 bridge.SaveGroup=vi.fn(async(id,name)=>{const group={id:id||'group-1',name:name.trim()};catalog=id?catalog.map(g=>g.id===id?group:g):[...catalog,group];return group})
 bridge.MoveConnectionToGroup=vi.fn(async(id,revision,groupId)=>{const profile={...records.find(p=>p.id===id)!,groupId,revision:revision+1};records=records.map(p=>p.id===id?profile:p);return profile})
 bridge.DisconnectAll=vi.fn(async()=>{})
 bridge.DeleteGroup=vi.fn(async(id)=>{catalog=catalog.filter(g=>g.id!==id);records=records.map(p=>p.groupId===id?{...p,groupId:'',revision:p.revision+1}:p)})
 bridge.TableDDL=vi.fn(async()=>'CREATE TABLE `customers` (`id` bigint NOT NULL) ENGINE=InnoDB')
 bridge.SelectSSHKey=vi.fn(async()=>'qa-test-private-key')
 root=document.createElement('div');document.body.append(root)
})
afterEach(()=>{app?.unmount();root.remove();delete window.go;vi.restoreAllMocks();vi.unstubAllGlobals()})

describe('desktop workspace and connection dialog',()=>{
 it('opens databases only on double click and offers functional database menus in the tree and list',async()=>{
  records=[savedProfile];await mount();await doubleClick(root.querySelector('.tree-connection')!);const database=root.querySelector<HTMLButtonElement>('.tree-database')!
  database.click();await settle();expect(bridge.ListObjects).not.toHaveBeenCalled();expect(database.querySelector('.database-open')).toBeNull()
  database.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();let menu=root.querySelector('[role="menu"]')!;expect(Array.from(menu.querySelectorAll('button')).map(b=>b.textContent?.trim())).toEqual(['打开数据库','编辑数据库','新建数据库','删除数据库','新建查询','命令行界面','运行SQL文件','转储SQL文件','数据字典','逆向数据库到模型','刷新']);button('打开数据库',menu).click();await settle();expect(bridge.ListObjects).toHaveBeenCalledOnce();expect(database.querySelector('.database-open')).not.toBeNull()
  database.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();menu=root.querySelector('[role="menu"]')!;button('关闭数据库',menu).click();await settle();expect(database.querySelector('.database-open')).toBeNull();expect(database.hasAttribute('aria-expanded')).toBe(false);expect(bridge.Disconnect).not.toHaveBeenCalled()
  root.querySelector('.object-table tbody tr')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();expect(root.querySelector('[role="menu"]')?.textContent).toContain('打开数据库')
 })
 it('shows one connection action and toggles expansion on double click without reconnecting',async()=>{
  records=[savedProfile];await mount();let connection=root.querySelector('.tree-connection')!
  connection.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();let menu=root.querySelector('[role="menu"]')!
  expect(menu.textContent).toContain('打开连接');expect(menu.textContent).not.toContain('关闭连接');menu.querySelector<HTMLButtonElement>('button')!.click();await settle()
  connection=root.querySelector('.tree-connection')!;connection.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();menu=root.querySelector('[role="menu"]')!;expect(menu.textContent).toContain('关闭连接');expect(menu.textContent).not.toContain('打开连接');expect(menu.textContent).not.toContain('展开连接')
  connection.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();expect(root.querySelector('.tree-database')).toBeNull();connection.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();expect(root.querySelector('.tree-database')).not.toBeNull();expect(bridge.Connect).toHaveBeenCalledOnce()
 })
 it('exposes DDL on table selection and switches tabs only when explicitly requested',async()=>{
  records=[savedProfile];await mount();root.querySelector('.tree-connection')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()
  root.querySelector<HTMLButtonElement>('.tree-database')!.click();await settle();expect(bridge.ListObjects).not.toHaveBeenCalled();expect(root.querySelector('.object-table tbody')?.textContent).not.toContain('customers');expect(root.querySelector('.tree-database.selected')).not.toBeNull();expect(root.querySelector('.information-details')?.textContent).toContain('test_db');expect(root.querySelector('.information-tabs')?.textContent).not.toContain('DDL');expect(root.querySelector('.tree-database')?.hasAttribute('aria-expanded')).toBe(false)
  root.querySelector('.tree-database')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();root.querySelector<HTMLTableRowElement>('.object-table tbody tr')!.click();await settle();expect(bridge.TableDDL).not.toHaveBeenCalled();expect(root.querySelector('.information-tabs')?.textContent).toContain('DDL');expect(root.querySelector('.information-tabs .active')?.textContent).toBe('信息')
  await doubleClick(root.querySelector('.object-table tbody tr')!);expect(bridge.TableDDL).not.toHaveBeenCalled();expect(root.querySelector('.ddl-panel')).toBeNull();button('DDL',root.querySelector('.information-tabs')!).click();await settle();expect(bridge.TableDDL).toHaveBeenCalledWith('test-profile','test_db','customers');expect(root.querySelector('.ddl-panel pre')?.textContent).toContain('CREATE TABLE')
  button('信息',root.querySelector('.information-tabs')!).click();await settle();await doubleClick(root.querySelector('.object-table tbody tr')!);expect(root.querySelector('.information-tabs .active')?.textContent).toBe('信息');expect(bridge.TableDDL).toHaveBeenCalledTimes(1)
  root.querySelector<HTMLButtonElement>('.tree-database')!.click();await settle();expect(root.querySelector('.information-tabs')?.textContent).not.toContain('DDL');expect(root.querySelector('.ddl-panel')).toBeNull()
 })
 it('shows a completion SVG only after a successful DDL copy and supports retrying clipboard failures',async()=>{
  const writeText=vi.fn().mockRejectedValueOnce(new Error('denied')).mockResolvedValue(undefined);vi.stubGlobal('navigator',{clipboard:{writeText}})
  records=[savedProfile];await mount();await doubleClick(root.querySelector('.tree-connection')!);await doubleClick(root.querySelector('.tree-database')!);root.querySelector<HTMLTableRowElement>('.object-table tbody tr')!.click();await settle();button('DDL',root.querySelector('.information-tabs')!).click();await settle()
  root.querySelector<HTMLButtonElement>('.ddl-copy-button')!.click();await settle();expect(root.querySelector('.ddl-copy-complete')).toBeNull();expect(root.querySelector('.ddl-panel [role="alert"]')?.textContent).toContain('复制失败')
  root.querySelector<HTMLButtonElement>('.ddl-copy-button')!.click();await settle();expect(writeText).toHaveBeenLastCalledWith('CREATE TABLE `customers` (`id` bigint NOT NULL) ENGINE=InnoDB');expect(root.querySelector('.ddl-copy-button svg.ddl-copy-complete')).not.toBeNull();expect(root.querySelector('.ddl-copy-status')?.textContent).toBe('已复制表定义');expect(root.querySelector('.ddl-panel [role="alert"]')).toBeNull()
  const completed=root.querySelector('.ddl-copy-complete');root.querySelector<HTMLButtonElement>('.ddl-copy-button')!.click();await settle();expect(root.querySelector('.ddl-copy-complete')).not.toBe(completed)
  let copied:()=>void=()=>{};writeText.mockImplementationOnce(()=>new Promise<void>(resolve=>{copied=resolve}));root.querySelector<HTMLButtonElement>('.ddl-copy-button')!.click();await settle();root.querySelector<HTMLButtonElement>('.tree-database')!.click();await settle();copied();await settle();expect(root.querySelector('.ddl-copy-complete')).toBeNull()
 })
 it('retains the manually selected DDL tab when selecting another table and loads that table definition',async()=>{
  bridge.ListObjects.mockResolvedValue({objects:['first_table','second_table'].map(name=>({name,kind:'tables',engine:'InnoDB'})),truncated:false});bridge.TableDDL.mockImplementation(async(_id,_db,table)=>'CREATE TABLE `'+table+'` (`id` bigint)')
  records=[savedProfile];await mount();await doubleClick(root.querySelector('.tree-connection')!);await doubleClick(root.querySelector('.tree-database')!);let rows=root.querySelectorAll<HTMLTableRowElement>('.object-table tbody tr');rows[0].click();await settle();button('DDL',root.querySelector('.information-tabs')!).click();await settle();rows[1].click();await settle();expect(root.querySelector('.information-tabs .active')?.textContent).toBe('DDL');expect(root.querySelector('.ddl-panel pre')?.textContent).toContain('second_table');expect(bridge.TableDDL).toHaveBeenLastCalledWith('test-profile','test_db','second_table')
 })
 it('toggles database expansion on real double-click sequences without reloading or reopening a collapsed node',async()=>{
  records=[savedProfile];await mount();await doubleClick(root.querySelector('.tree-connection')!)
  const database=root.querySelector<HTMLButtonElement>('.tree-database')!
  await doubleClick(database);expect(database.getAttribute('aria-expanded')).toBe('true');expect(root.querySelectorAll('.tree-category')).toHaveLength(4);expect(bridge.ListObjects).toHaveBeenCalledTimes(1)
  await doubleClick(database);expect(database.getAttribute('aria-expanded')).toBe('false');expect(root.querySelector('.tree-category')).toBeNull();expect(root.querySelector('.object-table tbody')?.textContent).toContain('customers');expect(bridge.ListObjects).toHaveBeenCalledTimes(1)
  button('刷新').click();await settle();expect(database.getAttribute('aria-expanded')).toBe('false')
  await doubleClick(database);expect(database.getAttribute('aria-expanded')).toBe('true');expect(bridge.ListObjects).toHaveBeenCalledTimes(2)
 })
 it('returns from a database to its connection and ignores any table response still in flight',async()=>{
  records=[savedProfile,{...savedProfile,id:'other-profile',name:'另一个连接'}];bridge.ListSessions.mockResolvedValue(records.map(p=>({id:p.id,connected:true,databases:['test_db'],serverVersion:'test',currentUser:'test',latencyMs:1,connectedAt:''})))
  await mount();const connection=root.querySelector<HTMLButtonElement>('.tree-connection')!;await doubleClick(connection)
  root.querySelector<HTMLButtonElement>('.tree-database')!.click();await settle();connection.click();await settle();expect(connection.classList.contains('selected')).toBe(true);expect(root.querySelector('.object-table tbody')?.textContent).toContain('test_db');expect(root.querySelector('.object-table tbody')?.textContent).not.toContain('customers');expect(root.querySelector('.information-heading')?.textContent).toContain('开发服务器')
  let release:(result:unknown)=>void=()=>{};bridge.ListObjects.mockImplementationOnce(()=>new Promise(resolve=>{release=resolve}))
  root.querySelector<HTMLButtonElement>('.tree-database')!.click();await settle();connection.click();await settle();release({objects:[{name:'stale_table',kind:'tables'}],truncated:false});await settle();expect(connection.classList.contains('selected')).toBe(true);expect(root.querySelector('.object-table tbody')?.textContent).toContain('test_db');expect(root.querySelector('.object-table tbody')?.textContent).not.toContain('stale_table')
  root.querySelectorAll<HTMLButtonElement>('.tree-connection')[1].click();await settle();expect(root.querySelector('.information-heading')?.textContent).toContain('另一个连接');expect(bridge.Disconnect).not.toHaveBeenCalled()
 })
 it('keeps a database collapsed when an earlier table request finishes after a second double click',async()=>{
  records=[savedProfile];let release:(result:unknown)=>void=()=>{};bridge.ListObjects.mockImplementationOnce(()=>new Promise(resolve=>{release=resolve}))
  await mount();await doubleClick(root.querySelector('.tree-connection')!);const database=root.querySelector<HTMLButtonElement>('.tree-database')!
  await doubleClick(database);await doubleClick(database);expect(bridge.ListObjects).toHaveBeenCalledTimes(1)
  release({objects:[{name:'loaded_table',kind:'tables'}],truncated:false});await settle();expect(database.getAttribute('aria-expanded')).toBe('false');expect(root.querySelector('.tree-category')).toBeNull();expect(root.querySelector('.object-table tbody')?.textContent).toContain('loaded_table')
 })
 it('enables SSH and PHP tunnel settings while keeping their credentials out of profile results',async()=>{
  await mount();createDialog();await settle();await fill('#connection-name','隧道测试');button('SSH',root.querySelector('.connection-tabbar')!).click();await settle();const ssh=root.querySelector<HTMLInputElement>('#connection-panel-ssh .native-check input')!;ssh.checked=true;ssh.dispatchEvent(new Event('change',{bubbles:true}));await settle();await fill('#ssh-host','ssh.example.test');await fill('#ssh-user','qa');await fill('#ssh-password','qa-ssh-password')
  const auth=root.querySelector<HTMLSelectElement>('#ssh-auth')!;auth.value='key';auth.dispatchEvent(new Event('change',{bubbles:true}));await settle();root.querySelector<HTMLButtonElement>('[aria-label="选择 SSH 私钥"]')!.click();await settle();expect(bridge.SelectSSHKey).toHaveBeenCalledOnce();expect((root.querySelector('#ssh-key') as HTMLInputElement).value).toBe('qa-test-private-key')
  button('HTTP',root.querySelector('.connection-tabbar')!).click();await settle();const http=root.querySelector<HTMLInputElement>('#connection-panel-http .native-check input')!;http.checked=true;http.dispatchEvent(new Event('change',{bubbles:true}));await settle();await fill('#http-url','https://example.test/bajiu-mysql.php');await fill('#http-token','qa-http-token');button('确定',root.querySelector('.connection-dialog')!).click();await settle()
  const request=bridge.SaveConnection.mock.calls[0][0];expect(request.tunnelMode).toBe('http');expect(request.httpToken).toBe('qa-http-token');expect(request.sshPassword).toBe('qa-ssh-password');expect(request.httpTokenAction).toBe('replace');expect(records[0]).not.toHaveProperty('httpToken')
 })
 it('drags the connection dialog beyond the viewport and recenters it on title double click',async()=>{
  await mount();createDialog();await settle();const handle=root.querySelector('.connection-drag-handle')!
  const pointer=(type:string,x:number,y:number)=>{const event=new MouseEvent(type,{bubbles:true,cancelable:true,button:0,clientX:x,clientY:y});Object.defineProperty(event,'pointerId',{value:1});return event}
  handle.dispatchEvent(pointer('pointerdown',100,100));document.dispatchEvent(pointer('pointermove',-900,800));document.dispatchEvent(pointer('pointerup',-900,800));await settle();expect((root.querySelector('.connection-dialog') as HTMLElement).style.transform).toBe('translate(-1000px,700px)')
  handle.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();expect((root.querySelector('.connection-dialog') as HTMLElement).style.transform).toBe('translate(0px,0px)')
 })
 it('drags a connection into a group and back to the root while keeping its database view and session',async()=>{
  catalog=[{id:'group-1',name:'项目组'}];records=[savedProfile];await mount()
  root.querySelector('.tree-connection')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();root.querySelector<HTMLButtonElement>('.tree-database')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()
  const data=dragData();drag(root.querySelector('.tree-connection')!,'dragstart',data)
  expect(data.effectAllowed).toBe('move');expect(drag(root.querySelector('.tree-group')!,'dragover',data).defaultPrevented).toBe(true);await settle();expect(root.querySelector('.tree-group.drop-target')).not.toBeNull()
  drag(root.querySelector('.tree-group')!,'drop',data);await settle()
  expect(bridge.MoveConnectionToGroup).toHaveBeenLastCalledWith('test-profile',1,'group-1');expect(root.querySelector('.group-members .tree-connection.connected')).not.toBeNull();expect(root.querySelector('.object-table tbody')?.textContent).toContain('customers')
  const out=dragData();drag(root.querySelector('.tree-connection')!,'dragstart',out);await settle();expect(root.querySelector('.tree-root-drop-zone')).not.toBeNull();drag(root.querySelector('.navigation-scroll')!,'dragover',out);drag(root.querySelector('.navigation-scroll')!,'drop',out);await settle()
  expect(bridge.MoveConnectionToGroup).toHaveBeenLastCalledWith('test-profile',2,'');expect(root.querySelector('.group-members .tree-connection')).toBeNull();expect(root.querySelector('.tree-connection.connected')).not.toBeNull();expect(root.querySelector('.object-table tbody')?.textContent).toContain('customers');expect(bridge.Disconnect).not.toHaveBeenCalled();expect(root.querySelector('.workspace-notice')).toBeNull()
 })
 it('rejects same-group and external drops, and clears drag state on cancellation',async()=>{
  catalog=[{id:'group-1',name:'项目组'}];records=[{...savedProfile,groupId:'group-1'}];await mount()
  const same=dragData();drag(root.querySelector('.tree-connection')!,'dragstart',same);expect(drag(root.querySelector('.tree-group')!,'dragover',same).defaultPrevented).toBe(false);drag(root.querySelector('.tree-group')!,'drop',same);await settle();expect(bridge.MoveConnectionToGroup).not.toHaveBeenCalled()
  const cancelled=dragData();drag(root.querySelector('.tree-connection')!,'dragstart',cancelled);await settle();expect(root.querySelector('.tree-root-drop-zone')).not.toBeNull();drag(root.querySelector('.tree-connection')!,'dragend',cancelled);await settle();expect(root.querySelector('.tree-root-drop-zone')).toBeNull()
  const external=dragData();external.setData('application/x-bajiu-connection','test-profile');drag(root.querySelector('.tree-root')!,'drop',external);await settle();expect(bridge.MoveConnectionToGroup).not.toHaveBeenCalled();expect(root.querySelector('.group-members .tree-connection')).not.toBeNull()
 })
 it('retains placement after a failed drag save and reports errors inline without a toast',async()=>{
  catalog=[{id:'group-1',name:'项目组'}];records=[savedProfile];bridge.MoveConnectionToGroup.mockRejectedValueOnce(new Error('无法保存分组归属'));await mount()
  const data=dragData();drag(root.querySelector('.tree-connection')!,'dragstart',data);drag(root.querySelector('.tree-group')!,'drop',data);await settle()
  expect(root.querySelector('.navigation-error')?.textContent).toContain('无法保存分组归属');expect(root.querySelector('.group-members .tree-connection')).toBeNull();expect(root.querySelector('.tree-connection')?.textContent).toContain('开发服务器');expect(root.querySelector('.workspace-notice')).toBeNull();expect(root.querySelector('.tree-root-drop-zone')).toBeNull()
  const retry=dragData();drag(root.querySelector('.tree-connection')!,'dragstart',retry);drag(root.querySelector('.tree-group')!,'drop',retry);await settle();expect(root.querySelector('.group-members .tree-connection')).not.toBeNull();expect(root.querySelector('.navigation-error')).toBeNull();expect(root.querySelector('.workspace-notice')).toBeNull()
 })
 it('creates a persistent group from the root context menu and moves a connection into it',async()=>{
  records=[savedProfile];await mount()
  root.querySelector('.tree-root')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:20,clientY:120}));await settle()
  const menu=root.querySelector('[role="menu"]')!
  expect(menu.textContent).toContain('新建连接');expect(menu.textContent).toContain('新建组');expect(menu.textContent).toContain('关闭所有连接')
  button('新建组',menu).click();await settle();expect(root.querySelector('[role="dialog"]')).toBeNull();expect(root.querySelector('.navigation-pane .inline-group-name')).not.toBeNull();await fill('.inline-group-name','项目 A');await key('.inline-group-name','Enter')
  expect(bridge.SaveGroup).toHaveBeenCalledWith('','项目 A');expect(root.querySelector('.tree-group')?.textContent).toContain('项目 A')
  root.querySelector('.tree-connection')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();button('移至分组…',root.querySelector('[role="menu"]')!).click();await settle()
  const target=root.querySelector<HTMLSelectElement>('#group-dialog-value')!;target.value='group-1';target.dispatchEvent(new Event('change',{bubbles:true}));await settle();button('确定',root.querySelector('.group-dialog')!).click();await settle()
  expect(bridge.MoveConnectionToGroup).toHaveBeenCalledWith('test-profile',1,'group-1');expect(root.querySelector('.group-members .tree-connection')?.textContent).toContain('开发服务器')
  app.unmount();await mount();expect(root.querySelector('.tree-group')?.textContent).toContain('项目 A');expect(root.querySelector('.group-members .tree-connection')).not.toBeNull()
  root.querySelector('.tree-group')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();button('新建连接…',root.querySelector('[role="menu"]')!).click();await settle();await fill('#connection-name','组内新连接');button('确定',root.querySelector('.connection-dialog')!).click();await settle()
  expect(bridge.SaveConnection.mock.calls[0][0].groupId).toBe('group-1')
 })
 it('keeps inline group editing active when persistence fails',async()=>{
  bridge.SaveGroup.mockRejectedValue(new Error('无法写入连接配置'))
  await mount();root.querySelector('.tree-root')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();button('新建组',root.querySelector('[role="menu"]')!).click();await settle();await fill('.inline-group-name','新分组');await key('.inline-group-name','Enter')
  expect(root.querySelector('.inline-group-error')?.textContent).toContain('无法写入');expect(root.querySelector('.tree-group')).toBeNull();expect((root.querySelector('.inline-group-name') as HTMLInputElement).value).toBe('新分组');expect(root.querySelector('[role="dialog"]')).toBeNull()
 })
 it('cancels inline creation with Escape and renames an existing group in place',async()=>{
  catalog=[{id:'group-1',name:'旧组'}];await mount()
  root.querySelector('.tree-root')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();button('新建组',root.querySelector('[role="menu"]')!).click();await settle();await fill('.inline-group-name','不保存');await key('.inline-group-name','Escape')
  expect(root.querySelector('.inline-group-name')).toBeNull();expect(bridge.SaveGroup).not.toHaveBeenCalled()
  root.querySelector('.tree-group')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();button('重命名组',root.querySelector('[role="menu"]')!).click();await settle()
  expect((root.querySelector('.inline-group-name') as HTMLInputElement).value).toBe('旧组');await fill('.inline-group-name','新组');await key('.inline-group-name','Enter')
  expect(bridge.SaveGroup).toHaveBeenCalledWith('group-1','新组');expect(root.querySelector('.tree-group')?.textContent).toContain('新组')
 })
 it('deletes a group after confirmation while keeping its connections and live session',async()=>{
  catalog=[{id:'group-1',name:'项目组'}];records=[{...savedProfile,groupId:'group-1'}];bridge.ListSessions.mockResolvedValue([{id:'test-profile',connected:true,databases:['test_db'],serverVersion:'test',currentUser:'test',latencyMs:1,connectedAt:''}]);await mount()
  root.querySelector('.tree-group')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();button('删除组',root.querySelector('[role="menu"]')!).click();await settle()
  expect(root.querySelector('[role="alertdialog"]')?.textContent).toContain('移回');expect(bridge.DeleteGroup).not.toHaveBeenCalled();button('取消',root.querySelector('[role="alertdialog"]')!).click();await settle();expect(root.querySelector('.tree-group')).not.toBeNull()
  root.querySelector('.tree-group')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();button('删除组',root.querySelector('[role="menu"]')!).click();await settle();button('删除组',root.querySelector('[role="alertdialog"]')!).click();await settle()
  expect(bridge.DeleteGroup).toHaveBeenCalledWith('group-1');expect(root.querySelector('.tree-group')).toBeNull();expect(root.querySelector('.tree-connection.connected')?.textContent).toContain('开发服务器');expect(root.querySelector('.group-members .tree-connection')).toBeNull();expect(bridge.DeleteConnection).not.toHaveBeenCalled();expect(bridge.Disconnect).not.toHaveBeenCalled()
 })
 it('closes every session from the root menu without deleting saved connections',async()=>{
  records=[savedProfile,{...savedProfile,id:'another',name:'另一个连接'}]
  bridge.ListSessions.mockResolvedValue(records.map(p=>({id:p.id,connected:true,databases:['test_db'],serverVersion:'test',currentUser:'test',latencyMs:1,connectedAt:''})))
  await mount();root.querySelector('.tree-root')!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();button('关闭所有连接',root.querySelector('[role="menu"]')!).click();await settle()
  expect(bridge.DisconnectAll).toHaveBeenCalledOnce();expect(root.querySelectorAll('.tree-connection.connected')).toHaveLength(0);expect(root.querySelectorAll('.tree-connection')).toHaveLength(2);expect(bridge.DeleteConnection).not.toHaveBeenCalled();expect(bridge.SaveConnection).not.toHaveBeenCalled()
 })
 it('creates a modal and saves without sending output timestamps through the Go bridge',async()=>{
  await mount();createDialog();await settle()
  expect(root.querySelector('[role="dialog"]')).not.toBeNull();expect(root.querySelector('.desktop-workspace')?.hasAttribute('inert')).toBe(true)
  await fill('#connection-name','我的数据库');button('确定',root.querySelector('[role="dialog"]')!).click();await settle()
  expect(bridge.SaveConnection).toHaveBeenCalledOnce();const request=bridge.SaveConnection.mock.calls[0][0]
  expect(request).not.toHaveProperty('createdAt');expect(request).not.toHaveProperty('updatedAt');expect(request.host).toBe('127.0.0.1')
  expect(root.querySelector('.tree-connection')?.textContent).toContain('我的数据库');expect(root.querySelector('[role="dialog"]')).toBeNull();expect(root.querySelector('.desktop-workspace')?.hasAttribute('inert')).toBe(false)
 })
 it('asks before discarding modal edits and permits keeping the draft',async()=>{
  await mount();createDialog();await settle();await fill('#connection-name','还未保存');button('取消',root.querySelector('[role="dialog"]')!).click();await settle()
  const alert=root.querySelector('[role="alertdialog"]')!;expect(alert).not.toBeNull();button('取消',alert).click();await settle();expect((root.querySelector('#connection-name') as HTMLInputElement).value).toBe('还未保存')
  button('取消',root.querySelector('[role="dialog"]')!).click();await settle();button('放弃修改',root.querySelector('[role="alertdialog"]')!).click();await settle()
  expect(root.querySelector('[role="dialog"]')).toBeNull();expect(bridge.SaveConnection).not.toHaveBeenCalled()
 })
 it('duplicates settings without copying the saved password or record identity',async()=>{
  records=[savedProfile];await mount();root.querySelector<HTMLButtonElement>('.tree-connection')!.click();await settle()
  root.querySelector<HTMLButtonElement>('.application-menu>button')!.click();await settle();button('复制连接…',root.querySelector('.menu-popup')!).click();await settle()
  const password=root.querySelector<HTMLInputElement>('#connection-password')!;expect(password.value).toBe('');expect(password.placeholder).toBe('')
  button('确定',root.querySelector('[role="dialog"]')!).click();await settle();const request=bridge.SaveConnection.mock.calls[0][0]
  expect(request.id).toBe('');expect(request.passwordAction).toBe('replace');expect(request.password).toBe('')
 })
 it('keeps the dialog open on failed tests without claiming a successful connection',async()=>{
  bridge.TestConnection.mockRejectedValue(new Error('MySQL 拒绝登录，请检查用户名、密码'))
  await mount();createDialog();await settle();button('测试连接').click();await settle()
  expect(root.querySelector('[role="alert"]')?.textContent).toContain('MySQL 拒绝登录');expect(root.querySelector('[role="dialog"]')).not.toBeNull();expect(bridge.Connect).not.toHaveBeenCalled()
 })
 it('opens a connection, expands its databases and renders real metadata from the bridge',async()=>{
  records=[savedProfile];await mount();const connection=root.querySelector<HTMLButtonElement>('.tree-connection')!;connection.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()
  expect(bridge.Connect).toHaveBeenCalledOnce();expect(root.querySelector('.tree-database')?.textContent).toContain('test_db')
  expect(root.querySelector('.tree-database')?.hasAttribute('aria-expanded')).toBe(false)
  expect(root.querySelector('.tree-database .database-open')).toBeNull()
  expect(root.querySelector('.tree-database .tree-expander svg')).toBeNull()
  bridge.ListObjects.mockRejectedValueOnce(new Error('无法读取数据库'))
  root.querySelector<HTMLButtonElement>('.tree-database')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()
  expect(root.querySelector('.tree-database .database-open')).toBeNull()
  expect(root.querySelector('.tree-category')).toBeNull()
  root.querySelector<HTMLButtonElement>('.tree-database')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()
  expect(bridge.ListObjects).toHaveBeenCalledWith('test-profile','test_db','tables');expect(root.querySelector('.object-table tbody')?.textContent).toContain('customers')
  expect(root.querySelector('.tree-database .database-open')).not.toBeNull()
  expect(root.querySelector('.tree-database')?.getAttribute('aria-expanded')).toBe('true')
  root.querySelector<HTMLTableRowElement>('.object-table tbody tr')!.click();await settle();expect(root.querySelector('.information-details')?.textContent).toContain('InnoDB')
  button('关闭连接').click();await settle();connection.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()
  expect(root.querySelector('.tree-database .database-open')).toBeNull()
  expect(root.querySelector('.tree-database .tree-expander svg')).toBeNull()
 })
 it('preserves fields when switching dialog tabs and uses the CA chooser',async()=>{
  await mount();createDialog();await settle();await fill('#connection-name','TLS 服务器');button('SSL',root.querySelector('.connection-tabbar')!).click();await settle()
  const ssl=root.querySelector<HTMLInputElement>('.ssl-check>input')!;ssl.checked=true;ssl.dispatchEvent(new Event('change',{bubbles:true}));await settle()
  root.querySelector<HTMLButtonElement>('[aria-label="选择 CA 证书"]')!.click();await settle();expect(bridge.SelectCAFile).toHaveBeenCalledOnce();expect((root.querySelector('#connection-ca') as HTMLInputElement).value).toBe('test-ca.pem')
  button('常规',root.querySelector('.connection-tabbar')!).click();await settle();expect((root.querySelector('#connection-name') as HTMLInputElement).value).toBe('TLS 服务器')
 })
 it('ignores stale metadata returned after a different database is selected',async()=>{
  records=[savedProfile];bridge.ListSessions.mockResolvedValue([{id:'test-profile',connected:true,databases:['first','second'],serverVersion:'test',currentUser:'test',latencyMs:1,connectedAt:''}]);let release:(result:unknown)=>void=()=>{}
  bridge.ListObjects.mockImplementation(async(_id,db)=>db==='first'?new Promise(resolve=>{release=resolve}):{objects:[{name:'current_object',kind:'tables',rows:'',dataLength:'',indexLength:'',autoIncrement:'',engine:'',comment:'',createdAt:'',modifiedAt:''}],truncated:false})
  await mount();root.querySelector<HTMLButtonElement>('.tree-connection')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()
  const databases=root.querySelectorAll<HTMLButtonElement>('.tree-database');databases[0].dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();databases[1].dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();release({objects:[{name:'stale_object'}],truncated:false});await settle()
  expect(root.querySelector('.object-table tbody')?.textContent).toContain('current_object');expect(root.querySelector('.object-table tbody')?.textContent).not.toContain('stale_object')
 })
})
