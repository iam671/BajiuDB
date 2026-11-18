import {createApp,nextTick,type App} from 'vue'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import Workspace from './App.vue'
import {defaults,installNativeBridge,type WindowDialog} from './api'

let app:App|undefined,root:HTMLDivElement
async function settle(){for(let i=0;i<20;i++){await nextTick();await Promise.resolve()}}
const mouse=(type:string)=>new MouseEvent(type,{bubbles:true,cancelable:true})
const key=async(node:Element,value:string)=>{node.dispatchEvent(new KeyboardEvent('keydown',{key:value,bubbles:true,cancelable:true}));await settle()}
function button(text:string,scope:ParentNode=root){return Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find(node=>node.textContent?.trim()===text)!}
function groups(){const result:string[][]=[[]];for(const node of Array.from(root.querySelector('.connection-context-menu')!.children)){if(node.getAttribute('role')==='separator')result.push([]);else result.at(-1)!.push(node.textContent!.trim())}return result}
async function context(selector:string){root.querySelector(selector)!.dispatchEvent(mouse('contextmenu'));await settle()}
async function mount(){app=createApp(Workspace);app.mount(root);await settle();root.querySelector('.tree-connection')!.dispatchEvent(mouse('dblclick'));await settle()}
beforeEach(()=>{
 localStorage.clear();root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}})
 window.go={main:{App:{ListConnections:vi.fn(async()=>[{...defaults(),id:'qa',name:'QA'}]),ListGroups:vi.fn(async()=>[{id:'group',name:'Group'}]),ListSessions:vi.fn(async()=>[{id:'qa',databases:['qa_db'],connected:true}]),ConfigPath:vi.fn(async()=>''),ListObjects:vi.fn(async()=>({objects:[{name:'items',kind:'tables',engine:'InnoDB'}],truncated:false}))} as never}}
})
afterEach(()=>{app?.unmount();root.remove();delete window.go;installNativeBridge(undefined);vi.unstubAllGlobals()})
it('separates database commands into the requested five groups and keeps the open/close item conditional',async()=>{
 await mount();await context('.tree-database');expect(groups()).toEqual([['打开数据库'],['编辑数据库','新建数据库','删除数据库'],['新建查询'],['命令行界面','运行SQL文件','转储SQL文件','数据字典','逆向数据库到模型'],['刷新']])
 button('打开数据库').click();await settle();await context('.tree-database');expect(groups()[0]).toEqual(['关闭数据库'])
})
it('opens dump choices using the keyboard, returns to the parent and dismisses the entire menu outside',async()=>{
 await mount();await context('.tree-database');const parent=button('转储SQL文件');parent.focus();await key(parent,'ArrowRight');expect(document.activeElement).toBe(button('结构和数据...'));await key(document.activeElement!,'ArrowDown');expect(document.activeElement).toBe(button('仅结构...'));await key(document.activeElement!,'ArrowLeft');expect(document.activeElement).toBe(parent);expect(root.querySelector('.context-submenu')).toBeNull()
 parent.click();await settle();await key(document.activeElement!,'Escape');expect(root.querySelector('.context-submenu')).toBeNull();expect(document.activeElement).toBe(parent)
 parent.dispatchEvent(mouse('mouseenter'));await settle();expect(root.querySelector('.context-submenu')).not.toBeNull();root.querySelector('.application-statusbar')!.dispatchEvent(mouse('pointerdown'));await settle();expect(root.querySelector('.connection-context-menu')).toBeNull()
 await context('.tree-database');button('转储SQL文件').click();await settle();root.querySelector('.application-statusbar')!.dispatchEvent(mouse('contextmenu'));await settle();expect(root.querySelector('.connection-context-menu')).toBeNull()
})
it.each([['结构和数据...',true],['仅结构...',false]] as const)('passes the %s choice into a native dump window without executing SQL',async(label,includeData)=>{
 const opened:WindowDialog[]=[];installNativeBridge(async()=>undefined);Object.assign(window.go!.main.App,{OpenWindowDialog:vi.fn(async(value:WindowDialog)=>{opened.push(value)}),CloseWindowDialog:vi.fn(async()=>{}),UpdateWindowDialog:vi.fn(async()=>{})})
 await mount();await context('.tree-database');button('转储SQL文件').click();await settle();button(label).dispatchEvent(mouse('pointerdown'));await settle();expect(root.querySelector('.context-submenu')).not.toBeNull();button(label).click();await settle()
 expect(opened).toHaveLength(1);expect(opened[0]).toMatchObject({kind:'database',payload:{profile:{id:'qa'},database:'qa_db',mode:'dump',dumpIncludeData:includeData}});expect(root.querySelector('.connection-context-menu')).toBeNull();expect(root.querySelector('.database-tool-dialog')).toBeNull()
})
it('groups connection, root, group, table and blank-area menus without changing their commands',async()=>{
 await mount();await context('.tree-connection');expect(groups()).toEqual([['关闭连接'],['编辑连接…','复制连接…'],['移至分组…'],['删除连接']]);await context('.tree-root');expect(groups()).toEqual([['新建连接…','新建组'],['关闭所有连接']]);await context('.tree-group');expect(groups()).toEqual([['新建连接…'],['重命名组'],['删除组'],['关闭所有连接']])
 root.querySelector('.tree-database')!.dispatchEvent(mouse('dblclick'));await settle();await context('.object-table tbody tr');expect(groups()).toEqual([['新建表','设计表'],['导入向导','导出向导'],['数据字典','数据生成','运行SQL文件...','在数据库中查询'],['删除表'],['刷新']]);await context('.object-list-scroll');expect(groups()).toEqual([['新建表'],['导入向导','导出向导'],['数据字典','数据生成','运行SQL文件'],['在数据库中查找'],['刷新']])
})
