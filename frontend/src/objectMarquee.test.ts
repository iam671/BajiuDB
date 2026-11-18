import {createApp,h,nextTick,ref,type App,type Ref} from 'vue'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import {useObjectMarquee} from './objectMarquee'
let app:App,host:HTMLElement,root:HTMLElement,selection:Set<string>,scope:Ref<string>,rectangle:ReturnType<typeof useObjectMarquee>,top:number
async function settle(){await nextTick();await Promise.resolve()}
function pointer(target:EventTarget,type:string,x:number,y:number,extra:MouseEventInit={}){const event=new MouseEvent(type,{button:0,clientX:x,clientY:y,bubbles:true,cancelable:true,...extra});Object.defineProperties(event,{pointerId:{value:7},pointerType:{value:'mouse'}});target.dispatchEvent(event)}
async function move(x:number,y:number){pointer(document,'pointermove',x,y);await vi.advanceTimersByTimeAsync(16);await settle()}
beforeEach(async()=>{
 vi.useFakeTimers();vi.stubGlobal('requestAnimationFrame',(fn:FrameRequestCallback)=>setTimeout(()=>fn(performance.now()),16));vi.stubGlobal('cancelAnimationFrame',(id:number)=>clearTimeout(id))
 selection=new Set();scope=ref('first');top=0;root=document.createElement('div');host=document.createElement('div');document.body.append(root,host)
 root.innerHTML='<table><thead><tr><th>名称</th></tr></thead><tbody>'+Array.from({length:6},(_,i)=>'<tr data-object-key="item'+i+'"><td>item'+i+'</td></tr>').join('')+'</tbody></table>'
 root.getBoundingClientRect=()=>new DOMRect(100,100,400,300);root.querySelector('thead')!.getBoundingClientRect=()=>new DOMRect(100,100,400,20)
 Object.defineProperties(root,{clientWidth:{value:400},clientHeight:{value:300},scrollTop:{get:()=>top,set:value=>{top=Math.max(0,Math.min(300,value))}}})
 root.querySelectorAll<HTMLElement>('[data-object-key]').forEach((node,i)=>{node.getBoundingClientRect=()=>new DOMRect(100,120+i*30-top,350,30)})
 const viewport=ref<HTMLElement>();app=createApp({setup(){rectangle=useObjectMarquee({viewport,enabled:()=>true,scope:()=>scope.value,selection:()=>selection,apply:keys=>{selection=new Set(keys)}});return()=>h('span')}});app.mount(host);viewport.value=root;await settle()
})
afterEach(()=>{app.unmount();root.remove();host.remove();vi.useRealTimers();vi.unstubAllGlobals()})
it('selects intersecting rows in either direction and consumes the release click',async()=>{
 pointer(root,'pointerdown',480,170);await move(120,125);expect([...selection]).toEqual(['item0','item1']);expect(rectangle.value?.width).toBe(360);pointer(document,'pointerup',120,125);root.dispatchEvent(new MouseEvent('click',{detail:1,clientX:120,clientY:125,bubbles:true}));await vi.advanceTimersByTimeAsync(32);expect([...selection]).toEqual(['item0','item1']);expect(rectangle.value).toBeNull();const accidentalOpen=vi.fn();const row=root.querySelector('[data-object-key]')!;row.addEventListener('dblclick',accidentalOpen);row.dispatchEvent(new MouseEvent('dblclick',{bubbles:true,cancelable:true}));expect(accidentalOpen).not.toHaveBeenCalled()
 pointer(root,'pointerdown',480,270);pointer(document,'pointerup',480,270);root.dispatchEvent(new MouseEvent('click',{detail:1,clientX:480,clientY:270,bubbles:true}));expect(selection.size).toBe(0)
})
it('adds/toggles from the initial set and restores it on Escape',async()=>{
 selection=new Set(['item0','item5']);pointer(root,'pointerdown',480,170,{ctrlKey:true});await move(120,125);expect([...selection].sort()).toEqual(['item1','item5']);document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}));await vi.advanceTimersByTimeAsync(32);expect([...selection]).toEqual(['item0','item5']);expect(rectangle.value).toBeNull()
})
it('never restores stale names after a database/filter scope change',async()=>{
 selection=new Set(['item5']);pointer(root,'pointerdown',480,170);await move(120,125);scope.value='second';selection.clear();await settle();await vi.advanceTimersByTimeAsync(32);expect(selection.size).toBe(0);expect(rectangle.value).toBeNull()
})
it('auto-scrolls near the edge and stops every animation when released',async()=>{
 pointer(root,'pointerdown',200,150);await move(200,430);expect(top).toBeGreaterThan(0);await vi.advanceTimersByTimeAsync(32);pointer(document,'pointerup',200,430);const finished=top;await vi.advanceTimersByTimeAsync(100);expect(top).toBe(finished);expect(rectangle.value).toBeNull()
})
it('leaves ordinary clicks and column headers to their existing handlers',async()=>{
 const row=root.querySelector<HTMLElement>('[data-object-key]')!;const click=vi.fn();row.addEventListener('click',click);pointer(row,'pointerdown',130,130);pointer(document,'pointerup',130,130);row.click();expect(click).toHaveBeenCalledOnce();expect(selection.size).toBe(0)
 pointer(root.querySelector('th')!,'pointerdown',130,110);await move(150,230);expect(rectangle.value).toBeNull();expect(selection.size).toBe(0)
})
