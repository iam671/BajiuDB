import {afterEach,expect,it} from 'vitest'
import {createApp,nextTick,type App} from 'vue'
import TableNameDialog from './TableNameDialog.vue'
let app:App|undefined
afterEach(()=>{app?.unmount();document.body.innerHTML=''})
it('validates the name and submits once while cancellation never saves',async()=>{
 const root=document.createElement('div');document.body.append(root);const saved:string[]=[];let cancelled=0
 app=createApp(TableNameDialog,{value:'',onSave:(name:string)=>saved.push(name),onCancel:()=>cancelled++});app.mount(root);await nextTick()
 const form=root.querySelector('form')!,input=root.querySelector('input')!;form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await nextTick();expect(root.querySelector('[role=alert]')?.textContent).toContain('1–64');expect(saved).toEqual([])
 input.value=' notes ';input.dispatchEvent(new Event('input',{bubbles:true}));await nextTick();form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));expect(saved).toEqual(['notes'])
 ;(Array.from(root.querySelectorAll('button')).find(button=>button.textContent==='取消') as HTMLButtonElement).click();expect(cancelled).toBe(1);expect(saved).toHaveLength(1)
})
