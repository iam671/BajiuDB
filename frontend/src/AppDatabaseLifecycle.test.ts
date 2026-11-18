import {createApp,nextTick,type App} from 'vue'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import Workspace from './App.vue'
import {defaults} from './api'
vi.mock('./TableDesigner.vue',()=>({default:{props:['profile','database'],emits:['dirty'],template:'<section class="qa-design"><span>{{profile.name}} / {{database}}</span><button @click="$emit(\'dirty\',true)">修改设计草稿</button></section>'}}))
let app:App|undefined,root:HTMLDivElement,admin:ReturnType<typeof vi.fn>,cancel:ReturnType<typeof vi.fn>,list:ReturnType<typeof vi.fn>,disconnect:ReturnType<typeof vi.fn>
const pending=new Map<string,(value:unknown)=>void>()
const result=(columns:string[]=[],rows:(string|null)[][]=[])=>({columns,rows,affected:'0',truncated:false})
async function settle(){for(let i=0;i<25;i++){await nextTick();await Promise.resolve()}}
const mouse=(type:string)=>new MouseEvent(type,{bubbles:true,cancelable:true})
function button(text:string,scope:ParentNode=root){return Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find(button=>button.textContent?.trim()===text)!}
function database(profile:string,db:string){const connection=Array.from(root.querySelectorAll('.tree-connection')).find(node=>node.textContent?.trim()===profile)!;return Array.from(connection.nextElementSibling!.querySelectorAll('.tree-database')).find(node=>node.getAttribute('title')===db)!}
async function openConnection(name:string){Array.from(root.querySelectorAll('.tree-connection')).find(node=>node.textContent?.trim()===name)!.dispatchEvent(mouse('dblclick'));await settle()}
async function menu(name:string,profile='QA',db='qa_db'){database(profile,db).dispatchEvent(mouse('contextmenu'));await settle();button(name,root.querySelector('[role="menu"]')!).click();await settle()}
function input(node:HTMLTextAreaElement|HTMLInputElement,text:string){node.value=text;node.dispatchEvent(new Event('input',{bubbles:true}))}
beforeEach(()=>{
 localStorage.clear();pending.clear();root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}})
 list=vi.fn(async()=>({objects:[{name:'items',kind:'tables',engine:'InnoDB'}],truncated:false}));disconnect=vi.fn(async()=>{})
 cancel=vi.fn(async(task:string)=>{pending.get(task)?.({completed:0,error:'cancelled',results:[],text:''});pending.delete(task)})
 admin=vi.fn(async(task,_id,_db,request)=>{
  if(request.operation==='transaction-begin')return {completed:1,error:'',results:[],text:'',transactionId:'qa-tx',transactionActive:true}
  if(request.operation==='transaction-rollback')return {completed:1,error:'',results:[],text:'',transactionActive:false}
  if(request.sql==='SELECT wait;')return new Promise(resolve=>pending.set(task,resolve))
  if(request.sql?.startsWith('SHOW FULL COLUMNS'))return {completed:4,error:'',text:'',results:[result(['Field','Type','Null','Key','Default','Extra','Comment'],[['id','int','NO','PRI',null,'',''],['name','varchar(50)','YES','',null,'','']]),result(['Table','Non_unique','Key_name','Seq_in_index','Column_name','Index_type'],[['items','0','PRIMARY','1','id','BTREE']]),result(),result()]}
  return {completed:1,error:'',text:'',results:[result(['id','name'],[['1','draft']])],transactionActive:!!request.transactionId}
 })
 window.go={main:{App:{ListConnections:vi.fn(async()=>[{...defaults(),id:'qa',name:'QA'},{...defaults(),id:'qb',name:'QB'}]),ListGroups:vi.fn(async()=>[]),ListSessions:vi.fn(async()=>[{id:'qa',databases:['qa_db','other_db'],connected:true,connectedAt:'qa'},{id:'qb',databases:['qa_db'],connected:true,connectedAt:'qb'}]),ConfigPath:vi.fn(async()=>''),ListObjects:list,DatabaseAdmin:admin,CancelDatabaseTask:cancel,Disconnect:disconnect} as never}}
})
afterEach(()=>{app?.unmount();root.remove();delete window.go;vi.unstubAllGlobals()})
async function mount(){app=createApp(Workspace);app.mount(root);await settle();await openConnection('QA');database('QA','qa_db').dispatchEvent(mouse('dblclick'));await settle()}
it('provides the exact blank-area menu, clears selection and routes find/new-table to this database',async()=>{
 await mount();root.querySelector('.object-table tbody tr')!.dispatchEvent(mouse('click'));await settle();const blank=root.querySelector('.object-list-scroll')!;blank.dispatchEvent(mouse('contextmenu'));await settle()
 expect(root.querySelectorAll('.object-table tbody tr.selected')).toHaveLength(0)
 expect(Array.from(root.querySelectorAll('[role="menu"] button')).map(node=>node.textContent?.trim())).toEqual(['新建表','导入向导','导出向导','数据字典','数据生成','运行SQL文件','在数据库中查找','刷新'])
 button('在数据库中查找',root.querySelector('[role="menu"]')!).click();await settle();expect(document.activeElement).toBe(root.querySelector('[aria-label="筛选对象"]'))
 blank.dispatchEvent(mouse('contextmenu'));await settle();button('刷新',root.querySelector('[role="menu"]')!).click();await settle();expect(list.mock.calls.at(-1)?.slice(0,2)).toEqual(['qa','qa_db'])
 blank.dispatchEvent(mouse('contextmenu'));await settle();button('新建表',root.querySelector('[role="menu"]')!).click();await settle();expect(root.querySelector('.qa-design')?.textContent).toContain('QA / qa_db')
})
it('closes every matching tab after one dirty confirmation, cancels work/rolls back, and preserves other databases/connections',async()=>{
 await mount();root.querySelector('.object-table tbody tr')!.dispatchEvent(mouse('dblclick'));await settle();button('开始事务').click();await settle()
 button('对象').click();await settle();button('新建表').click();await settle();button('修改设计草稿').click();await settle()
 await menu('新建查询');const query=root.querySelector<HTMLTextAreaElement>('.query-editor-input')!;input(query,'SELECT wait;');await settle();button('执行（选中或全部）').click();await settle();const running=admin.mock.calls.find(call=>call[3].sql==='SELECT wait;')![0]
 await menu('命令行界面');input(root.querySelector<HTMLTextAreaElement>('.sql-console textarea')!,'pending command');await settle()
 await menu('新建查询','QA','other_db');await openConnection('QB');await menu('新建查询','QB','qa_db');expect(root.querySelectorAll('.query-workspace-tab')).toHaveLength(6)
 await menu('关闭数据库');expect(root.querySelector('[role="alertdialog"]')?.textContent).toContain('4 个标签页');button('取消').click();await settle();expect(root.querySelectorAll('.query-workspace-tab')).toHaveLength(6);expect(cancel).not.toHaveBeenCalled();expect(database('QA','qa_db').getAttribute('aria-expanded')).not.toBeNull()
 await menu('关闭数据库');button('关闭数据库',root.querySelector('[role="alertdialog"]')!).click();await settle()
 expect(root.querySelectorAll('.query-workspace-tab')).toHaveLength(2);const titles=Array.from(root.querySelectorAll('.query-tab-button')).map(node=>node.getAttribute('title'));expect(titles).toEqual([expect.stringContaining('QA / other_db'),expect.stringContaining('QB / qa_db')]);expect(root.querySelector('.table-data-editor')).toBeNull();expect(root.querySelector('.qa-design')).toBeNull();expect(root.querySelector('.sql-console')).toBeNull()
 expect(cancel).toHaveBeenCalledWith(running);expect(admin).toHaveBeenCalledWith(expect.any(String),'qa','qa_db',{operation:'transaction-rollback',transactionId:'qa-tx'});expect(disconnect).not.toHaveBeenCalled();expect(database('QA','qa_db').getAttribute('aria-expanded')).toBeNull()
})
it('closes clean tabs immediately and allows reopening the database',async()=>{
 await mount();await menu('新建查询');expect(root.querySelectorAll('.query-workspace-tab')).toHaveLength(1);await menu('关闭数据库');expect(root.querySelector('[role="alertdialog"]')).toBeNull();expect(root.querySelectorAll('.query-workspace-tab')).toHaveLength(0);await menu('打开数据库');expect(database('QA','qa_db').getAttribute('aria-expanded')).toBe('true')
})
