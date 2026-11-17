import {afterEach,beforeEach,expect,it} from 'vitest'
import {createApp,nextTick,type App} from 'vue'
import IndexFieldsDialog,{type IndexFieldsPayload} from './IndexFieldsDialog.vue'
let app:App|undefined,root:HTMLDivElement,applied:string[],cancelled:number
const fields=[{name:'id',type:'int',length:'11'},{name:'title',type:'varchar',length:'80'}]
async function settle(){await nextTick();await Promise.resolve();await nextTick()}
async function mount(value:string,extra:Partial<IndexFieldsPayload>={}){app=createApp(IndexFieldsDialog,{fields,value,kind:'NORMAL',method:'',sqlMode:'',...extra,onApply:(value:string)=>applied.push(value),onCancel:()=>cancelled++});app.mount(root);await settle()}
function button(label:string){return Array.from(root.querySelectorAll<HTMLButtonElement>('button')).find(button=>button.textContent?.trim()===label||button.getAttribute('aria-label')===label)!}
beforeEach(()=>{root=document.createElement('div');document.body.append(root);applied=[];cancelled=0})
afterEach(()=>{app?.unmount();root.remove()})
it('restores selected order, edits a prefix and moves a selected field before applying',async()=>{
 await mount('`title`(12) DESC, `id` ASC');expect(root.querySelectorAll('tbody tr')[0]?.textContent).toContain('title')
 const prefix=root.querySelector<HTMLInputElement>('[aria-label="title子部分"]')!;prefix.value='8';prefix.dispatchEvent(new Event('input',{bubbles:true}));root.querySelector<HTMLInputElement>('[aria-label="选择字段 id"]')!.focus();await settle();button('上移所选字段').click();await settle();button('确定').click();expect(applied).toEqual(['`id` ASC, `title`(8) DESC'])
})
it('keeps the original value unchanged on cancel and supports select all and clear',async()=>{
 await mount('id');button('选择全部字段').click();await settle();expect(root.querySelectorAll('input[type=checkbox]:checked')).toHaveLength(2);button('清空字段选择').click();await settle();expect(button('确定').disabled).toBe(true);button('取消').click();expect(cancelled).toBe(1);expect(applied).toEqual([])
})
it('does not discard missing fields and reports invalid numeric prefixes',async()=>{
 await mount('`gone`, `id`(4)');button('确定').click();await settle();expect(root.querySelector('[role=alert]')?.textContent).toContain('已不存在');root.querySelector<HTMLInputElement>('[aria-label="选择字段 gone"]')!.click();await settle();button('确定').click();await settle();expect(root.querySelector('[role=alert]')?.textContent).toContain('不支持子部分');expect(applied).toEqual([])
})
it('disables prefix and order for full-text indexes and keeps expressions out of the picker',async()=>{
 await mount('title',{kind:'FULLTEXT'});expect(root.querySelector<HTMLInputElement>('[aria-label="title子部分"]')?.disabled).toBe(true);expect(root.querySelector<HTMLInputElement>('[aria-label="title排序顺序"]')?.disabled).toBe(true);button('确定').click();expect(applied).toEqual(['`title`'])
 app?.unmount();await mount('(lower(title))');expect(button('确定').disabled).toBe(true);expect(root.querySelector('[role=alert]')?.textContent).toContain('表达式')
})
