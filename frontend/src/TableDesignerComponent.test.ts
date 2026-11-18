import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {createApp,nextTick,type App} from 'vue'
import TableDesigner from './TableDesigner.vue'
import type {IndexFieldsRequest} from './IndexFieldsDialog.vue'
import type {TableNameRequest} from './TableNameDialog.vue'
import {defaults,type DialogDetails} from './api'
import {parseDesign} from './tableDesigner'
let app:App|undefined,root:HTMLDivElement,commands:string[],confirmations:{title:string;action:()=>void|Promise<void>}[],saved:string[],ddl:string,details:DialogDetails[],failure:string,failureCode:number,pickers:IndexFieldsRequest[],namePrompts:TableNameRequest[]
const schema='utf8mb4_unicode_ci'
const result=(columns:string[],rows:(string|null)[][])=>({columns,rows,truncated:false,affected:'0'})
async function settle(){for(let i=0;i<16;i++){await Promise.resolve();await nextTick()}}
function button(text:string){const element=Array.from(root.querySelectorAll<HTMLButtonElement>('button')).find(b=>b.textContent?.trim()===text);if(!element)throw new Error('Button missing: '+text);return element}
async function click(text:string){button(text).click();await settle()}
async function input(label:string,value:string){const element=root.querySelector<HTMLInputElement|HTMLTextAreaElement>('[aria-label="'+label+'"]');if(!element)throw new Error('Input missing: '+label);element.value=value;element.dispatchEvent(new Event('input',{bubbles:true}));await settle()}
async function mount(table?:string){app=createApp(TableDesigner,{profile:{...defaults(),id:'fake'},database:'qa',table,connected:true,active:true,sessionKey:'qa-session','onIndex-fields':(value:IndexFieldsRequest)=>pickers.push(value),'onRequest-name':(value:TableNameRequest)=>namePrompts.push(value),onDetails:(value:DialogDetails)=>details.push(value),onConfirm:(request:typeof confirmations[number])=>confirmations.push(request),onSaved:(name:string)=>saved.push(name)});app.mount(root);await settle()}
beforeEach(()=>{
 localStorage.clear();commands=[];confirmations=[];saved=[];ddl='';details=[];failure='';failureCode=0;pickers=[];namePrompts=[];root=document.createElement('div');document.body.append(root);vi.stubGlobal('ResizeObserver',class {observe(){}disconnect(){}})
 window.go={main:{App:{DatabaseAdmin:vi.fn(async(_task,_id,_db,req)=>{
  if(req.operation==='catalog')return {results:[result(['Collation','Charset','Id','Default'],[[schema,'utf8mb4','1',''],['utf8mb4_general_ci','utf8mb4','2','Yes'],['latin1_swedish_ci','latin1','3','Yes']])],completed:1,error:'',text:''}
  if(req.operation==='schema')return {results:[result(['DEFAULT_CHARACTER_SET_NAME','DEFAULT_COLLATION_NAME'],[['utf8mb4',schema]])],completed:1,error:'',text:''}
  commands.push(req.sql??'')
  if(failure&&req.sqlMode!==undefined)return {results:[],completed:1,error:failure,errorCode:failureCode||undefined,text:''}
  if(req.sql?.startsWith('SELECT TRIGGER_NAME'))return {results:[result(['TRIGGER_NAME','ACTION_TIMING','EVENT_MANIPULATION','ACTION_STATEMENT'],[])],completed:1,error:'',text:''}
  if(req.sql?.startsWith('SHOW CREATE TABLE')){const design=parseDesign(ddl);return {results:[result(['Table','Create Table'],[['items',ddl]]),result(['sql_mode'],[['STRICT_TRANS_TABLES']]),result(['TRIGGER_NAME','ACTION_TIMING','EVENT_MANIPULATION','ACTION_STATEMENT'],[]),result(['Field','Collation'],design.fields.map(f=>[f.name,/^(?:char|varchar|text)$/i.test(f.type)?f.collation||design.options.collation||schema:null])),result(['TABLE_COLLATION'],[[design.options.collation||schema]])],completed:5,error:'',text:''}}
  if(req.sql?.includes('CREATE TABLE')){ddl=req.sql.slice(req.sql.indexOf('CREATE TABLE'));return {results:[],completed:2,error:'',text:''}}
  return {results:[result(['sql_mode'],[['STRICT_TRANS_TABLES']])],completed:1,error:'',text:''}
 }),TableDDL:vi.fn(async()=>ddl),CancelDatabaseTask:vi.fn(async()=>{})} as never}}
})
afterEach(()=>{app?.unmount();app=undefined;root.remove();delete window.go;vi.unstubAllGlobals()})
describe('designer controls',()=>{
 it('offers the duplicate name again on the next save without losing the field draft',async()=>{
  await mount();await input('字段 1 名称','title');await click('选项');await input('表名','f_test1');await click('字段');failure="第 2 条语句失败：Error 1050 (42S01): Table 'f_test1' already exists";await click('保存')
  expect(details.at(-1)?.details).toContain('1050');expect(namePrompts).toHaveLength(0);await click('保存');expect(namePrompts[0]?.value).toBe('f_test1');expect(root.querySelector<HTMLInputElement>('[aria-label="字段 1 名称"]')?.value).toBe('title')
  await click('保存');expect(namePrompts).toHaveLength(2);failure='';await namePrompts[1]!.save('f_test2');await settle();expect(saved).toEqual(['f_test2']);expect(commands.at(-2)?.includes('f_test2')||commands.some(sql=>sql.includes('CREATE TABLE `qa`.`f_test2`'))).toBe(true)
 })
 it('uses a structured duplicate-table code from the tunnel even without an English message',async()=>{
  await mount();await input('字段 1 名称','title');await click('选项');await input('表名','duplicate');await click('字段');failure='表已经存在';failureCode=1050;await click('保存');await click('保存');expect(namePrompts[0]?.value).toBe('duplicate')
 })
 it('suggests type and length only for new fields whose type settings have not been edited',async()=>{
  await mount();await input('字段 1 名称','user_id');root.querySelector('[aria-label="字段 1 名称"]')!.dispatchEvent(new Event('change',{bubbles:true}));await settle();expect(root.querySelector<HTMLInputElement>('[aria-label="字段 1 类型"]')?.value).toBe('bigint');expect(root.querySelector<HTMLInputElement>('[aria-label="user_id长度"]')?.value).toBe('20')
  await input('字段 1 类型','varchar');await input('user_id长度','32');await input('字段 1 名称','price');root.querySelector('[aria-label="字段 1 名称"]')!.dispatchEvent(new Event('change',{bubbles:true}));await settle();expect(root.querySelector<HTMLInputElement>('[aria-label="字段 1 类型"]')?.value).toBe('varchar');expect(root.querySelector<HTMLInputElement>('[aria-label="price长度"]')?.value).toBe('32')
  app?.unmount();ddl='CREATE TABLE `items` (`title` varchar(80))';await mount('items');await input('字段 1 名称','id');root.querySelector('[aria-label="字段 1 名称"]')!.dispatchEvent(new Event('change',{bubbles:true}));await settle();expect(root.querySelector<HTMLInputElement>('[aria-label="字段 1 类型"]')?.value).toBe('varchar');expect(root.querySelector<HTMLInputElement>('[aria-label="id长度"]')?.value).toBe('80')
 })
 it('requests the table name on save and continues saving on the current tab after naming',async()=>{
  await mount();await input('字段 1 名称','title');await click('保存');expect(namePrompts).toHaveLength(1);expect(commands.some(sql=>sql.includes('CREATE TABLE'))).toBe(false);expect(details).toHaveLength(0)
  await namePrompts[0]!.save('named');await settle();expect(saved).toEqual(['named']);expect(commands.some(sql=>sql.includes('CREATE TABLE `qa`.`named`'))).toBe(true);expect(root.querySelector('.designer-tabs button.active')?.textContent).toBe('字段')
 })
 it('toggles the primary key from the whole cell and button without cancellation confirmations',async()=>{
  await mount();await input('字段 1 名称','id');const cell=root.querySelector<HTMLElement>('.designer-key-cell')!,key=cell.querySelector('button')!
  cell.click();await settle();expect(key.getAttribute('aria-pressed')).toBe('true');cell.click();await settle();expect(key.getAttribute('aria-pressed')).toBe('false');expect(confirmations).toHaveLength(0)
  key.click();await settle();expect(key.getAttribute('aria-pressed')).toBe('true');key.click();await settle();expect(key.getAttribute('aria-pressed')).toBe('false');expect(confirmations).toHaveLength(0)
 })
 it('applies selected index columns only on confirmation and protects an intervening edit',async()=>{
  await mount();await input('字段 1 名称','title');await click('索引');await click('添加索引');const open=()=>{root.querySelector<HTMLButtonElement>('[aria-label="选择索引字段 idx_1"]')!.click()}
  open();await settle();expect(pickers[0]?.fields.map(field=>field.name)).toEqual(['title']);expect(root.querySelector<HTMLInputElement>('[aria-label="索引字段"]')?.value).toBe('')
  pickers[0]!.apply('`title`(12) DESC');await settle();expect(root.querySelector<HTMLInputElement>('[aria-label="索引字段"]')?.value).toBe('`title`(12) DESC')
  open();await settle();await input('索引字段','title');pickers[1]!.apply('`title`(8) ASC');await settle();expect(root.querySelector<HTMLInputElement>('[aria-label="索引字段"]')?.value).toBe('title');expect(details.at(-1)?.details).toContain('已变化');expect(commands.some(sql=>sql.includes('CREATE TABLE'))).toBe(false)
 })
 it('automatically loads changed server structure, retains the prior draft SQL and does not execute it',async()=>{
  ddl="CREATE TABLE `items` (`title` varchar(80)) ENGINE=InnoDB";await mount('items');await input('title长度','120')
  ddl="CREATE TABLE `items` (`title` varchar(100), `remote` int) ENGINE=InnoDB";await click('保存')
  expect(root.textContent).toContain('已自动加载最新结构');expect(root.querySelector<HTMLInputElement>('[aria-label="title长度"]')?.value).toBe('100');expect(root.querySelectorAll('[aria-label="字段设计"] tbody tr')).toHaveLength(2)
  expect(commands.some(sql=>sql.includes('ALTER TABLE'))).toBe(false);expect(details).toHaveLength(0);expect(root.querySelector('.designer-tabs button.active')?.textContent).toBe('字段')
  await click('查看刷新前的草稿 SQL');expect(details[0]?.sql).toContain('VARCHAR(120)');expect(details[0]?.sql).toContain('ALTER TABLE')
 })
 it('saves through a changed table counter and keeps the current designer tab',async()=>{
  ddl="CREATE TABLE `items` (`title` varchar(80)) ENGINE=InnoDB AUTO_INCREMENT=12";await mount('items');await input('title长度','120');ddl=ddl.replace('AUTO_INCREMENT=12','AUTO_INCREMENT=13');await click('保存')
  expect(commands.some(sql=>sql.includes('ALTER TABLE'))).toBe(true);expect(root.textContent).toContain('表结构已保存');expect(details).toHaveLength(0);expect(root.querySelector('.designer-tabs button.active')?.textContent).toBe('字段')
 })
 it('opens error details without switching tabs for local and server syntax failures',async()=>{
  await mount();await input('字段 1 名称','title');await click('选项');await input('表名','items');await click('字段');await input('title长度','bad');await click('保存')
  expect(details[0]?.details).toContain('字段长度');expect(root.querySelector('.designer-tabs button.active')?.textContent).toBe('字段');expect(commands.some(sql=>sql.includes('CREATE TABLE'))).toBe(false)
  await input('title长度','80');failure="第 2 条语句失败：Error 1064 (42000): syntax error near 'test' at line 2";await click('保存')
  expect(details[1]?.details).toContain(failure);expect(details[1]?.sql).toContain('CREATE TABLE');expect(root.querySelector('.designer-tabs button.active')?.textContent).toBe('字段');expect(root.querySelector<HTMLInputElement>('[aria-label="title长度"]')?.value).toBe('80');expect(button('保存').disabled).toBe(false)
 })
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
