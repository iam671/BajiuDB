import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {installTableResizing} from './tableResizing'
let cleanup:(()=>void)|undefined
beforeEach(()=>{
 localStorage.clear();document.body.innerHTML='<table aria-label="fixture"><thead><tr><th>Name</th><th>Type</th></tr></thead><tbody><tr><td>value</td><td>int</td></tr></tbody></table>'
 vi.stubGlobal('ResizeObserver',class {observe(){}disconnect(){}})
 vi.spyOn(Element.prototype,'getClientRects').mockReturnValue([{}] as unknown as DOMRectList)
 vi.spyOn(Element.prototype,'getBoundingClientRect').mockImplementation(()=>({width:120,height:28,x:0,y:0,left:0,right:120,top:0,bottom:28,toJSON:()=>({})}))
})
afterEach(()=>{cleanup?.();cleanup=undefined;document.body.innerHTML='';vi.restoreAllMocks();vi.unstubAllGlobals()})
describe('table column resizing',()=>{
 it('resizes through the keyboard, persists only layout and restores it on remount',()=>{
  cleanup=installTableResizing();const handle=document.querySelector<HTMLElement>('[data-table-resize-handle]')!;handle.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));expect(handle.getAttribute('aria-valuenow')).toBe('128');expect(document.querySelector('col')?.style.width).toBe('128px');const saved=JSON.parse(localStorage.getItem(localStorage.key(0)!)!);expect(saved).toEqual({version:1,widths:[128,120]});cleanup();cleanup=installTableResizing();expect(document.querySelector('[data-table-resize-handle]')?.getAttribute('aria-valuenow')).toBe('128')
 })
 it('supports pointer dragging without sorting or retaining document drag state after disposal',()=>{
  cleanup=installTableResizing();const head=document.querySelector('th')!,click=vi.fn();head.addEventListener('click',click);const handle=head.querySelector<HTMLElement>('[data-table-resize-handle]')!;handle.click();expect(click).not.toHaveBeenCalled();handle.dispatchEvent(new MouseEvent('pointerdown',{button:0,clientX:100,bubbles:true}));document.dispatchEvent(new MouseEvent('pointermove',{clientX:180,bubbles:true}));expect(handle.getAttribute('aria-valuenow')).toBe('200');cleanup();cleanup=undefined;expect(document.body.style.cursor).toBe('');expect(document.querySelector('[data-table-resize-handle]')).toBeNull()
 })
 it('does not add duplicate handles or replace the data grid column layout',()=>{
  document.body.innerHTML+='<div class="table-data-grid"><table><thead><tr><th>id<span class="table-column-resizer"></span></th></tr></thead></table></div>';cleanup=installTableResizing();expect(document.querySelectorAll('[data-table-resize-handle]')).toHaveLength(2);expect(document.querySelectorAll('.table-column-resizer')).toHaveLength(1);expect(document.querySelector('.table-data-grid colgroup')).toBeNull()
 })
})
