import {createApp,nextTick,type App} from 'vue'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import SqlConsole from './SqlConsole.vue'
import {defaults} from './api'
let app:App|undefined,root:HTMLDivElement,admin:ReturnType<typeof vi.fn>,cancel:ReturnType<typeof vi.fn>
async function settle(){for(let i=0;i<12;i++){await nextTick();await Promise.resolve()}}
const input=()=>root.querySelector<HTMLTextAreaElement>('[aria-label="SQL 命令"]')!
async function type(sql:string){input().value=sql;input().dispatchEvent(new Event('input',{bubbles:true}));await settle()}
async function key(key:string,extra:KeyboardEventInit={}){input().dispatchEvent(new KeyboardEvent('keydown',{key,bubbles:true,cancelable:true,...extra}));await settle()}
beforeEach(()=>{root=document.createElement('div');document.body.append(root);cancel=vi.fn(async()=>{});admin=vi.fn(async(_task,_id,db)=>({completed:1,error:'',text:'',results:[{columns:['DATABASE()'],rows:[[db]],affected:'0',truncated:false}]}));window.go={main:{App:{DatabaseAdmin:admin,CancelDatabaseTask:cancel} as never}}})
afterEach(()=>{app?.unmount();root.remove();delete window.go})
async function mount(){const dirty=vi.fn(),close=vi.fn(),change=vi.fn();app=createApp(SqlConsole,{profile:{...defaults(),id:'qa'},database:'selected_db',active:true,connected:true,sessionKey:'qa',onDirty:dirty,onClose:close,onDatabaseChange:change});app.mount(root);await settle();return {dirty,close,change}}
it('starts as an inline mysql prompt and executes in the selected database, never on mount',async()=>{
 const {dirty}=await mount();expect(root.querySelector('[role="dialog"]')).toBeNull();expect(root.textContent).toContain('mysql>');expect(root.textContent).toContain('selected_db');expect(admin).not.toHaveBeenCalled();await type('SELECT DATABASE();');expect(dirty).toHaveBeenLastCalledWith(true);await key('Enter');expect(admin).toHaveBeenCalledWith(expect.any(String),'qa','selected_db',{operation:'query',sql:'SELECT DATABASE();'});expect(root.textContent).toMatch(/1 row\(s\) in set \(\d+\.\d{2} sec\)/);expect(input().value).toBe('');expect(dirty).toHaveBeenLastCalledWith(false)
})
it('keeps history and unfinished input in memory and waits for a complete command',async()=>{
 await mount();await type('SELECT 1');await key('Enter');expect(admin).not.toHaveBeenCalled();await key('Enter',{ctrlKey:true});await type('draft');await key('ArrowUp');expect(input().value).toBe('SELECT 1');await key('ArrowDown');expect(input().value).toBe('draft');input().setSelectionRange(0,5);const copy=new KeyboardEvent('keydown',{key:'c',ctrlKey:true,bubbles:true,cancelable:true});input().dispatchEvent(copy);await settle();expect(copy.defaultPrevented).toBe(false);expect(input().value).toBe('draft')
})
it('changes the default database only after USE succeeds, and retains it after a rejected switch',async()=>{
 const {change}=await mount();await type('USE `other_db`;');await key('Enter');expect(admin).toHaveBeenLastCalledWith(expect.any(String),'qa','other_db',{operation:'query',sql:'USE `other_db`;'});expect(change).toHaveBeenCalledWith('other_db');await type('SELECT DATABASE();');await key('Enter');expect(admin.mock.calls.at(-1)?.[2]).toBe('other_db')
 admin.mockResolvedValueOnce({completed:0,error:'Unknown database',results:[]});await type('USE missing;');await key('Enter');expect(root.querySelector('[role="alert"]')?.textContent).toContain('Unknown database');await type('SELECT 1;');await key('Enter');expect(admin.mock.calls.at(-1)?.[2]).toBe('other_db');expect(change).toHaveBeenCalledTimes(1)
})
it('handles help, clear and exit locally without database requests',async()=>{
 const {close}=await mount();await type('help');await key('Enter');expect(root.textContent).toContain('独立会话');await type('clear');await key('Enter');expect(root.querySelectorAll('pre')).toHaveLength(0);await type('exit');await key('Enter');expect(close).toHaveBeenCalledOnce();expect(admin).not.toHaveBeenCalled()
})
it('cancels an owned request when the console unmounts and ignores its late response',async()=>{
 let finish!:(response:unknown)=>void;admin.mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve}));await mount();await type('SELECT 1;');await key('Enter');expect(input().readOnly).toBe(true);const task=admin.mock.calls[0]![0];app!.unmount();app=undefined;expect(cancel).toHaveBeenCalledWith(task);finish({completed:1,error:'',results:[]});await settle();expect(root.textContent).toBe('')
})
