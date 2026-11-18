import {createApp,nextTick,type App} from 'vue'
import {afterEach,expect,it,vi} from 'vitest'
import CellValueDialog from './CellValueDialog.vue'
import type {TableColumn,CellDraft} from './tableEditing'
let app:App|undefined,root:HTMLDivElement
const column=(type:string):TableColumn=>({name:'value',type,nullable:true,key:'',defaultValue:null,extra:'',comment:'',generated:false,binary:false,spatial:false})
async function settle(){for(let i=0;i<6;i++){await nextTick();await Promise.resolve()}}
async function mount(type:string,value:string){root=document.createElement('div');document.body.append(root);const apply=vi.fn(),cancel=vi.fn();app=createApp(CellValueDialog,{column:column(type),cell:{mode:'value',value},isNew:false,onApply:apply,onCancel:cancel});app.mount(root);await settle();return {apply,cancel}}
function button(text:string){return Array.from(root.querySelectorAll<HTMLButtonElement>('button')).find(b=>b.textContent===text)!}
afterEach(()=>{app?.unmount();root.remove()})
it('stages calendar changes until confirm and keeps a cancelled date unchanged',async()=>{
 const {apply,cancel}=await mount('date','2026-11-18');root.querySelector<HTMLButtonElement>('[aria-label="2026-11-19"]')!.click();await settle();expect(apply).not.toHaveBeenCalled();button('取消').click();expect(cancel).toHaveBeenCalledOnce();expect(apply).not.toHaveBeenCalled();button('确定').click();expect(apply).toHaveBeenCalledWith({mode:'value',value:'2026-11-19'})
})
it('preserves fractional seconds when choosing a new day and reports invalid dates',async()=>{
 const {apply}=await mount('datetime(6)','2026-11-18 08:09:10.123456');root.querySelector<HTMLButtonElement>('[aria-label="2026-11-20"]')!.click();await settle();button('确定').click();expect(apply).toHaveBeenLastCalledWith({mode:'value',value:'2026-11-20 08:09:10.123456'} satisfies CellDraft)
 const input=root.querySelector<HTMLInputElement>('[aria-label="日期值"]')!;input.value='2026-02-30';input.dispatchEvent(new Event('input',{bubbles:true}));await settle();button('确定').click();await settle();expect(apply).toHaveBeenCalledTimes(1);expect(root.querySelector('[role="alert"]')?.textContent).toContain('有效日期时间')
})
it('provides editable searchable enum options and exact numeric shortcuts',async()=>{
 const {apply}=await mount("enum('draft','active')",'draft');root.querySelector<HTMLInputElement>('[role="combobox"]')!.focus();await settle();const input=root.querySelector<HTMLInputElement>('[role="combobox"]')!;input.value='active';input.dispatchEvent(new Event('input',{bubbles:true}));await settle();button('确定').click();expect(apply).toHaveBeenCalledWith({mode:'value',value:'active'})
 app!.unmount();root.remove();const numeric=await mount('decimal(30,2)','999999999999999999.99');button('+1').click();await settle();button('确定').click();expect(numeric.apply).toHaveBeenCalledWith({mode:'value',value:'1000000000000000000.99'})
})
