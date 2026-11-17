import {afterEach,describe,expect,it} from 'vitest'
import {createApp,nextTick,ref,type App} from 'vue'
import SearchableSelect from './SearchableSelect.vue'
let app:App|undefined
async function settle(){await nextTick();await Promise.resolve();await nextTick()}
afterEach(()=>{app?.unmount();app=undefined;document.body.innerHTML=''})
describe('editable matching dropdown',()=>{
 it('positions options relative to a transformed draggable dialog',async()=>{
  const root=document.createElement('div');root.style.transform='translate(0px, 0px)';document.body.append(root)
  app=createApp(SearchableSelect,{id:'drag-options',label:'Order',modelValue:'',options:['ASC','DESC']});app.mount(root)
  root.getBoundingClientRect=()=>({left:100,top:70,right:500,bottom:400,width:400,height:330,x:100,y:70,toJSON(){}})
  root.querySelector<HTMLElement>('.searchable-select-input')!.getBoundingClientRect=()=>({left:130,top:110,right:330,bottom:138,width:200,height:28,x:130,y:110,toJSON(){}})
  root.querySelector('input')!.focus();await settle();const list=root.querySelector('ul')!
  expect(list.style.left).toBe('30px');expect(list.style.top).toBe('70px');expect(list.style.width).toBe('200px')
 })
 it('filters labels while keeping coded values valid and commits matching selection',async()=>{
  const value=ref('none');const root=document.createElement('div');document.body.append(root);app=createApp({components:{SearchableSelect},setup:()=>({value}),template:'<SearchableSelect id="qa" label="Mode" v-model="value" :allowCustom="false" :options="[{value:\'none\',label:\'No default\'},{value:\'expression\',label:\'SQL expression\'}]" />'});app.mount(root);const input=root.querySelector('input')!;input.focus();input.value='SQL';input.dispatchEvent(new Event('input',{bubbles:true}));await settle();expect(value.value).toBe('none');expect(root.querySelectorAll('[role=option]')).toHaveLength(1);input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));await settle();expect(value.value).toBe('expression');expect(input.value).toBe('SQL expression')
 })
 it('preserves free text for charset-style inputs and closes on outside right clicks',async()=>{
  const value=ref('utf8mb4');const root=document.createElement('div');document.body.append(root);app=createApp({components:{SearchableSelect},setup:()=>({value}),template:'<SearchableSelect id="qa" label="Charset" v-model="value" :options="[\'utf8mb4\',\'latin1\']" />'});app.mount(root);const input=root.querySelector('input')!;input.focus();input.value='custom';input.dispatchEvent(new Event('input',{bubbles:true}));await settle();expect(value.value).toBe('custom');expect(input.getAttribute('aria-expanded')).toBe('true');document.body.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true}));await settle();expect(input.getAttribute('aria-expanded')).toBe('false');expect(input.value).toBe('custom')
 })
})
