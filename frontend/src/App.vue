<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { Plus, Search, Database, Server, ChevronRight, ChevronDown, Plug, Unplug, Play, Pencil, Trash2, Copy, Save, X, Check, CheckCircle2, AlertCircle, ShieldCheck, FolderOpen, SlidersHorizontal, RefreshCw, KeyRound, Clock3, Cable, MoreHorizontal, LoaderCircle } from '@lucide/vue'
import { api, defaults, fromProfile, errorText, isDesktop, type Profile, type SessionInfo, type TestResult } from './api'

const profiles = ref<Profile[]>([])
const sessions = reactive<Record<string,SessionInfo>>({})
const selectedID = ref('')
const mode = ref<'welcome'|'editor'|'overview'>('welcome')
const form = reactive(defaults())
const baseline = ref(JSON.stringify(form))
const search = ref('')
const group = ref('all')
const busy = ref('')
const loading = ref(true)
const fatalError = ref('')
const message = ref('')
const messageKind = ref('success')
const configPath = ref('')
const showPassword = ref(false)
const advanced = ref(false)
const testResult = ref<TestResult|null>(null)
const testError = ref('')
const activeMenu = ref(false)
const dbSearch = ref('')
const errors = reactive<Record<string,string>>({})
const desktop = ref(isDesktop())
const modal = reactive({open:false,title:'',body:'',label:'',danger:false,action:null as null|(()=>void|Promise<void>)})
const selected = computed(()=>profiles.value.find(p=>p.id===selectedID.value))
const live = computed(()=>sessions[selectedID.value])
const dirty = computed(()=>JSON.stringify(form)!==baseline.value)
const groups = [{id:'all',label:'全部连接'},{id:'local',label:'本地'},{id:'development',label:'开发'},{id:'staging',label:'测试'},{id:'production',label:'生产'}]
const filtered = computed(()=>profiles.value.filter(p=>(group.value==='all'||p.environment===group.value)&&`${p.name} ${p.host} ${p.username}`.toLowerCase().includes(search.value.toLowerCase())))
const databases = computed(()=>live.value?.databases.filter(n=>n.toLowerCase().includes(dbSearch.value.toLowerCase()))??[])
const connectedCount = computed(()=>Object.keys(sessions).length)
const envName = (id:string)=>groups.find(g=>g.id===id)?.label??'未分组'
const address = (p:Profile)=>`${p.host.includes(':')?'['+p.host+']':p.host}:${p.port}`
const dateText = (s:string)=>s?new Date(s).toLocaleString('zh-CN',{hour12:false}):'—'
let timer:ReturnType<typeof setTimeout>|undefined
let poll:ReturnType<typeof setInterval>|undefined

