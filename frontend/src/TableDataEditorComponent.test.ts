import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import {createApp,nextTick,type App} from 'vue'
import TableDataEditor from './TableDataEditor.vue'
import {defaults,type QueryResult} from './api'
let app:App|undefined,root:HTMLDivElement,commands:string[]
const result=(columns:string[],rows:(string|null)[][]):QueryResult=>({columns,rows,affected:'0',truncated:false})
const names=['id','first','calculated','shape','last']
async function settle(){for(let i=0;i<16;i++){await Promise.resolve();await nextTick()}}
function button(text:string){return Array.from(root.querySelectorAll<HTMLButtonElement>('button')).find(button=>button.textContent?.trim()===text)!}
function edit(value:string){const input=document.activeElement as HTMLTextAreaElement;input.value=value;input.dispatchEvent(new Event('input',{bubbles:true}))}
async function tab(shift=false){document.activeElement!.dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',shiftKey:shift,bubbles:true,cancelable:true}));await settle()}
beforeEach(()=>{
 localStorage.clear();commands=[];root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class {observe(){}disconnect(){}})
 window.go={main:{App:{CancelDatabaseTask:vi.fn(async()=>{}),DatabaseAdmin:vi.fn(async(_task,_id,_db,req)=>{
  commands.push(req.sql??'')
  if(req.sql?.startsWith('SHOW FULL COLUMNS'))return {completed:4,error:'',text:'',results:[result(['Field','Type','Collation','Null','Key','Default','Extra','Privileges','Comment'],[['id','int',null,'NO','PRI',null,'auto_increment','',''],['first','varchar(50)',null,'YES','',null,'','',''],['calculated','int',null,'YES','',null,'VIRTUAL GENERATED','',''],['shape','point',null,'YES','',null,'','',''],['last','varchar(50)',null,'YES','',null,'','','']]),result(['Table','Non_unique','Key_name','Seq_in_index','Column_name','Index_type'],[['items','0','PRIMARY','1','id','BTREE']]),result([],[]),result([],[])]}
  return {completed:1,error:'',text:'',results:[result(names,[['1','a','1',null,'z']])]}
 })} as never}}
})
afterEach(()=>{app?.unmount();root.remove();delete window.go;vi.unstubAllGlobals()})
async function mount(onDesign=()=>{}){app=createApp(TableDataEditor,{profile:{...defaults(),id:'qa'},database:'qa',table:'items',section:'data',active:true,connected:true,sessionKey:'qa',onDesign});app.mount(root);await settle()}
it('places save/add/delete first and tabs through writable new cells without writing to the server',async()=>{
 await mount();expect(Array.from(root.querySelectorAll('.table-data-toolbar>button')).slice(0,4).map(button=>button.textContent?.trim())).toEqual(['保存','新增','删除','开始事务'])
 button('新增').click();await settle();expect(document.activeElement?.getAttribute('aria-label')).toBe('id');await tab();expect(document.activeElement?.getAttribute('aria-label')).toBe('first');edit('draft');await tab();expect(document.activeElement?.getAttribute('aria-label')).toBe('last');await tab(true);expect(document.activeElement?.getAttribute('aria-label')).toBe('first');await tab();await tab();expect(document.activeElement).toBe(button('保存'));expect(commands.some(sql=>/^\s*(?:INSERT|UPDATE|DELETE)\b/.test(sql))).toBe(false)
})
it('keeps the same Tab sequence in form view',async()=>{
 await mount();root.querySelector<HTMLButtonElement>('[aria-label="表单视图"]')!.click();await settle();button('新增').click();await settle();expect(document.activeElement?.getAttribute('aria-label')).toBe('id');await tab();expect(document.activeElement?.getAttribute('aria-label')).toBe('first');await tab();expect(document.activeElement?.getAttribute('aria-label')).toBe('last');expect(commands.some(sql=>/^\s*(?:INSERT|UPDATE|DELETE)\b/.test(sql))).toBe(false)
})
it('opens design from each metadata section immediately before Refresh without writing data',async()=>{
 const design=vi.fn();await mount(design);expect(root.querySelector('.table-data-toolbar')?.textContent).not.toContain('设计表')
 for(const label of ['字段','索引','外键','触发器']){button(label).click();await settle();expect(Array.from(root.querySelectorAll('.table-data-toolbar>button')).map(b=>b.textContent?.trim())).toEqual(['设计表','刷新']);button('设计表').click();await settle()}
 expect(design).toHaveBeenCalledTimes(4);expect(commands.some(sql=>/^\s*(?:INSERT|UPDATE|DELETE|ALTER)\b/.test(sql))).toBe(false)
})
