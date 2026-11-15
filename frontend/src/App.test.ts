import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, nextTick, type App as VueApp } from 'vue'
import App from './App.vue'
import { defaults, type Profile } from './api'

let app:VueApp, root:HTMLDivElement
let records:Profile[]
let bridge:Record<string,ReturnType<typeof vi.fn>>
const savedProfile:Profile={...defaults(),id:'test-profile',revision:1,name:'开发服务器',hasPassword:true,createdAt:'2026-11-15T00:00:00Z',updatedAt:'2026-11-15T00:00:00Z'}
async function settle(){await Promise.resolve();await nextTick();await Promise.resolve();await nextTick()}
function button(text:string,scope:ParentNode=root){const found=Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find(b=>b.textContent?.trim()===text);if(!found)throw new Error('Button not found: '+text);return found}
async function fill(selector:string,value:string){const input=root.querySelector<HTMLInputElement>(selector);if(!input)throw new Error('Input not found: '+selector);input.value=value;input.dispatchEvent(new Event('input',{bubbles:true}));await settle()}
async function mount(){app=createApp(App);app.mount(root);await settle()}

beforeEach(()=>{
 records=[]
 bridge={ListConnections:vi.fn(async()=>records),ListSessions:vi.fn(async()=>[]),ConfigPath:vi.fn(async()=>'test-config.json'),SaveConnection:vi.fn(async(input)=>{const p={...savedProfile,...input,id:'test-profile',revision:1};delete p.password;records=[p];return p}),DeleteConnection:vi.fn(async()=>{}),TestConnection:vi.fn(async()=>({success:true,message:'ok',serverVersion:'test',latencyMs:1})),Connect:vi.fn(),Disconnect:vi.fn(),RefreshDatabases:vi.fn(),SelectCAFile:vi.fn()}
 window.go={main:{App:bridge as never}}
 root=document.createElement('div');document.body.append(root)
})
afterEach(()=>{app?.unmount();root.remove();delete window.go;vi.restoreAllMocks()})

describe('connection management workflows',()=>{
 it('saves a connection without sending output timestamps through the Go bridge',async()=>{
  await mount();button('新建 MySQL 连接').click();await settle();await fill('#connection-name','我的数据库');button('保存连接').click();await settle()
  expect(bridge.SaveConnection).toHaveBeenCalledOnce()
  const request=bridge.SaveConnection.mock.calls[0][0]
  expect(request).not.toHaveProperty('createdAt');expect(request).not.toHaveProperty('updatedAt')
  expect(request.host).toBe('127.0.0.1');expect(root.querySelector('.connection-copy')?.textContent).toContain('我的数据库')
  expect(root.querySelector('.connection-form')).toBeNull()
 })
 it('asks before discarding edits and permits keeping the draft',async()=>{
  await mount();button('新建 MySQL 连接').click();await settle();await fill('#connection-name','还未保存');button('取消').click();await settle()
  const dialog=root.querySelector('[role="alertdialog"]')!;expect(dialog).not.toBeNull();button('取消',dialog).click();await settle()
  expect((root.querySelector('#connection-name') as HTMLInputElement).value).toBe('还未保存')
  button('取消').click();await settle();button('放弃修改',root.querySelector('[role="alertdialog"]')!).click();await settle()
  expect(root.textContent).toContain('从一个连接开始');expect(bridge.SaveConnection).not.toHaveBeenCalled()
 })
 it('duplicates connection settings without copying a stored password or identity',async()=>{
  records=[savedProfile];await mount();(root.querySelector('.connection-item') as HTMLButtonElement).click();await settle();button('复制配置').click();await settle()
  const password=root.querySelector<HTMLInputElement>('input[autocomplete="new-password"]')!
  expect(password.value).toBe('');expect(password.placeholder).toBe('输入数据库密码')
  button('保存连接').click();await settle();const request=bridge.SaveConnection.mock.calls[0][0]
  expect(request.id).toBe('');expect(request.passwordAction).toBe('replace');expect(request.password).toBe('')
 })
 it('displays a failed test without marking the connection as connected',async()=>{
  bridge.TestConnection.mockRejectedValue(new Error('MySQL 拒绝登录，请检查用户名、密码'))
  await mount();button('新建 MySQL 连接').click();await settle();button('测试连接').click();await settle()
  expect(root.querySelector('[role="alert"]')?.textContent).toContain('MySQL 拒绝登录')
  expect(root.textContent).not.toContain('连接测试成功');expect(bridge.Connect).not.toHaveBeenCalled()
 })
})
