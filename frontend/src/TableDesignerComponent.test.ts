import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {createApp,nextTick,type App} from 'vue'
import TableDesigner from './TableDesigner.vue'
import {defaults} from './api'
import {parseDesign} from './tableDesigner'
let app:App|undefined,root:HTMLDivElement,commands:string[],confirmations:{title:string;action:()=>void|Promise<void>}[],saved:string[],ddl:string
const schema='utf8mb4_unicode_ci'
const result=(columns:string[],rows:(string|null)[][])=>({columns,rows,truncated:false,affected:'0'})
async function settle(){for(let i=0;i<16;i++){await Promise.resolve();await nextTick()}}
function button(text:string){const element=Array.from(root.querySelectorAll<HTMLButtonElement>('button')).find(b=>b.textContent?.trim()===text);if(!element)throw new Error('Button missing: '+text);return element}
async function click(text:string){button(text).click();await settle()}
async function input(label:string,value:string){const element=root.querySelector<HTMLInputElement|HTMLTextAreaElement>('[aria-label="'+label+'"]');if(!element)throw new Error('Input missing: '+label);element.value=value;element.dispatchEvent(new Event('input',{bubbles:true}));await settle()}
async function mount(table?:string){app=createApp(TableDesigner,{profile:{...defaults(),id:'fake'},database:'qa',table,connected:true,active:true,sessionKey:'qa-session',onConfirm:(request:typeof confirmations[number])=>confirmations.push(request),onSaved:(name:string)=>saved.push(name)});app.mount(root);await settle()}
beforeEach(()=>{
 localStorage.clear();commands=[];confirmations=[];saved=[];ddl='';root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class {observe(){}disconnect(){}})
 window.go={main:{App:{DatabaseAdmin:vi.fn(async(_task,_id,_db,req)=>{
  if(req.operation==='catalog')return {results:[result(['Collation','Charset','Id','Default'],[[schema,'utf8mb4','1',''],['utf8mb4_general_ci','utf8mb4','2','Yes'],['latin1_swedish_ci','latin1','3','Yes']])],completed:1,error:'',text:''}
  if(req.operation==='schema')return {results:[result(['DEFAULT_CHARACTER_SET_NAME','DEFAULT_COLLATION_NAME'],[['utf8mb4',schema]])],completed:1,error:'',text:''}
  commands.push(req.sql??'')
  if(req.sql?.startsWith('SHOW CREATE TABLE')){const design=parseDesign(ddl);return {results:[result(['Table','Create Table'],[['items',ddl]]),result(['sql_mode'],[['STRICT_TRANS_TABLES']]),result(['TRIGGER_NAME','ACTION_TIMING','EVENT_MANIPULATION','ACTION_STATEMENT'],[]),result(['Field','Collation'],design.fields.map(f=>[f.name,/^(?:char|varchar|text)$/i.test(f.type)?f.collation||design.options.collation||schema:null])),result(['TABLE_COLLATION'],[[design.options.collation||schema]])],completed:5,error:'',text:''}}
  if(req.sql?.includes('CREATE TABLE')){ddl=req.sql.slice(req.sql.indexOf('CREATE TABLE'));return {results:[],completed:2,error:'',text:''}}
  return {results:[result(['sql_mode'],[['STRICT_TRANS_TABLES']])],completed:1,error:'',text:''}
 }),CancelDatabaseTask:vi.fn(async()=>{})} as never}}
})
afterEach(()=>{app?.unmount();app=undefined;root.remove();delete window.go;vi.unstubAllGlobals()})
describe('designer controls',()=>{
 it('shows type-specific properties, retains comments and clears incompatible attributes on type changes',async()=>{
  await mount();await input('字段 1 名称','value');await click('选项');await input('表名','typed');await click('字段')
  const checkbox=(text:string)=>Array.from(root.querySelectorAll<HTMLLabelElement>('.native-check')).find(label=>label.textContent?.trim()===text)?.querySelector<HTMLInputElement>('input')
  expect(checkbox('自动递增')).toBeUndefined();expect(root.querySelector('[aria-label="字段字符集"]')).not.toBeNull()
  await input('字段 1 类型','int');expect(checkbox('自动递增')).toBeDefined();expect(checkbox('填充零')).toBeDefined();expect(root.querySelector('[aria-label="字段字符集"]')).toBeNull()
  checkbox('填充零')!.click();await settle();expect(checkbox('无符号')!.checked).toBe(true)
  checkbox('自动递增')!.click();await settle();expect(root.querySelector<HTMLInputElement>('[aria-label="字段默认值"]')!.disabled).toBe(true)
  await input('字段 1 类型','timestamp');expect(checkbox('自动递增')).toBeUndefined();expect(checkbox('根据当前时间戳更新')).toBeDefined();checkbox('根据当前时间戳更新')!.click();await settle()
  await input('字段默认值','CURRENT_TIMESTAMP');await input('value长度','3');await input('字段注释','all types');await click('SQL 预览');expect(root.querySelector('textarea')?.value).toContain('DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)');expect(root.querySelector('textarea')?.value).not.toMatch(/UNSIGNED|ZEROFILL|AUTO_INCREMENT/)
  await click('字段');await input('字段 1 类型','date');expect(checkbox('根据当前时间戳更新')).toBeUndefined();expect(root.querySelector<HTMLTextAreaElement>('[aria-label="字段注释"]')!.value).toBe('all types')
  await input('字段 1 类型','varchar');expect(root.querySelector('[aria-label="字段字符集"]')).not.toBeNull();expect(checkbox('填充零')).toBeUndefined()
  expect(commands.some(sql=>sql.includes('CREATE TABLE')||sql.includes('ALTER TABLE'))).toBe(false)
 })
 it('edits defaults in one combobox with empty-string and NULL presets',async()=>{
  await mount();await input('字段 1 名称','title');await click('选项');await input('表名','defaults');await click('字段')
  const control=root.querySelector<HTMLInputElement>('[aria-label="字段默认值"]')!
  expect(control.value).toBe('');expect(root.querySelector('[aria-label="默认值类型"]')).toBeNull()
  control.focus();await settle();expect(Array.from(root.querySelectorAll('[role=option]')).map(el=>el.textContent)).toEqual(['EMPTY STRING','NULL'])
  ;(root.querySelector('[role=option]') as HTMLElement).dispatchEvent(new MouseEvent('mousedown',{bubbles:true}));await settle();expect(control.value).toBe('EMPTY STRING')
  await click('SQL 预览');expect(root.querySelector('textarea')?.value).toContain("DEFAULT ''")
  await click('字段');await input('字段默认值','NULL');await click('SQL 预览');expect(root.querySelector('textarea')?.value).toContain('DEFAULT NULL')
  await click('字段');await input('字段默认值','custom');await click('SQL 预览');expect(root.querySelector('textarea')?.value).toContain("DEFAULT 'custom'")
  await click('字段');await input('字段默认值','');await click('SQL 预览');expect(root.querySelector('textarea')?.value.split('\n').find(line=>line.includes('`title`'))).not.toContain(' DEFAULT ')
  expect(commands.some(sql=>sql.includes('CREATE TABLE')||sql.includes('ALTER TABLE'))).toBe(false)
 })
 it('inherits database defaults, allows multiline comments and saves directly without a confirmation event',async()=>{
  await mount();expect(root.querySelector<HTMLInputElement>('[aria-label="字段字符集"]')?.value).toBe('utf8mb4');expect(root.querySelector<HTMLInputElement>('[aria-label="字段排序规则"]')?.value).toBe(schema);expect(root.querySelector('select')).toBeNull();await input('字段 1 名称','title');await input('字段注释','first line\nsecond line');await click('选项');await input('表名','notes');await click('保存');expect(confirmations).toHaveLength(0);expect(saved).toEqual(['notes']);expect(commands.some(sql=>sql.includes('CREATE TABLE'))).toBe(true);expect(ddl).toContain("COMMENT 'first line\nsecond line'")
 })
 it('confirms removal of every unsaved field, index, foreign key and trigger before changing the draft',async()=>{
  await mount();await click('删除字段');expect(confirmations).toHaveLength(1);expect(root.querySelectorAll('[aria-label="字段设计"] tbody tr')).toHaveLength(1);await confirmations.at(-1)!.action();await settle();expect(root.querySelectorAll('[aria-label="字段设计"] tbody tr')).toHaveLength(0)
  for(const [tab,add,remove,label] of [['索引','添加索引','删除索引','索引设计'],['外键','添加外键','删除外键','外键设计'],['触发器','添加触发器','删除触发器','触发器主体']]){await click(tab!);await click(add!);const before=confirmations.length;await click(remove!);expect(confirmations).toHaveLength(before+1);if(tab!=='触发器')expect(root.querySelectorAll('[aria-label="'+label+'"] tbody tr')).toHaveLength(1);await confirmations.at(-1)!.action();await settle();if(tab!=='触发器')expect(root.querySelectorAll('[aria-label="'+label+'"] tbody tr')).toHaveLength(0);else expect(root.querySelector('[aria-label="触发器主体"]')).toBeNull()}
  expect(commands.some(sql=>sql.includes('CREATE TABLE')||sql.includes('ALTER TABLE'))).toBe(false)
 })
 it('updates collation to the matching charset instead of retaining an incompatible old value',async()=>{await mount();await click('选项');await input('表字符集','latin1');expect(root.querySelector<HTMLInputElement>('[aria-label="表排序规则"]')?.value).toBe('latin1_swedish_ci')})
 it('shows the actual existing table charset and supports resizing the properties area without dirtying schema',async()=>{
  ddl="CREATE TABLE `items` (`title` varchar(80) DEFAULT NULL) ENGINE=InnoDB DEFAULT CHARSET=latin1 COLLATE=latin1_swedish_ci";await mount('items');expect(root.querySelector<HTMLInputElement>('[aria-label="字段字符集"]')?.value).toBe('latin1');expect(root.querySelector<HTMLInputElement>('[aria-label="字段排序规则"]')?.value).toBe('latin1_swedish_ci');const separator=root.querySelector<HTMLElement>('[aria-label="调整字段属性区域高度"]')!,before=Number(separator.getAttribute('aria-valuenow'));separator.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowUp',bubbles:true}));await settle();expect(Number(separator.getAttribute('aria-valuenow'))).toBe(before+12);expect(JSON.parse(localStorage.getItem('bajiu.designer-properties.v1')!).height).toBe(before+12);expect(button('保存').disabled).toBe(true)
 })
})
