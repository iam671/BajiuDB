import {createApp,nextTick,type App} from 'vue'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import QueryEditor from './QueryEditor.vue'
import {defaults,type QueryResult} from './api'
import type {CellValueRequest} from './cellValueTools'
let app:App|undefined,root:HTMLDivElement,commands:string[],keyAvailable:boolean,failRead:boolean
const result=(columns:string[],rows:(string|null)[][]):QueryResult=>({columns,rows,affected:'0',truncated:false})
async function settle(){for(let i=0;i<30;i++){await Promise.resolve();await nextTick()}}
function button(text:string){return Array.from(root.querySelectorAll<HTMLButtonElement>('button')).find(b=>b.textContent?.trim()===text)!}
beforeEach(()=>{
 localStorage.clear();commands=[];keyAvailable=true;failRead=false;root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}})
 window.go={main:{App:{CancelDatabaseTask:vi.fn(async()=>{}),DatabaseAdmin:vi.fn(async(_task,_id,_db,req)=>{
  const sql=req.sql??'';commands.push(sql)
  if(sql.startsWith('SHOW FULL'))return {completed:4,error:'',text:'',results:[result(['Field','Type','Null','Key','Default','Extra','Comment'],[['id','bigint','NO','PRI',null,'',''],['name','varchar(50)','NO','',null,'',''],['birthday','date','YES','',null,'','']]),result(['Table','Non_unique','Key_name','Seq_in_index','Column_name','Index_type'],keyAvailable?[['items','0','PRIMARY','1','id','BTREE']]:[]),result([],[]),result([],[])]}
  if(sql.startsWith('UPDATE'))return {completed:1,error:'',text:'',results:[{...result([],[]),affected:'1'}]}
  if(sql.startsWith('SELECT CONVERT'))return {completed:failRead?0:1,error:failRead?'read failed':'',text:'',results:failRead?[]:[result(['id','name','birthday'],[['18446744073709551615','original','2026-11-18']])]}
  return {completed:1,error:'',text:'',results:[result(['name'],[['original']])]}
 })} as never}}
})
afterEach(()=>{app?.unmount();root.remove();delete window.go;vi.unstubAllGlobals()})
async function mount(sql='SELECT name AS display_name,birthday FROM items WHERE id>0 LIMIT 10'){
 const confirm=vi.fn(),quick=vi.fn();app=createApp(QueryEditor,{profile:{...defaults(),id:'qa'},database:'qa',initialSQL:sql,active:true,connected:true,sessionKey:'qa',onConfirm:confirm,onQuickEdit:quick});app.mount(root);await settle();button('执行（选中或全部）').click();await settle();return {confirm,quick}
}
it('edits projected fields, retains hidden key/snapshot guards and only writes on Save',async()=>{
 const {confirm,quick}=await mount();expect(root.querySelectorAll('.table-data-grid thead th')).toHaveLength(3);expect(root.querySelector('.table-data-grid thead')?.textContent).toContain('display_name');expect(commands.filter(s=>s.startsWith('SELECT'))).toHaveLength(1)
 const cells=root.querySelectorAll('.table-data-grid tbody tr:not(.table-virtual-spacer) td');cells[1]!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();const input=root.querySelector<HTMLTextAreaElement>('.grid-cell-editor textarea')!;input.value='changed';input.dispatchEvent(new Event('input',{bubbles:true}));await settle();expect(commands.some(s=>s.startsWith('UPDATE'))).toBe(false)
 button('执行（选中或全部）').click();await settle();expect(confirm).toHaveBeenCalledOnce();expect(commands.filter(s=>s.startsWith('SELECT'))).toHaveLength(1)
 button('保存').click();await settle();const update=commands.find(s=>s.startsWith('UPDATE'))!;expect(update).toContain('`id` <=>');expect(update).toContain('CAST(CONVERT(`birthday` USING utf8mb4) AS BINARY)');expect(update).toMatch(/LIMIT 1;$/)
 const dateCell=root.querySelectorAll('.table-data-grid tbody tr:not(.table-virtual-spacer) td')[2]!;dateCell.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await settle();root.querySelector<HTMLButtonElement>('[aria-label="快捷编辑birthday"]')!.click();await settle();expect(quick).toHaveBeenCalledOnce();const request=quick.mock.calls[0]![0] as CellValueRequest;expect(request.column.type).toBe('date');request.apply({mode:'value',value:'2026-12-01'});await settle();expect(root.textContent).toContain('2026-12-01');expect(commands.filter(s=>s.startsWith('UPDATE'))).toHaveLength(1)
})
it('executes complex queries once as read-only and falls back before data reads when no unique key exists',async()=>{
 await mount('SELECT count(*) FROM items');expect(commands).toEqual(['SELECT count(*) FROM items']);expect(root.querySelector('.table-data-editor')).toBeNull()
 app!.unmount();root.innerHTML='';commands=[];keyAvailable=false;await mount('SELECT name FROM items');expect(commands.filter(s=>s==='SELECT name FROM items')).toHaveLength(1);expect(commands.some(s=>s.startsWith('SELECT CONVERT'))).toBe(false);expect(root.textContent).toContain('只读结果')
})
it('does not execute the original SQL again when an actual canonical data read fails',async()=>{
 failRead=true;await mount('SELECT name FROM items');expect(commands.filter(s=>s.startsWith('SELECT'))).toHaveLength(1);expect(commands).not.toContain('SELECT name FROM items');expect(root.textContent).toContain('read failed')
})
