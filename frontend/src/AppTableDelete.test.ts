import {createApp,nextTick,type App} from 'vue'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import Workspace from './App.vue'
import {defaults} from './api'
let app:App|undefined,root:HTMLDivElement,admin:ReturnType<typeof vi.fn>,list:ReturnType<typeof vi.fn>,names:string[],fail:boolean
const original=['alpha','beta','odd`table']
async function settle(){for(let i=0;i<20;i++){await nextTick();await Promise.resolve()}}
const rows=()=>Array.from(root.querySelectorAll<HTMLElement>('.object-table tbody tr'))
const selected=()=>rows().filter(row=>row.classList.contains('selected')).map(row=>row.dataset.objectKey)
function button(text:string,scope:ParentNode=root){return Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find(button=>button.textContent?.trim()===text)!}
async function click(index:number,options:MouseEventInit={}){rows()[index]!.dispatchEvent(new MouseEvent('click',{bubbles:true,...options}));await settle()}
beforeEach(()=>{
 localStorage.clear();names=[...original];fail=false;root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}})
 list=vi.fn(async(_id,db)=>({objects:(db==='qa_db'?names:['other_table']).map(name=>({name,kind:'tables',engine:'InnoDB'})),truncated:false}))
 admin=vi.fn(async()=>{names=fail?['beta','odd`table']:[];return {completed:fail?0:1,error:fail?'Foreign key constraint failure':'',results:[],text:''}})
 window.go={main:{App:{ListConnections:vi.fn(async()=>[{...defaults(),id:'qa',name:'QA'}]),ListGroups:vi.fn(async()=>[]),ListSessions:vi.fn(async()=>[{id:'qa',databases:['qa_db','other_db'],connected:true,connectedAt:'qa-session'}]),ConfigPath:vi.fn(async()=>''),ListObjects:list,DatabaseAdmin:admin} as never}}
})
afterEach(()=>{app?.unmount();root.remove();delete window.go;vi.unstubAllGlobals()})
async function mount(){app=createApp(Workspace);app.mount(root);await settle();root.querySelector('.tree-connection')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();root.querySelector('.tree-database')!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle()}
it('supports Ctrl/Shift selection and preserves the group on right click; cancel never deletes',async()=>{
 await mount();await click(0);await click(2,{ctrlKey:true});expect(selected()).toEqual(['alpha','odd`table']);await click(0,{ctrlKey:true});expect(selected()).toEqual(['odd`table']);await click(0);await click(2,{shiftKey:true});expect(selected()).toEqual(original)
 rows()[1]!.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await settle();expect(selected()).toEqual(original);button('删除表',root.querySelector('[role="menu"]')!).click();await settle();expect(Array.from(root.querySelectorAll('.confirmation-items li')).map(n=>n.textContent)).toEqual(original);expect(root.querySelector('[role="alertdialog"]')?.textContent).toContain('qa_db');expect(admin).not.toHaveBeenCalled();button('取消').click();await settle();expect(admin).not.toHaveBeenCalled();expect(selected()).toEqual(original)
})
it('deletes exactly the confirmed names in one quoted SQL statement then refreshes the table list',async()=>{
 await mount();await click(0);await click(2,{shiftKey:true});button('删除表',root.querySelector('.object-toolbar')!).click();await settle();button('删除表',root.querySelector('[role="alertdialog"]')!).click();await settle();expect(admin).toHaveBeenCalledOnce();expect(admin).toHaveBeenCalledWith(expect.any(String),'qa','qa_db',{operation:'query',sql:'DROP TABLE `qa_db`.`alpha`, `qa_db`.`beta`, `qa_db`.`odd``table`;'});expect(rows()).toHaveLength(0);expect(list).toHaveBeenCalledTimes(2)
})
it('refreshes after a partial server failure and clears the old selection before a retry',async()=>{
 fail=true;await mount();await click(0);await click(1,{ctrlKey:true});button('删除表',root.querySelector('.object-toolbar')!).click();await settle();button('删除表',root.querySelector('[role="alertdialog"]')!).click();await settle();expect(rows().map(n=>n.dataset.objectKey)).toEqual(['beta','odd`table']);expect(selected()).toEqual([]);expect(root.textContent).toContain('Foreign key constraint failure');expect(root.textContent).toContain('部分表可能已删除');expect(button('删除表',root.querySelector('.object-toolbar')!).disabled).toBe(true)
})
it('clears selections when filtering or switching database and supports icon-view select all',async()=>{
 await mount();await click(0);await click(2,{shiftKey:true});const filter=root.querySelector<HTMLInputElement>('[aria-label="筛选对象"]')!;filter.value='beta';filter.dispatchEvent(new Event('input',{bubbles:true}));await settle();expect(selected()).toEqual([]);expect(button('删除表',root.querySelector('.object-toolbar')!).disabled).toBe(true)
 filter.value='';filter.dispatchEvent(new Event('input',{bubbles:true}));await settle();root.querySelector<HTMLButtonElement>('[aria-label="图标视图"]')!.click();await settle();const icons=root.querySelectorAll<HTMLButtonElement>('.object-icon-grid>button');icons[0]!.dispatchEvent(new MouseEvent('click',{bubbles:true}));icons[0]!.dispatchEvent(new KeyboardEvent('keydown',{key:'a',ctrlKey:true,bubbles:true,cancelable:true}));await settle();expect(root.querySelectorAll('.object-icon-grid>button.selected')).toHaveLength(3)
 root.querySelectorAll('.tree-database')[1]!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();expect(root.querySelectorAll('.object-icon-grid>button.selected')).toHaveLength(0);expect(admin).not.toHaveBeenCalled()
})