function notice(text:string,kind='success'){message.value=text;messageKind.value=kind;clearTimeout(timer);timer=setTimeout(()=>message.value='',5500)}
function clearFeedback(){testResult.value=null;testError.value='';for(const key of Object.keys(errors))delete errors[key]}
function assignProfile(p?:Profile){Object.assign(form,p?fromProfile(p):defaults());baseline.value=JSON.stringify(form);showPassword.value=false;clearFeedback()}
function confirmAction(title:string,body:string,label:string,action:()=>void|Promise<void>,danger=false){Object.assign(modal,{open:true,title,body,label,action,danger});nextTick(()=>document.querySelector<HTMLButtonElement>('[data-modal-cancel]')?.focus())}
async function acceptModal(){const action=modal.action;modal.open=false;modal.action=null;await action?.()}
function navigate(action:()=>void){activeMenu.value=false;if(mode.value==='editor'&&dirty.value){confirmAction('放弃未保存的修改？','当前连接的修改尚未保存。放弃后将恢复到上次保存的配置。','放弃修改',action,true)}else action()}
function create(){navigate(()=>{selectedID.value='';assignProfile();mode.value='editor';advanced.value=false;nextTick(()=>document.getElementById('connection-name')?.focus())})}
function select(p:Profile){if(selectedID.value===p.id)return;navigate(()=>{selectedID.value=p.id;assignProfile(p);mode.value='overview';dbSearch.value=''})}
function edit(){if(selected.value){assignProfile(selected.value);mode.value='editor';advanced.value=form.tlsMode==='verify'}}
function duplicate(){if(!selected.value)return;const p=selected.value;navigate(()=>{assignProfile();Object.assign(form,{...p,id:'',revision:0,name:p.name+' 副本',hasPassword:false,password:'',passwordAction:'replace',rememberPassword:true,createdAt:'',updatedAt:''});baseline.value=JSON.stringify(defaults());selectedID.value='';mode.value='editor';notice('已复制配置，请重新输入密码后保存')})}
function validate(requireName=true){for(const key of Object.keys(errors))delete errors[key];if(requireName&&!form.name.trim())errors.name='请输入连接名称';if(!form.host.trim())errors.host='请输入主机地址';if(!Number.isInteger(Number(form.port))||form.port<1||form.port>65535)errors.port='端口范围为 1–65535';if(!form.username.trim())errors.username='请输入用户名';return Object.keys(errors).length===0}
function payload(){const {createdAt:_,updatedAt:__,hasPassword:___,...input}=form;return {...input,port:Number(form.port),timeout:Number(form.timeout)}}
async function save(stayInEditor=false){if(!validate()||busy.value)return false;busy.value='save';try{const password=form.password;const action=form.passwordAction;const remember=form.rememberPassword;const p=await api.SaveConnection(payload());delete sessions[p.id];profiles.value=await api.ListConnections();selectedID.value=p.id;assignProfile(p);if(!remember&&action==='replace'){form.password=password;form.passwordAction='replace';baseline.value=JSON.stringify(form)}mode.value=stayInEditor?'editor':'overview';notice('连接已保存');return true}catch(e){notice(errorText(e),'error');return false}finally{busy.value=''}}
async function connect(){if(busy.value)return;if(mode.value==='editor'){if(!await save(true))return}if(!selectedID.value)return;busy.value='connect';testError.value='';try{sessions[selectedID.value]=await api.Connect(payload());mode.value='overview';notice('已连接到 '+(selected.value?.name??'MySQL'))}catch(e){testError.value=errorText(e);notice(testError.value,'error')}finally{busy.value=''}}
async function test(){if(!validate(false)||busy.value)return;busy.value='test';testResult.value=null;testError.value='';try{testResult.value=await api.TestConnection(payload())}catch(e){testError.value=errorText(e)}finally{busy.value=''}}
async function disconnect(){if(busy.value)return;busy.value='disconnect';try{await api.Disconnect(selectedID.value);delete sessions[selectedID.value];notice('连接已断开')}catch(e){notice(errorText(e),'error')}finally{busy.value=''}}
async function refresh(){if(busy.value||!live.value)return;busy.value='refresh';try{sessions[selectedID.value]=await api.RefreshDatabases(selectedID.value)}catch(e){delete sessions[selectedID.value];notice(errorText(e),'error')}finally{busy.value=''}}
function remove(){const p=selected.value;if(!p)return;confirmAction('删除“'+p.name+'”？','将删除此设备上的连接配置和保存的密码，不会删除服务器上的任何数据库。','删除连接',async()=>{busy.value='delete';try{await api.DeleteConnection(p.id,p.revision);delete sessions[p.id];profiles.value=await api.ListConnections();selectedID.value='';assignProfile();mode.value='welcome';notice('连接已删除')}catch(e){notice(errorText(e),'error')}finally{busy.value=''}},true)}
function cancelEdit(){navigate(()=>{assignProfile(selected.value);mode.value=selected.value?'overview':'welcome'})}
async function chooseCA(){try{const path=await api.SelectCAFile();if(path)form.caPath=path}catch(e){notice(errorText(e),'error')}}
function pwdChanged(){form.passwordAction='replace';clearFeedback()}
function forgetPassword(){form.password='';form.passwordAction='forget';form.rememberPassword=false;form.hasPassword=false}
async function copyAddress(){try{await navigator.clipboard.writeText(address(form));notice('地址已复制')}catch{notice('无法访问剪贴板，可在连接设置中复制地址','error')}}
function shortcuts(e:KeyboardEvent){if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='n'){e.preventDefault();if(!busy.value)create()}if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='s'&&mode.value==='editor'){e.preventDefault();void save()}if(e.key==='Escape'){if(modal.open)modal.open=false;else activeMenu.value=false}}
function trapDialog(e:KeyboardEvent){const dialog=e.currentTarget as HTMLElement;const buttons=Array.from(dialog.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'));const first=buttons[0],last=buttons[buttons.length-1];if(e.shiftKey&&document.activeElement===first){e.preventDefault();last?.focus()}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first?.focus()}}
watch(()=>JSON.stringify(form),()=>{testResult.value=null;testError.value=''})
onMounted(async()=>{document.addEventListener('keydown',shortcuts);try{if(desktop.value){profiles.value=await api.ListConnections();configPath.value=await api.ConfigPath();for(const s of await api.ListSessions())sessions[s.id]=s}}catch(e){fatalError.value=errorText(e)}finally{loading.value=false}poll=setInterval(async()=>{if(!desktop.value||busy.value)return;try{const current=await api.ListSessions();const ids=new Set(current.map(s=>s.id));for(const id of Object.keys(sessions))if(!ids.has(id))delete sessions[id]}catch{/* the next explicit operation reports bridge errors */}},15000)})
onUnmounted(()=>{clearTimeout(timer);clearInterval(poll);document.removeEventListener('keydown',shortcuts)})
</script>

<template>
  <div class="app-shell" @click="activeMenu=false">
    <header class="app-header">
      <a class="brand" href="#" @click.prevent="navigate(()=>mode='welcome')" aria-label="Bajiu DB 首页"><span class="brand-mark"><span></span><span></span></span><span>Bajiu <b>DB</b><small>捌玖</small></span></a>
      <span class="header-divider"></span><span class="header-title">MySQL 连接管理</span>
      <div class="header-right"><span class="local-indicator"><span></span>本地工作空间</span><span class="version">v0.1.0</span></div>
    </header>
    <div v-if="!desktop" class="preview-banner">界面预览模式。连接测试、保存和数据库访问请在 Bajiu DB 桌面版中使用。</div>
    <div v-if="fatalError" class="fatal-banner" role="alert"><AlertCircle :size="17"/>{{fatalError}}。为保护原配置，当前无法保存连接。</div>
    <div class="workbench">
      <aside class="sidebar" aria-label="连接列表">
        <div class="sidebar-heading"><h2>连接</h2><span class="count">{{profiles.length}}</span><button class="icon-button sidebar-add" aria-label="新建连接" title="新建连接 Ctrl+N" @click="create" :disabled="!!busy||!!fatalError"><Plus :size="18"/></button></div>
        <div class="sidebar-search"><Search :size="15"/><input v-model="search" aria-label="搜索连接" placeholder="搜索名称或主机…"/><kbd>⌕</kbd></div>
        <div class="group-select"><SlidersHorizontal :size="14"/><select v-model="group" aria-label="按分组筛选"><option v-for="g in groups" :key="g.id" :value="g.id">{{g.label}}</option></select><ChevronDown :size="13"/></div>
        <div class="connection-list">
          <p v-if="loading" class="list-empty">正在读取连接…</p>
          <div v-else-if="profiles.length===0" class="sidebar-empty"><Server :size="25" stroke-width="1.4"/><p>还没有连接</p><span>添加你的第一个 MySQL 服务器</span></div>
          <p v-else-if="filtered.length===0" class="list-empty">没有匹配的连接</p>
          <button v-for="p in filtered" :key="p.id" class="connection-item" :class="{selected:selectedID===p.id}" @click="!busy&&select(p)" :disabled="!!busy">
            <span class="db-icon"><Database :size="19" stroke-width="1.6"/><i v-if="sessions[p.id]" class="online-dot"></i></span>
            <span class="connection-copy"><strong>{{p.name}}</strong><small>{{address(p)}}</small></span><span class="env-dot" :class="p.environment" :title="envName(p.environment)"></span>
          </button>
        </div>
        <div class="sidebar-bottom"><span class="status-dot" :class="{'is-online':connectedCount>0}"></span>{{connectedCount?connectedCount+' 个连接已打开':'暂无活动连接'}}<ShieldCheck :size="14" title="配置保存在本机"/></div>
      </aside>
      <main class="main-pane">
        <template v-if="mode==='welcome'">
          <div class="welcome-toolbar"><span>工作空间</span><span class="muted">MySQL</span></div>
          <section class="welcome">
            <div class="welcome-illustration" aria-hidden="true"><div class="server-glyph"><Database :size="58" stroke-width="1.15"/></div><div class="illustration-line"></div><span class="terminal-glyph"><span>›</span><span class="terminal-cursor"></span></span><i class="illustration-check"><Check :size="13"/></i></div>
            <h1>从一个连接开始</h1><p>把常用的 MySQL 服务器放在一起。<br/>清楚地管理每个环境，随时确认连接状态。</p>
            <button class="button primary welcome-create" @click="create" :disabled="!!fatalError"><Plus :size="17"/>新建 MySQL 连接</button>
            <span class="shortcut-hint">也可以按 <kbd>Ctrl</kbd> + <kbd>N</kbd></span>
            <div class="welcome-features"><div><Server :size="18"/><strong>分环境管理</strong><span>本地、开发、测试和生产</span></div><div><KeyRound :size="18"/><strong>系统凭据保护</strong><span>密码不写入连接配置</span></div><div><Plug :size="18"/><strong>真实连接测试</strong><span>检查登录与数据库访问</span></div></div>
          </section>
          <footer class="welcome-footer"><ShieldCheck :size="15"/>连接配置保存在此设备，不上传到云端。</footer>
        </template>
        <template v-else>
          <div class="connection-toolbar"><div class="breadcrumb"><span>连接</span><ChevronRight :size="14"/><strong>{{selected?.name??'新建连接'}}</strong></div><div class="toolbar-actions" v-if="selected"><button class="text-button" @click="duplicate" :disabled="!!busy"><Copy :size="14"/>复制配置</button><div class="more-wrap"><button class="icon-button" aria-label="更多连接操作" @click.stop="activeMenu=!activeMenu" :disabled="!!busy"><MoreHorizontal :size="19"/></button><div v-if="activeMenu" class="dropdown" @click.stop><button @click="activeMenu=false;remove()"><Trash2 :size="15"/>删除连接</button></div></div></div></div>
          <section class="connection-intro"><span class="large-db-icon"><Database :size="27" stroke-width="1.5"/></span><div><div class="intro-title"><h1>{{mode==='editor'?(selected?'编辑连接':'新建 MySQL 连接'):selected?.name}}</h1><span v-if="selected" class="env-badge" :class="selected.environment">{{envName(selected.environment)}}</span><span v-if="dirty&&mode==='editor'" class="unsaved">未保存</span></div><p>{{mode==='editor'?'配置服务器地址和登录信息。保存前可以先测试连接。':address(form)+' / '+form.username}}</p></div><div v-if="mode==='overview'" class="intro-connect"><span class="connection-status" :class="{'is-online':live}"><i></i>{{live?'已连接':'未连接'}}</span><button v-if="live" class="button" @click="disconnect" :disabled="!!busy"><Unplug :size="15"/>断开</button><button v-else class="button primary" @click="connect" :disabled="!!busy"><LoaderCircle v-if="busy==='connect'" :size="15" class="spin"/><Plug v-else :size="15"/>{{busy==='connect'?'正在连接…':'连接'}}</button></div></section>
          <div class="workspace-tabs"><button :class="{active:mode==='overview'}" :disabled="!selected||!!busy" @click="navigate(()=>{assignProfile(selected);mode='overview'})">连接概览</button><button :class="{active:mode==='editor'}" @click="!busy&&edit()" :disabled="!selected&&mode!=='editor'||!!busy">连接设置</button></div>
          <div class="content-scroll">
            <template v-if="mode==='editor'">
              <form class="connection-form" @submit.prevent="save()" :aria-busy="!!busy">
                <fieldset :disabled="!!busy||!!fatalError"><legend class="section-title">连接信息<span>为不同服务器设置容易识别的名称</span></legend>
                  <div class="form-grid"><label class="field">连接名称<span class="required">*</span><input id="connection-name" v-model="form.name" placeholder="例如：本地开发数据库" maxlength="80" :aria-invalid="!!errors.name"/><small v-if="errors.name" class="field-error">{{errors.name}}</small></label><label class="field">环境分组<select v-model="form.environment"><option v-for="g in groups.filter(g=>g.id!=='all')" :key="g.id" :value="g.id">{{g.label}}</option></select></label></div>
                  <div class="section-divider"></div><h2 class="section-title">服务器<span>使用 MySQL 服务的实际监听地址</span></h2>
                  <div class="form-grid host-grid"><label class="field">主机地址<span class="required">*</span><input v-model="form.host" placeholder="127.0.0.1 或 db.example.com" spellcheck="false" :aria-invalid="!!errors.host"/><small v-if="errors.host" class="field-error">{{errors.host}}</small></label><label class="field">端口<span class="required">*</span><input v-model.number="form.port" type="number" min="1" max="65535" :aria-invalid="!!errors.port"/><small v-if="errors.port" class="field-error">{{errors.port}}</small></label></div>
                  <div class="form-grid"><label class="field">用户名<span class="required">*</span><input v-model="form.username" placeholder="数据库用户名" autocomplete="off" spellcheck="false" :aria-invalid="!!errors.username"/><small v-if="errors.username" class="field-error">{{errors.username}}</small></label><label class="field">密码<span class="optional">可选</span><div class="password-input"><input v-model="form.password" :type="showPassword?'text':'password'" :placeholder="form.hasPassword&&form.passwordAction==='keep'?'已保存，留空保持原密码':'输入数据库密码'" autocomplete="new-password" @input="pwdChanged"/><button type="button" @click="showPassword=!showPassword" :aria-label="showPassword?'隐藏密码':'显示密码'">{{showPassword?'隐藏':'显示'}}</button></div></label></div>
                  <div class="password-options"><label class="checkbox"><input type="checkbox" v-model="form.rememberPassword"/><span>记住密码</span></label><small>使用此设备的系统凭据库保存</small><button v-if="form.hasPassword&&form.passwordAction==='keep'" type="button" class="text-button danger-text" @click="forgetPassword">移除已存密码</button></div>
                  <label class="field database-field">默认数据库<span class="optional">可选</span><input v-model="form.database" placeholder="留空，连接后查看可访问的数据库" spellcheck="false"/></label>
                  <button type="button" class="advanced-toggle" @click="advanced=!advanced"><ChevronDown :size="16" :class="{collapsed:!advanced}"/>高级设置<span>TLS 与连接超时</span><span v-if="form.tlsMode==='verify'" class="tls-enabled">TLS 已启用</span></button>
                  <div v-if="advanced" class="advanced-panel"><div class="form-grid"><label class="field">TLS 加密<select v-model="form.tlsMode"><option value="disabled">不启用 TLS</option><option value="verify">启用 TLS 并验证服务器证书</option></select></label><label class="field">连接超时<select v-model.number="form.timeout"><option :value="5">5 秒</option><option :value="10">10 秒</option><option :value="15">15 秒</option><option :value="30">30 秒</option></select></label></div><template v-if="form.tlsMode==='verify'"><label class="field">CA 证书<span class="optional">可选，默认使用系统信任库</span><div class="file-input"><input v-model="form.caPath" placeholder="选择 PEM 格式的 CA 证书"/><button type="button" class="icon-button" @click="chooseCA" aria-label="选择 CA 证书"><FolderOpen :size="17"/></button></div></label><label class="field">证书主机名<span class="optional">可选，默认使用连接主机</span><input v-model="form.serverName" placeholder="例如：mysql.example.com"/></label><p class="tls-note"><ShieldCheck :size="14"/>将验证服务器证书与主机名。</p></template><label class="field">备注<span class="optional">可选</span><textarea v-model="form.notes" rows="2" maxlength="2000" placeholder="记录连接用途或注意事项"></textarea></label></div>
                </fieldset>
              </form>
            </template>
            <template v-else>
              <section class="overview-section"><div class="section-heading"><h2>连接信息</h2><button class="text-button" @click="edit" :disabled="!!busy"><Pencil :size="14"/>编辑设置</button></div><dl class="details-grid"><div><dt>主机与端口</dt><dd>{{address(form)}}<button class="icon-button small-icon" aria-label="复制地址" @click="copyAddress"><Copy :size="13"/></button></dd></div><div><dt>用户名</dt><dd>{{form.username}}</dd></div><div><dt>默认数据库</dt><dd :class="{muted:!form.database}">{{form.database||'未指定'}}</dd></div><div><dt>密码保存</dt><dd><KeyRound :size="14"/>{{selected?.hasPassword?'已保存在系统凭据库':'未保存'}}</dd></div><div><dt>TLS</dt><dd>{{form.tlsMode==='verify'?'启用，验证服务器证书':'未启用'}}</dd></div><div><dt>连接超时</dt><dd>{{form.timeout}} 秒</dd></div></dl><p v-if="form.notes" class="connection-note">{{form.notes}}</p></section>
              <section class="overview-section database-section"><div class="section-heading"><h2>数据库<span v-if="live" class="count">{{live.databases.length}}</span></h2><button v-if="live" class="text-button" @click="refresh" :disabled="!!busy"><RefreshCw :size="14" :class="{spin:busy==='refresh'}"/>刷新列表</button></div><template v-if="live"><div class="server-meta"><CheckCircle2 :size="15"/><strong>{{live.serverVersion}}</strong><span>{{live.currentUser}}</span><span>连接耗时 {{live.latencyMs}} ms</span></div><div class="database-search"><Search :size="14"/><input v-model="dbSearch" placeholder="筛选数据库…" aria-label="筛选数据库"/></div><div class="database-list"><div v-for="db in databases" :key="db" class="database-row"><Database :size="17"/><span>{{db}}</span><span class="database-type">数据库</span></div><p v-if="databases.length===0" class="muted">没有匹配的数据库</p></div><p class="list-caption">列表仅显示当前账户有权限访问的数据库。点击刷新可检查连接是否仍然可用。</p></template><div v-else class="database-empty"><Cable :size="28" stroke-width="1.3"/><strong>连接后查看数据库</strong><p>使用保存的配置登录服务器，读取你有权限访问的数据库。</p><button class="button" @click="connect" :disabled="!!busy"><Plug :size="15"/>{{busy==='connect'?'正在连接…':'连接到 MySQL'}}</button></div></section>
              <div class="record-time"><Clock3 :size="13"/>最近修改 {{dateText(selected?.updatedAt??'')}}</div>
            </template>
            <div v-if="testResult" class="result-panel success-result" role="status"><CheckCircle2 :size="19"/><div><strong>连接测试成功</strong><p>{{testResult.serverVersion}}<span>耗时 {{testResult.latencyMs}} ms</span></p></div></div>
            <div v-if="testError" class="result-panel error-result" role="alert"><AlertCircle :size="19"/><div><strong>连接未成功</strong><p>{{testError}}</p></div></div>
          </div>
          <footer class="editor-footer" v-if="mode==='editor'"><button class="button" @click="test" :disabled="!!busy||!desktop||!!fatalError"><LoaderCircle v-if="busy==='test'" :size="15" class="spin"/><Play v-else :size="14"/>{{busy==='test'?'正在测试…':'测试连接'}}</button><div class="footer-right"><button class="button quiet" @click="cancelEdit" :disabled="!!busy">取消</button><button class="button" @click="save()" :disabled="!!busy||!desktop||!!fatalError"><Save :size="15"/>保存连接</button><button class="button primary" @click="connect" :disabled="!!busy||!desktop||!!fatalError"><LoaderCircle v-if="busy==='connect'||busy==='save'" :size="15" class="spin"/><Plug v-else :size="15"/>保存并连接</button></div></footer>
          <footer class="overview-footer" v-else><ShieldCheck :size="14"/>配置仅保存在本机<button class="text-button" @click="test" :disabled="!!busy"><LoaderCircle v-if="busy==='test'" :size="13" class="spin"/><Play v-else :size="13"/>测试连接</button></footer>
        </template>
      </main>
    </div>
    <div class="app-statusbar"><span><span class="status-dot" :class="{'is-online':desktop}"></span>{{desktop?'本地桌面应用':'浏览器预览'}}</span><span>MySQL<span class="status-separator">/</span>连接管理</span><span class="statusbar-right" :title="configPath"><ShieldCheck :size="12"/>本机存储</span></div>
    <Transition name="toast"><div v-if="message" class="toast-message" :class="messageKind" role="status"><AlertCircle v-if="messageKind==='error'" :size="18"/><CheckCircle2 v-else :size="18"/><span>{{message}}</span><button class="icon-button" aria-label="关闭提示" @click="message=''"><X :size="14"/></button></div></Transition>
    <div v-if="modal.open" class="modal-backdrop" @click.self="modal.open=false" @keydown.tab="trapDialog"><section class="modal" role="alertdialog" aria-modal="true" aria-labelledby="dialog-title" aria-describedby="dialog-body"><span class="modal-icon"><AlertCircle :size="22"/></span><h2 id="dialog-title">{{modal.title}}</h2><p id="dialog-body">{{modal.body}}</p><div class="modal-actions"><button class="button" data-modal-cancel @click="modal.open=false">取消</button><button class="button" :class="modal.danger?'danger':'primary'" @click="acceptModal">{{modal.label}}</button></div></section></div>
  </div>
</template>

