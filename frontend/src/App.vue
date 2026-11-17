<script setup lang="ts">
import {useTableResizing} from './tableResizing'
useTableResizing()
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { Plug, Unplug, Plus, Database, Table2, PanelsTopLeft, Braces, Users, Wrench, FileCode2, Archive, Workflow, Network, ChartNoAxesColumn, ChevronDown, ChevronRight, Folder, Search, X, RefreshCw, Pencil, Trash2, Copy, Info, List, LayoutGrid, PanelRight, SlidersHorizontal, FileText, Clock3, AlertCircle, CheckCircle2, LoaderCircle, Eye, ShieldCheck, Check, FolderOpen } from '@lucide/vue'
import ConnectionDialog from './ConnectionDialog.vue'
import GroupDialog from './GroupDialog.vue'
import InlineGroupEditor from './InlineGroupEditor.vue'
import DatabaseTools, {type DatabaseToolMode} from './DatabaseTools.vue'
import QueryEditor from './QueryEditor.vue'
import TableDataEditor from './TableDataEditor.vue'
import TableDesigner from './TableDesigner.vue'
import type {TableSection} from './tableEditing'
import ConfirmationDialog from './ConfirmationDialog.vue'
import AboutDialog from './AboutDialog.vue'
import {quoteIdentifier} from './tableData'
import {useNativeDialogs} from './nativeDialogs'
import {useMenuDismissal} from './menuDismissal'
import {hasNativeWindows,type DialogDetails,type DialogAction} from './api'
import { api, defaults, fromProfile, errorText, isDesktop, type Profile, type ConnectionInput, type ConnectionGroup, type SessionInfo, type DatabaseObject, type TestResult } from './api'

type Category = 'tables'|'views'|'functions'|'events'
type WorkspaceRow = { key:string; name:string; kind:'connection'|'database'|Category; profile?:Profile; database?:string; object?:DatabaseObject }
type MenuItem = {label:string; shortcut?:string; disabled?:boolean; danger?:boolean; checked?:boolean; action:()=>void|Promise<void>}
const profiles = ref<Profile[]>([])
const connectionGroups=ref<ConnectionGroup[]>([])
const selectedGroupID=ref('')
const expandedGroups=reactive(new Set<string>())
const groupDialogOpen=ref(false)
const groupForm=reactive({groupId:''})
const groupError=ref('')
const groupEditor=reactive({active:false,id:'',name:'',error:''})
let groupConnection:Profile|undefined
const sessions = reactive<Record<string,SessionInfo>>({})
const selectedID = ref('')
const selectedDatabase = ref('')
const databaseNodeSelected = ref(false)
const category = ref<Category>('tables')
const selectedRowKey = ref('')
const objects = ref<DatabaseObject[]>([])
const objectsLoading = ref(false)
const objectsError = ref('')
const truncated = ref(false)
const informationTab=ref<'info'|'ddl'>('info')
const ddlText=ref(''),ddlError=ref(''),ddlLoading=ref(false)
const ddlCopied=ref(false),ddlCopying=ref(false),ddlCopyError=ref(''),ddlCopyVersion=ref(0)
let ddlRequest=0
const search = ref('')
const objectSearch = ref('')
const group = ref('all')
const rootExpanded = ref(true)
const expandedConnections = reactive(new Set<string>())
const expandedDatabases = reactive(new Set<string>())
const openedDatabases = reactive(new Set<string>())
const expandedCategories=reactive(new Set<string>()),expandedTables=reactive(new Set<string>())
const treeObjects=reactive<Record<string,DatabaseObject[]>>({})
const treeKey=(id:string,db:string,kind:string)=>JSON.stringify([id,db,kind])
function clearTree(id?:string,db?:string){for(const key of Object.keys(treeObjects)){const parts=JSON.parse(key);if((!id||parts[0]===id)&&(!db||parts[1]===db))delete treeObjects[key]}for(const state of [expandedCategories,expandedTables])for(const key of state){const parts=JSON.parse(key);if((!id||parts[0]===id)&&(!db||parts[1]===db))state.delete(key)}}
function treeList(p:Profile,db:string,kind:Category){return treeObjects[treeKey(p.id,db,kind)]??[]}
function categoryExpandable(p:Profile,db:string,kind:Category){const list=treeObjects[treeKey(p.id,db,kind)];return !list||list.length>0}
async function toggleCategory(p:Profile,db:string,kind:Category,open?:boolean){const key=treeKey(p.id,db,kind);if(open===false||(open===undefined&&expandedCategories.has(key))){expandedCategories.delete(key);return}expandedCategories.add(key);await selectDatabase(p,db,kind)}
async function selectTreeObject(p:Profile,db:string,kind:Category,object:DatabaseObject){activeWorkspaceTab.value='objects';if(selectedID.value!==p.id||selectedDatabase.value!==db||category.value!==kind)await selectDatabase(p,db,kind);if(selectedID.value===p.id&&selectedDatabase.value===db&&category.value===kind){databaseNodeSelected.value=false;selectWorkspaceRow({key:object.name,name:object.name,kind,object});showInformation.value=true}}
function toggleTable(p:Profile,db:string,object:DatabaseObject,open?:boolean){const key=treeKey(p.id,db,object.name);if(open===false||(open===undefined&&expandedTables.has(key)))expandedTables.delete(key);else expandedTables.add(key)}
async function refreshTableList(p:Profile,db:string){clearTree(p.id,db);await selectDatabase(p,db,'tables');await loadObjects()}
async function contextForTable(e:MouseEvent,p:Profile,db:string,object?:DatabaseObject){contextMenu.table=object?.name??'';contextMenu.database=db;contextMenu.connectionID=p.id;showContext(e,'table');if(object)await selectTreeObject(p,db,'tables',object)}

const leftWidth = ref(240)
const rightWidth = ref(218)
const showSidebar = ref(true)
const showInformation = ref(true)
const viewMode = ref<'details'|'icons'>('details')
const busy = ref('')
const loading = ref(true)
const fatalError = ref('')
const configPath = ref('')
const desktop = ref(isDesktop())
const nativeWindows=hasNativeWindows()
const menu = ref('')
const contextMenu = reactive({open:false,x:0,y:0,kind:'connection' as 'root'|'group'|'connection'|'database'|'table',database:'',connectionID:'',table:''})
const databaseTool=ref<{profile:Profile;database:string;mode:DatabaseToolMode;table?:string;initialSQL?:string}|null>(null)
type WorkspaceDocument={id:string;title:string;profile:Profile;database:string}&({kind:'query';initialSQL:string}|{kind:'table';table:string;section:TableSection;dirty:boolean}|{kind:'design';table?:string;dirty:boolean})
const workspaceTabs=ref<WorkspaceDocument[]>([]),activeWorkspaceTab=ref('objects')
const activeDocument=computed(()=>workspaceTabs.value.find(tab=>tab.id===activeWorkspaceTab.value))
let querySequence=0
function activateWorkspaceTab(id:string,focusTab=false){activeWorkspaceTab.value=id;nextTick(()=>{const tab=document.getElementById('workspace-tab-'+id);tab?.scrollIntoView?.({block:'nearest',inline:'nearest'});if(focusTab)tab?.focus();else if(id!=='objects')document.getElementById('workspace-panel-'+id)?.querySelector<HTMLTextAreaElement>('textarea')?.focus()})}
function createQueryTab(profile:Profile,database:string,table?:string){
 const tab:WorkspaceDocument={kind:'query',id:crypto.randomUUID(),title:'查询 '+(++querySequence),profile:{...profile},database,initialSQL:table?'SELECT * FROM '+quoteIdentifier(table)+' LIMIT 1000;':''}
 workspaceTabs.value.push(tab);activateWorkspaceTab(tab.id)
}
function removeWorkspaceTab(id:string){const index=workspaceTabs.value.findIndex(tab=>tab.id===id);if(index<0)return;const active=activeWorkspaceTab.value===id;workspaceTabs.value.splice(index,1);if(active)activateWorkspaceTab(workspaceTabs.value[index]?.id??workspaceTabs.value[index-1]?.id??'objects',true)}
const queryDocuments=computed(()=>workspaceTabs.value.filter((tab):tab is Extract<WorkspaceDocument,{kind:'query'}>=>tab.kind==='query'))
const designDocuments=computed(()=>workspaceTabs.value.filter((tab):tab is Extract<WorkspaceDocument,{kind:'design'}>=>tab.kind==='design'))
function openDesignTab(profile:Profile,database:string,table?:string){closeMenus();if(!sessions[profile.id]){showNavigationError('请先打开连接');return}let tab=table?workspaceTabs.value.find(t=>t.kind==='design'&&t.profile.id===profile.id&&t.database===database&&t.table===table):undefined;if(!tab){tab={kind:'design',id:crypto.randomUUID(),title:table?table+' - 设计':'无标题 - 表',profile:{...profile},database,table,dirty:false};workspaceTabs.value.push(tab)}activateWorkspaceTab(tab.id)}
async function designSaved(tab:Extract<WorkspaceDocument,{kind:'design'}>,name:string){tab.table=name;tab.title=name+' - 设计';try{const session=sessions[tab.profile.id]?.connectedAt,request=objectRequest,result=await api.ListObjects(tab.profile.id,tab.database,'tables');if(sessions[tab.profile.id]?.connectedAt!==session)return;treeObjects[treeKey(tab.profile.id,tab.database,'tables')]=result.objects;if(request===objectRequest&&selectedID.value===tab.profile.id&&selectedDatabase.value===tab.database&&category.value==='tables'){objects.value=result.objects;truncated.value=result.truncated;clearDDL()}}catch(e){showNavigationError('结构已保存，但表列表刷新失败：'+errorText(e))}}
const tableDocuments=computed(()=>workspaceTabs.value.filter((tab):tab is Extract<WorkspaceDocument,{kind:'table'}>=>tab.kind==='table'))
function closeWorkspaceTab(id:string){const tab=workspaceTabs.value.find(tab=>tab.id===id);if(tab?.kind==='design'&&tab.dirty){ask('关闭设计并放弃修改？','当前表设计有未保存的修改，关闭将丢弃设计草稿并取消请求，已完成的结构变更不会回滚。','放弃修改',()=>removeWorkspaceTab(id),true);return}if(tab?.kind==='table'&&tab.dirty){ask('关闭表并放弃修改？','当前表有未保存记录或尚未结束的事务，关闭将放弃草稿并请求回滚未提交事务。','放弃修改',()=>removeWorkspaceTab(id),true);return}removeWorkspaceTab(id)}
function openTableTab(profile:Profile,database:string,table:string,section:TableSection='data'){
 closeMenus();if(!sessions[profile.id]){showNavigationError('请先打开连接');return}
 let tab=workspaceTabs.value.find(tab=>tab.kind==='table'&&tab.profile.id===profile.id&&tab.database===database&&tab.table===table)
 if(tab?.kind==='table')tab.section=section
 else{tab={kind:'table',id:crypto.randomUUID(),title:table,profile:{...profile},database,table,section,dirty:false};workspaceTabs.value.push(tab)}
 rootExpanded.value=true;if(profile.groupId)expandedGroups.add(profile.groupId);expandedConnections.add(profile.id);expandedDatabases.add(profile.id+'/'+database);expandedCategories.add(treeKey(profile.id,database,'tables'));expandedTables.add(treeKey(profile.id,database,table));activateWorkspaceTab(tab.id)
}
function workspaceTabKeys(event:KeyboardEvent,id:string){
 if(event.key==='Delete'&&id!=='objects'){event.preventDefault();closeWorkspaceTab(id);return}
 if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return
 event.preventDefault();const ids=['objects',...workspaceTabs.value.map(tab=>tab.id)],index=ids.indexOf(id)
 activateWorkspaceTab(ids[event.key==='Home'?0:event.key==='End'?ids.length-1:(index+(event.key==='ArrowRight'?1:ids.length-1))%ids.length]!,true)
}
const navigationError=ref('')
const draggedConnectionID=ref('')
const movingConnectionID=ref('')
const dropTarget=ref<string|null>(null)
const draggedConnection=computed(()=>profiles.value.find(p=>p.id===draggedConnectionID.value))
const connectionDragType='application/x-bajiu-connection'
const form = reactive(defaults())
const baseline = ref(JSON.stringify(form))
const dialogOpen = ref(false)
const connectionClientVersion=ref(0),groupClientVersion=ref(0)
const editingID = ref('')
const connectOnSave = ref(false)
const dialogRef = ref<InstanceType<typeof ConnectionDialog>|null>(null)
const errors = reactive<Record<string,string>>({})
const testResult = ref<TestResult|null>(null)
const testError = ref('')
const confirm = reactive({open:false,title:'',body:'',label:'',danger:false,action:null as null|(()=>void|Promise<void>)})
const about = ref(false)
const detailDialog=ref<DialogDetails>()
type RuntimeCredentials=Pick<ConnectionInput,'password'|'sshPassword'|'sshKeyPassphrase'|'httpToken'>
const transientPasswords = new Map<string,RuntimeCredentials>()
let returnFocus:HTMLElement|null=null
let poll:ReturnType<typeof setInterval>|undefined
let objectRequest=0
let releaseDrag:(()=>void)|undefined

const groups=[{id:'all',label:'全部'},{id:'local',label:'本地'},{id:'development',label:'开发'},{id:'staging',label:'测试'},{id:'production',label:'生产'}]
const tableDetails:[TableSection,string,typeof Table2][]=[['fields','字段',List],['indexes','索引',Table2],['foreignKeys','外键',Network],['triggers','触发器',Workflow]]
const categories:[Category,string,typeof Table2][]=[['tables','表',Table2],['views','视图',PanelsTopLeft],['functions','函数',Braces],['events','事件',Clock3]]
const selected = computed(()=>profiles.value.find(p=>p.id===selectedID.value))
const selectedConnectionGroup=computed(()=>connectionGroups.value.find(g=>g.id===selectedGroupID.value))
const live = computed(()=>sessions[selectedID.value])
const toolProfile=computed(()=>activeDocument.value?.profile??selected.value)
const toolLive=computed(()=>toolProfile.value?sessions[toolProfile.value.id]:undefined)
const toolDatabase=computed(()=>activeDocument.value?.database||selectedDatabase.value||selectedRow.value?.database||'')
function openSelectedDatabaseTool(mode:DatabaseToolMode){if(toolProfile.value&&toolDatabase.value)showDatabaseTool(toolProfile.value,toolDatabase.value,mode)}
const categoryName = computed(()=>categories.find(c=>c[0]===category.value)?.[1]??'表')
const filteredProfiles = computed(()=>profiles.value.filter(p=>(group.value==='all'||p.environment===group.value)&&`${p.name} ${p.host} ${p.username} ${(sessions[p.id]?.databases??[]).join(' ')}`.toLowerCase().includes(search.value.toLowerCase())))
const navigationSections=computed(()=>[
 {id:'',name:'',profiles:filteredProfiles.value.filter(p=>!p.groupId)},
 ...connectionGroups.value.map(g=>({...g,profiles:(search.value.trim()&&g.name.toLowerCase().includes(search.value.trim().toLowerCase())?profiles.value.filter(p=>group.value==='all'||p.environment===group.value):filteredProfiles.value).filter(p=>p.groupId===g.id)})).filter(g=>!search.value.trim()||g.profiles.length||g.name.toLowerCase().includes(search.value.trim().toLowerCase()))
])
const dirty = computed(()=>JSON.stringify(form)!==baseline.value)
const workspaceRows = computed<WorkspaceRow[]>(()=>{
  let rows:WorkspaceRow[]
  if(selectedDatabase.value) rows=objects.value.map(o=>({key:o.name,name:o.name,kind:category.value,object:o}))
  else if(selected.value) rows=(live.value?.databases??[]).map(name=>({key:name,name,kind:'database',database:name}))
  else rows=filteredProfiles.value.filter(p=>!selectedGroupID.value||p.groupId===selectedGroupID.value).map(p=>({key:p.id,name:p.name,kind:'connection',profile:p}))
  const term=objectSearch.value.trim().toLowerCase()
  return term?rows.filter(row=>`${row.name} ${row.object?.comment??''}`.toLowerCase().includes(term)):rows
})
const selectedRow = computed(()=>workspaceRows.value.find(r=>r.key===selectedRowKey.value))
const selectedObject = computed(()=>selectedRow.value?.object)
const tableToolbarVisible=computed(()=>!!toolDatabase.value)
const toolbarTable=computed(()=>selectedObject.value?.kind==='tables'?selectedObject.value.name:'')
const tableToolbarDisabled=computed(()=>!live.value||!toolDatabase.value||!!busy.value||objectsLoading.value)
function openTableTool(mode:DatabaseToolMode){const p=selected.value,db=toolDatabase.value;if(!p||!db||tableToolbarDisabled.value)return;if((mode==='openTable'||mode==='designTable')&&!toolbarTable.value)return;showDatabaseTool(p,db,mode,toolbarTable.value||undefined)}
function deleteToolbarTable(){
 const p=selected.value,db=toolDatabase.value,table=toolbarTable.value
 if(!p||!db||!table||tableToolbarDisabled.value)return
 ask('删除表“'+table+'”？','将永久删除数据库“'+db+'”中的表“'+table+'”及其全部数据，此操作无法撤销。','删除表',async()=>{
  busy.value='drop-table'
  try{const result=await api.DatabaseAdmin(crypto.randomUUID(),p.id,db,{operation:'query',sql:'DROP TABLE '+quoteIdentifier(db)+'.'+quoteIdentifier(table)+';'});if(result.error)throw new Error(result.error);busy.value='';await refreshTableList(p,db)}catch(e){showNavigationError(errorText(e))}finally{busy.value=''}
 },true)
}
const canShowDDL=computed(()=>selectedObject.value?.kind==='tables')
const informationName = computed(()=>selectedObject.value?.name??selectedRow.value?.name??selectedDatabase.value??'')
const scopeName = computed(()=>selectedDatabase.value?categoryName.value:selected.value?'数据库':'连接')
const columns = computed(()=>selectedDatabase.value
  ? category.value==='tables'||category.value==='views'?['名称','自动递增值','修改日期','数据长度','引擎','行','注释']:['名称','类型 / 状态','修改日期','创建日期','注释']
  : selected.value?['名称','类型']:['名称','主机','端口','用户名','环境','状态'])

function address(p:Profile){return `${p.host.includes(':')?'['+p.host+']':p.host}:${p.port}`}
function envName(id:string){return groups.find(g=>g.id===id)?.label??'本地'}
function dateText(date:string){return date?new Date(date).toLocaleString('zh-CN',{hour12:false}):'—'}
function bytesText(value:string){if(!value)return '—';try{const number=BigInt(value);if(number<1024n)return number+' B';if(number<1048576n)return (Number(number)/1024).toFixed(1)+' KiB';if(number<1073741824n)return (Number(number)/1048576).toFixed(1)+' MiB';return (Number(number)/1073741824).toFixed(1)+' GiB'}catch{return value}}
function showNavigationError(text:string){navigationError.value=text}
function clearFeedback(){testResult.value=null;testError.value='';for(const key of Object.keys(errors))delete errors[key]}
function requestInput(){const {createdAt:_,updatedAt:__,hasPassword:___,...input}=form;return {...input,port:Number(form.port),timeout:Number(form.timeout)}}
function connectionInput(p:Profile){const input=fromProfile(p);if(transientPasswords.has(p.id))Object.assign(input,transientPasswords.get(p.id),{passwordAction:'replace',sshPasswordAction:'replace',sshKeyPassphraseAction:'replace',httpTokenAction:'replace'});const {createdAt:_,updatedAt:__,hasPassword:___,...request}=input;return request}
function ask(title:string,body:string,label:string,action:()=>void|Promise<void>,danger=false){Object.assign(confirm,{open:true,title,body,label,action,danger});nextTick(()=>document.querySelector<HTMLButtonElement>('[data-confirm-cancel]')?.focus())}
async function acceptConfirm(){const action=confirm.action;confirm.open=false;confirm.action=null;await action?.()}
function cancelConfirm(){confirm.open=false;nextTick(()=>{if(dialogOpen.value)document.getElementById('connection-name')?.focus();else(document.querySelector('[data-tree-row]') as HTMLElement|null)?.focus()})}
function focusReturn(){nextTick(()=>{if(returnFocus?.isConnected)returnFocus.focus();else(document.querySelector<HTMLElement>('.tree-connection.selected')??document.querySelector<HTMLElement>('.tree-group.selected')??document.querySelector<HTMLElement>('.tree-root'))?.focus()})}
function closeDialog(){dialogOpen.value=false;clearFeedback();Object.assign(form,defaults());baseline.value=JSON.stringify(form);focusReturn()}
function cancelDialog(){if(busy.value)return;if(dirty.value)ask('放弃未保存的修改？','此连接的修改尚未保存。是否放弃这些修改并关闭窗口？','放弃修改',closeDialog,true);else closeDialog()}
function openDialog(p?:Profile,connect=false){
  if(busy.value)return
  connectionClientVersion.value=0
  returnFocus=document.activeElement as HTMLElement
  clearFeedback();menu.value='';contextMenu.open=false
  Object.assign(form,p?fromProfile(p):defaults())
  if(!p)form.groupId=selectedGroupID.value
  if(p&&transientPasswords.has(p.id))Object.assign(form,transientPasswords.get(p.id),{passwordAction:'replace',sshPasswordAction:'replace',sshKeyPassphraseAction:'replace',httpTokenAction:'replace'})
  editingID.value=p?.id??'';connectOnSave.value=connect;baseline.value=JSON.stringify(form);dialogOpen.value=true
}
function create(){openDialog()}
function edit(){if(selected.value)openDialog(selected.value)}
function duplicate(){const p=selected.value;if(!p)return;openDialog();Object.assign(form,{...p,id:'',revision:0,name:p.name+' 副本',hasPassword:false,password:'',passwordAction:'replace',rememberPassword:true,createdAt:'',updatedAt:''});}
function validate(requireName=true){for(const key of Object.keys(errors))delete errors[key];if(requireName&&!form.name.trim())errors.name='请输入连接名';if(!form.host.trim())errors.host='请输入主机地址';if(!Number.isInteger(Number(form.port))||form.port<1||form.port>65535)errors.port='端口范围为 1–65535';if(!form.username.trim())errors.username='请输入用户名';if(Object.keys(errors).length){dialogRef.value?.showValidation();return false}return true}
function clearDDL(resetTab=true){ddlRequest++;if(resetTab)informationTab.value='info';ddlText.value='';ddlError.value='';ddlLoading.value=false;ddlCopied.value=false;ddlCopying.value=false;ddlCopyError.value=''}
function invalidateObjects(){objectRequest++;objects.value=[];objectsError.value='';objectsLoading.value=false;selectedRowKey.value='';truncated.value=false;clearDDL()}
function clearDatabaseState(id:string){clearTree(id);for(const state of [openedDatabases,expandedDatabases])for(const key of state)if(key.startsWith(id+'/'))state.delete(key)}
function databaseIsOpen(id:string,database:string){return !!sessions[id]&&openedDatabases.has(id+'/'+database)}
function databaseIconClass(id:string,database:string){return databaseIsOpen(id,database)?'database-icon database-open':'database-icon'}
function selectProfile(p:Profile){if(busy.value||dialogOpen.value||groupDialogOpen.value)return;activeWorkspaceTab.value='objects';selectedGroupID.value=p.groupId??'';selectedID.value=p.id;selectedDatabase.value='';databaseNodeSelected.value=false;objectSearch.value='';invalidateObjects()}
function selectRoot(){if(busy.value||dialogOpen.value||groupDialogOpen.value)return;activeWorkspaceTab.value='objects';selectedGroupID.value='';selectedID.value='';selectedDatabase.value='';databaseNodeSelected.value=false;objectSearch.value='';invalidateObjects()}
function selectConnectionGroup(id:string){if(busy.value||dialogOpen.value||groupDialogOpen.value)return;selectRoot();selectedGroupID.value=id}
function toggleGroup(id:string){if(expandedGroups.has(id))expandedGroups.delete(id);else expandedGroups.add(id)}
function startConnectionDrag(e:DragEvent,p:Profile){
 if(busy.value||loading.value||dialogOpen.value||groupDialogOpen.value||confirm.open||about.value||groupEditor.active||!e.dataTransfer){e.preventDefault();return}
 closeMenus();navigationError.value='';draggedConnectionID.value=p.id;dropTarget.value=null
 e.dataTransfer.effectAllowed='move';e.dataTransfer.setData(connectionDragType,p.id)
}
function endConnectionDrag(){draggedConnectionID.value='';dropTarget.value=null}
function canDropConnection(target:string,e:DragEvent){
 const p=draggedConnection.value
 return !!p&&!busy.value&&!dialogOpen.value&&!groupDialogOpen.value&&!confirm.open&&!about.value&&!groupEditor.active&&!!e.dataTransfer?.types.includes(connectionDragType)&&(p.groupId??'')!==target&&(!target||connectionGroups.value.some(g=>g.id===target))
}
function overConnectionTarget(e:DragEvent,target:string){
 if(!canDropConnection(target,e)){if(e.dataTransfer)e.dataTransfer.dropEffect='none';dropTarget.value=null;return}
 e.preventDefault();if(e.dataTransfer)e.dataTransfer.dropEffect='move';dropTarget.value=target
}
function leaveConnectionTarget(e:DragEvent,target:string){
 if(e.relatedTarget instanceof Node&&(e.currentTarget as HTMLElement).contains(e.relatedTarget))return
 if(dropTarget.value===target)dropTarget.value=null
}
async function dropConnection(e:DragEvent,target:string){
 e.preventDefault()
 const p=draggedConnection.value
 const valid=p&&canDropConnection(target,e)&&e.dataTransfer?.getData(connectionDragType)===p.id
 endConnectionDrag()
 if(!valid||!p)return
 navigationError.value='';busy.value='move-connection';movingConnectionID.value=p.id
 try{
  const updated=await api.MoveConnectionToGroup(p.id,p.revision,target)
  profiles.value=profiles.value.map(profile=>profile.id===updated.id?updated:profile)
  if(selectedID.value===updated.id)selectedGroupID.value=updated.groupId??''
  if(updated.groupId)expandedGroups.add(updated.groupId)
  rootExpanded.value=true
 }catch(error){showNavigationError(errorText(error))}finally{busy.value='';movingConnectionID.value=''}
}
function showGroupDialog(){
 if(busy.value)return
 groupClientVersion.value=0
 returnFocus=document.activeElement as HTMLElement;closeMenus();groupError.value=''
 groupConnection=selected.value
 groupForm.groupId=groupConnection?.groupId??''
 groupDialogOpen.value=true
}
function beginGroupEdit(target?:ConnectionGroup){
 if(busy.value||dialogOpen.value||groupDialogOpen.value)return
 closeMenus();rootExpanded.value=true;search.value=''
 if(target)selectConnectionGroup(target.id);else selectRoot()
 let name=target?.name??'新建组'
 if(!target){let index=2;while(connectionGroups.value.some(g=>g.name===name))name='新建组 ('+(index++)+')'}
 Object.assign(groupEditor,{active:true,id:target?.id??'',name,error:''})
 nextTick(()=>{const input=document.querySelector<HTMLInputElement>('.inline-group-name');input?.focus();input?.select()})
}
function cancelGroupEdit(){if(busy.value)return;groupEditor.active=false;groupEditor.error='';focusReturn()}
async function saveInlineGroup(){
 if(!groupEditor.active||busy.value)return
 if(!groupEditor.name.trim()){groupEditor.error='请输入组名';return}
 busy.value='group-edit';groupEditor.error=''
 const id=groupEditor.id
 try{
  const saved=await api.SaveGroup(id,groupEditor.name)
  const index=connectionGroups.value.findIndex(g=>g.id===saved.id)
  if(index<0)connectionGroups.value=[...connectionGroups.value,saved];else connectionGroups.value=connectionGroups.value.map(g=>g.id===saved.id?saved:g)
  selectedGroupID.value=saved.id;selectedID.value='';selectedDatabase.value='';invalidateObjects();expandedGroups.add(saved.id);groupEditor.active=false
  focusReturn()
 }catch(e){groupEditor.error=errorText(e)}finally{busy.value=''}
}
function removeGroup(){
 const target=selectedConnectionGroup.value
 if(!target||busy.value)return
 const count=profiles.value.filter(p=>p.groupId===target.id).length
 ask('删除组“'+target.name+'”？',count?'组内 '+count+' 个连接将移回“我的连接”，连接配置、密码和已打开的会话都会保留。':'只删除此空组，不会删除任何连接或数据库。','删除组',async()=>{
  busy.value='delete-group'
  try{
   await api.DeleteGroup(target.id)
   profiles.value=await api.ListConnections()
   connectionGroups.value=connectionGroups.value.filter(g=>g.id!==target.id);expandedGroups.delete(target.id);selectedGroupID.value='';selectedID.value='';selectedDatabase.value='';invalidateObjects();focusReturn()
  }catch(e){showNavigationError(errorText(e))}finally{busy.value=''}
 },true)
}
function closeGroupDialog(){if(busy.value)return;groupDialogOpen.value=false;groupError.value='';groupConnection=undefined;focusReturn()}
async function saveGroupDialog(){
 if(busy.value)return
 groupError.value='';busy.value='group'
 try{
   if(!groupConnection)throw new Error('请重新选择连接')
   const updated=await api.MoveConnectionToGroup(groupConnection.id,groupConnection.revision,groupForm.groupId)
   profiles.value=profiles.value.map(p=>p.id===updated.id?updated:p);selectedGroupID.value=updated.groupId??''
   if(updated.groupId)expandedGroups.add(updated.groupId)
   
  busy.value='';closeGroupDialog()
 }catch(e){groupError.value=errorText(e)}finally{busy.value=''}
}
async function disconnectAll(){
 if(busy.value)return
 busy.value='disconnect-all'
 try{
  await api.DisconnectAll()
  for(const id of Object.keys(sessions))delete sessions[id]
  clearTree();openedDatabases.clear();expandedDatabases.clear();expandedConnections.clear();selectedDatabase.value='';invalidateObjects();
 }catch(e){showNavigationError(errorText(e))}finally{busy.value=''}
}
async function saveDialog(){
  if(!validate()||busy.value)return
  busy.value='save'
  const source=requestInput(),shouldConnect=connectOnSave.value
  try{
    const profile=await api.SaveConnection(source)
    delete sessions[profile.id];clearDatabaseState(profile.id)
    if(!source.rememberPassword)transientPasswords.set(profile.id,{password:source.password,sshPassword:source.sshPassword,sshKeyPassphrase:source.sshKeyPassphrase,httpToken:source.httpToken})
    else transientPasswords.delete(profile.id)
    profiles.value=await api.ListConnections();selectedID.value=profile.id;selectedGroupID.value=profile.groupId??'';if(profile.groupId)expandedGroups.add(profile.groupId);selectedDatabase.value='';invalidateObjects()
    closeDialog();rootExpanded.value=true;expandedConnections.delete(profile.id)
    
    busy.value=''
    if(shouldConnect)await openConnection(profile)
  }catch(e){testError.value=errorText(e)}finally{busy.value=''}
}
async function testConnection(){if(!validate(false)||busy.value)return;busy.value='test';testResult.value=null;testError.value='';try{testResult.value=await api.TestConnection(requestInput())}catch(e){testError.value=errorText(e)}finally{busy.value=''}}
async function openConnection(p=selected.value){
  if(!p||busy.value||dialogOpen.value)return
  selectProfile(p)
  if(sessions[p.id]){expandedConnections.add(p.id);return}
  if(!p.hasPassword&&!transientPasswords.has(p.id)){openDialog(p,true);return}
  busy.value='connect';objectsError.value=''
  try{sessions[p.id]=await api.Connect(connectionInput(p));expandedConnections.add(p.id);}
  catch(e){objectsError.value=errorText(e)}
  finally{busy.value=''}
}
async function toggleConnection(p:Profile){if(busy.value)return;if(expandedConnections.has(p.id)){expandedConnections.delete(p.id);return}await openConnection(p)}
async function disconnect(){const id=selectedID.value;if(!id||busy.value)return;busy.value='disconnect';try{await api.Disconnect(id);delete sessions[id];clearDatabaseState(id);expandedConnections.delete(id);selectedDatabase.value='';invalidateObjects();}catch(e){showNavigationError(errorText(e))}finally{busy.value=''}}
async function selectDatabase(p:Profile,database:string,kind:Category='tables',expand=true){
  if(busy.value||dialogOpen.value||!sessions[p.id])return
  activeWorkspaceTab.value='objects'
  const sameScope=selectedID.value===p.id&&selectedDatabase.value===database&&category.value===kind
  selectedID.value=p.id;selectedGroupID.value=p.groupId??'';selectedDatabase.value=database;databaseNodeSelected.value=false;category.value=kind;objectSearch.value='';expandedConnections.add(p.id);selectedRowKey.value='';clearDDL()
  if(expand)expandedDatabases.add(p.id+'/'+database)
  if(sameScope&&(objectsLoading.value||(!objectsError.value&&databaseIsOpen(p.id,database))))return
  await loadObjects()
}
function selectDatabaseInfo(p:Profile,database:string){
 if(busy.value||dialogOpen.value||groupDialogOpen.value||!sessions[p.id])return
 if(databaseIsOpen(p.id,database)||(objectsLoading.value&&selectedID.value===p.id&&selectedDatabase.value===database))void selectDatabase(p,database,'tables',false)
 else{selectProfile(p);selectedRowKey.value=database}
 databaseNodeSelected.value=true;showInformation.value=true
}
async function loadObjects(){
  const id=selectedID.value,database=selectedDatabase.value,kind=category.value
  if(!id||!database||!sessions[id])return
  const request=++objectRequest;objectsLoading.value=true;objectsError.value='';objects.value=[];truncated.value=false;selectedRowKey.value='';clearDDL()
  try{const result=await api.ListObjects(id,database,kind);if(request===objectRequest){objects.value=result.objects??[];treeObjects[treeKey(id,database,kind)]=objects.value;truncated.value=result.truncated;openedDatabases.add(id+'/'+database)}}
  catch(e){if(request===objectRequest)objectsError.value=errorText(e)}
  finally{if(request===objectRequest)objectsLoading.value=false}
}
async function changeCategory(kind:Category){activeWorkspaceTab.value='objects';category.value=kind;databaseNodeSelected.value=false;if(selectedDatabase.value)await loadObjects()}
function toggleDatabase(p:Profile,database:string){
  if(busy.value||dialogOpen.value||groupDialogOpen.value||!sessions[p.id])return
  const key=p.id+'/'+database
  const pending=objectsLoading.value&&selectedID.value===p.id&&selectedDatabase.value===database
  if(!databaseIsOpen(p.id,database)&&!pending){void selectDatabase(p,database);databaseNodeSelected.value=true;return}
  if(expandedDatabases.has(key))expandedDatabases.delete(key);else expandedDatabases.add(key)
}
async function refresh(){if(busy.value||!live.value)return;if(selectedDatabase.value){await loadObjects();return}busy.value='refresh';try{sessions[selectedID.value]=await api.RefreshDatabases(selectedID.value);objectsError.value=''}catch(e){delete sessions[selectedID.value];clearDatabaseState(selectedID.value);expandedConnections.delete(selectedID.value);invalidateObjects();objectsError.value=errorText(e)}finally{busy.value=''}}
function selectWorkspaceRow(row:WorkspaceRow){
  if(selectedRowKey.value===row.key)return
  const keepDDL=informationTab.value==='ddl'&&row.object?.kind==='tables'
  selectedRowKey.value=row.key;clearDDL(!keepDDL)
  if(keepDDL)void showDDL()
}
async function showDDL(){
 if(!canShowDDL.value||!selectedObject.value)return
 informationTab.value='ddl';if(ddlLoading.value||ddlText.value)return
 const request=++ddlRequest;ddlLoading.value=true;ddlError.value=''
 try{const result=await api.TableDDL(selectedID.value,selectedDatabase.value,selectedObject.value.name);if(request===ddlRequest)ddlText.value=result}catch(error){if(request===ddlRequest)ddlError.value=errorText(error)}finally{if(request===ddlRequest)ddlLoading.value=false}
}
function openRow(row:WorkspaceRow){selectWorkspaceRow(row);if(row.profile)void toggleConnection(row.profile);else if(row.database&&selected.value)void selectDatabase(selected.value,row.database);else if(row.object?.kind==='tables'&&selected.value&&selectedDatabase.value)openTableTab(selected.value,selectedDatabase.value,row.name);else showInformation.value=true}
async function copyDDL(){
  if(!ddlText.value||ddlCopying.value)return
  const request=ddlRequest
  ddlCopying.value=true;ddlCopied.value=false;ddlCopyError.value=''
  try{await navigator.clipboard.writeText(ddlText.value);if(request===ddlRequest){ddlCopied.value=true;ddlCopyVersion.value++}}
  catch{if(request===ddlRequest)ddlCopyError.value='复制失败，请检查剪贴板权限后重试'}
  finally{if(request===ddlRequest)ddlCopying.value=false}
}
function remove(){const p=selected.value;if(!p)return;ask('删除连接“'+p.name+'”？','仅删除本机的连接配置和保存的密码，不会删除服务器上的数据库。','删除连接',async()=>{busy.value='delete';try{await api.DeleteConnection(p.id,p.revision);delete sessions[p.id];clearDatabaseState(p.id);transientPasswords.delete(p.id);expandedConnections.delete(p.id);profiles.value=await api.ListConnections();busy.value='';selectRoot();}catch(e){showNavigationError(errorText(e))}finally{busy.value=''}},true)}
function passwordChanged(){form.passwordAction='replace';clearFeedback()}
function forgetPassword(){form.password='';form.sshPassword='';form.sshKeyPassphrase='';form.httpToken='';form.passwordAction='forget';form.sshPasswordAction='replace';form.sshKeyPassphraseAction='replace';form.httpTokenAction='replace';form.rememberPassword=false;form.hasPassword=false}
async function chooseCA(){try{const path=await api.SelectCAFile();if(path)form.caPath=path}catch(e){testError.value=errorText(e)}}
async function chooseSSHKey(){try{const path=await api.SelectSSHKey();if(path)form.sshKeyPath=path}catch(error){testError.value=errorText(error)}}
function showContext(e:MouseEvent,kind:'root'|'group'|'connection'|'database'|'table'){
 if(busy.value||dialogOpen.value||groupDialogOpen.value)return
 navigationError.value='';contextMenu.kind=kind;contextMenu.open=true;contextMenu.x=Math.max(0,Math.min(e.clientX,window.innerWidth-225));contextMenu.y=Math.max(0,Math.min(e.clientY,window.innerHeight-(kind==='database'?330:kind==='table'?260:230)));menu.value=''
 nextTick(()=>document.querySelector<HTMLButtonElement>('.connection-context-menu button:not(:disabled)')?.focus())
}
function contextFor(e:MouseEvent,p:Profile){selectProfile(p);showContext(e,'connection')}
function contextForRoot(e:MouseEvent){selectRoot();showContext(e,'root')}
function contextForGroup(e:MouseEvent,id:string){selectConnectionGroup(id);showContext(e,'group')}
function contextForDatabase(e:MouseEvent,p:Profile,database:string){selectDatabaseInfo(p,database);contextMenu.database=database;contextMenu.connectionID=p.id;showContext(e,'database')}
function contextForWorkspaceRow(e:MouseEvent,row:WorkspaceRow){if(row.object?.kind==='tables'&&selected.value){void contextForTable(e,selected.value,selectedDatabase.value,row.object);return}if(row.database&&selected.value)contextForDatabase(e,selected.value,row.database)}
function closeDatabase(p:Profile,database:string){clearTree(p.id,database);openedDatabases.delete(p.id+'/'+database);expandedDatabases.delete(p.id+'/'+database);if(selectedID.value===p.id&&selectedDatabase.value===database){selectProfile(p);selectedRowKey.value=database;databaseNodeSelected.value=true}}
function closeDatabaseTool(){databaseTool.value=null;nextTick(()=>document.querySelector<HTMLElement>('.tree-row.selected')?.focus())}
function showDatabaseTool(p:Profile,database:string,mode:DatabaseToolMode,table?:string){closeMenus();if(mode==='createTable'||mode==='designTable'){openDesignTab(p,database,mode==='designTable'?table:undefined);return}if(mode==='query'){createQueryTab(p,database,table);return}if(mode==='openTable'&&table){openTableTab(p,database,table);return}databaseTool.value={profile:p,database,mode,table}}
async function databaseChanged(operation:string,name:string){
 const target=databaseTool.value;if(!target)return
 if(operation==='table'){try{await refreshTableList(target.profile,target.database)}catch(e){showNavigationError(errorText(e))}return}
 try{sessions[target.profile.id]=await api.RefreshDatabases(target.profile.id);if(operation==='drop')closeDatabase(target.profile,target.database);else if(operation==='create'){selectProfile(target.profile);selectedRowKey.value=name;databaseNodeSelected.value=true}}
 catch(e){showNavigationError(errorText(e))}
}
function contextItems():MenuItem[]{
 if(contextMenu.kind==='table'){
  const p=profiles.value.find(p=>p.id===contextMenu.connectionID),db=contextMenu.database,table=contextMenu.table;if(!p)return []
  return ([['createTable','新建表'],['designTable','设计表'],['import','导入向导'],['export','导出向导'],['dictionary','数据字典'],['generate','数据生成'],['run','运行SQL文件...'],['query','在数据库中查询']] as [DatabaseToolMode,string][]).map<MenuItem>(([mode,label])=>({label,disabled:mode==='designTable'&&!table,action:()=>showDatabaseTool(p,db,mode,table)})).concat([{label:'刷新',action:()=>void refreshTableList(p,db)}])
 }

 if(contextMenu.kind==='database'){
  const p=profiles.value.find(p=>p.id===contextMenu.connectionID),database=contextMenu.database;if(!p)return []
  return [{label:databaseIsOpen(p.id,database)?'关闭数据库':'打开数据库',action:()=>{if(databaseIsOpen(p.id,database))closeDatabase(p,database);else void selectDatabase(p,database)}},...([['alter','编辑数据库'],['create','新建数据库'],['drop','删除数据库'],['query','新建查询'],['console','命令行界面'],['run','运行SQL文件'],['dump','转储SQL文件'],['dictionary','数据字典'],['model','逆向数据库到模型']] as [DatabaseToolMode,string][]).map(([mode,label])=>({label,danger:mode==='drop',action:()=>showDatabaseTool(p,database,mode)})),{label:'刷新',action:async()=>{try{sessions[p.id]=await api.RefreshDatabases(p.id);if(databaseIsOpen(p.id,database)){await selectDatabase(p,database);await loadObjects()}}catch(e){showNavigationError(errorText(e))}}}]
 }
 if(contextMenu.kind==='connection')return [...connectionMenu().slice(0,3),{label:'移至分组…',disabled:!selected.value||!!busy.value||(!connectionGroups.value.length&&!selected.value.groupId),action:showGroupDialog},...connectionMenu().slice(3)]
 const items:MenuItem[]=[{label:'新建连接…',disabled:!!busy.value||!!fatalError.value,action:create}]
 if(contextMenu.kind==='root')items.push({label:'新建组',disabled:!!busy.value||!!fatalError.value,action:()=>beginGroupEdit()})
 else items.push({label:'重命名组',disabled:!!busy.value||!!fatalError.value,action:()=>beginGroupEdit(selectedConnectionGroup.value)},{label:'删除组',disabled:!!busy.value||!!fatalError.value,danger:true,action:removeGroup})
 items.push({label:'关闭所有连接',disabled:!!busy.value||!Object.keys(sessions).length,action:disconnectAll})
 return items
}
const contextMenuRef=ref<HTMLElement>()
useMenuDismissal(()=>contextMenu.open?contextMenuRef.value:undefined,closeMenus)
function closeMenus(){menu.value='';contextMenu.open=false}
function showMenu(name:string){contextMenu.open=false;menu.value=menu.value===name?'':name;nextTick(()=>document.querySelector<HTMLButtonElement>('.menu-popup button:not(:disabled)')?.focus())}
function connectionMenu():MenuItem[]{return [{label:live.value?'关闭连接':'打开连接',shortcut:live.value?'':'Enter',disabled:!selected.value||!!busy.value,action:live.value?disconnect:()=>openConnection()},{label:'编辑连接…',disabled:!selected.value||!!busy.value,action:edit},{label:'复制连接…',disabled:!selected.value||!!busy.value,action:duplicate},{label:'删除连接',disabled:!selected.value||!!busy.value,danger:true,action:remove}]}
function itemsFor(name:string):MenuItem[]{
  if(name==='file')return [{label:'新建 MySQL 连接…',shortcut:'Ctrl+N',disabled:!!busy.value||!!fatalError.value,action:create},...connectionMenu(),{label:'退出',action:async()=>{if(desktop.value)await api.QuitApp()}}]
  if(name==='view')return [{label:'刷新',shortcut:'F5',disabled:!live.value||!!busy.value,action:refresh},{label:'导航窗格',checked:showSidebar.value,action:()=>{showSidebar.value=!showSidebar.value}},{label:'信息窗格',checked:showInformation.value,action:()=>{showInformation.value=!showInformation.value}},{label:'详细信息',checked:viewMode.value==='details',action:()=>{viewMode.value='details'}},{label:'图标',checked:viewMode.value==='icons',action:()=>{viewMode.value='icons'}}]
  if(name==='tools')return [{label:'编辑连接…',disabled:!selected.value||!!busy.value,action:edit},{label:'复制主机地址',disabled:!selected.value,action:()=>copyText(selected.value?address(selected.value):'')}]
  if(name==='window')return profiles.value.length?profiles.value.map(p=>({label:p.name,checked:p.id===selectedID.value,action:()=>selectProfile(p)})):[{label:'没有打开的连接',disabled:true,action:()=>{}}]
  return [{label:'关于 Bajiu DB…',action:()=>{about.value=true;nextTick(()=>document.querySelector<HTMLButtonElement>('[data-about-close]')?.focus())}}]
}
function invoke(item:MenuItem){if(item.disabled)return;closeMenus();void item.action()}
function menuKeys(e:KeyboardEvent){if(!['ArrowDown','ArrowUp','Home','End'].includes(e.key))return;e.preventDefault();const nodes=Array.from((e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('button:not(:disabled)'));const i=nodes.indexOf(document.activeElement as HTMLButtonElement);nodes[e.key==='Home'?0:e.key==='End'?nodes.length-1:(i+(e.key==='ArrowDown'?1:nodes.length-1))%nodes.length]?.focus()}
function treeKeys(e:KeyboardEvent){if(!['ArrowDown','ArrowUp','Home','End'].includes(e.key))return;e.preventDefault();const nodes=Array.from(document.querySelectorAll<HTMLElement>('[data-tree-row]'));const i=nodes.indexOf(e.currentTarget as HTMLElement);nodes[e.key==='Home'?0:e.key==='End'?nodes.length-1:(i+(e.key==='ArrowDown'?1:nodes.length-1))%nodes.length]?.focus()}
function visibleDatabases(p:Profile){const dbs=sessions[p.id]?.databases??[];const term=search.value.toLowerCase();return !term||`${p.name} ${p.host}`.toLowerCase().includes(term)?dbs:dbs.filter(db=>db.toLowerCase().includes(term))}
function connectionHasDatabases(id:string){return (sessions[id]?.databases.length??0)>0}
function trap(e:KeyboardEvent){const nodes=Array.from((e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('button:not(:disabled)'));const first=nodes[0],last=nodes[nodes.length-1];if(e.shiftKey&&document.activeElement===first){e.preventDefault();last?.focus()}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first?.focus()}}
function startResize(e:PointerEvent,side:'left'|'right'){e.preventDefault();releaseDrag?.();const start=e.clientX,width=side==='left'?leftWidth.value:rightWidth.value;const move=(event:PointerEvent)=>{const next=width+(event.clientX-start)*(side==='left'?1:-1);if(side==='left')leftWidth.value=Math.min(400,Math.max(170,next));else rightWidth.value=Math.min(360,Math.max(180,next))};const stop=()=>{document.removeEventListener('pointermove',move);document.removeEventListener('pointerup',stop);document.body.style.cursor='';releaseDrag=undefined};document.addEventListener('pointermove',move);document.addEventListener('pointerup',stop,{once:true});document.body.style.cursor='col-resize';releaseDrag=stop}
function resizeKeys(e:KeyboardEvent,side:'left'|'right'){if(e.key!=='ArrowLeft'&&e.key!=='ArrowRight')return;e.preventDefault();const delta=e.key==='ArrowLeft'?-10:10;if(side==='left')leftWidth.value=Math.min(400,Math.max(170,leftWidth.value+delta));else rightWidth.value=Math.min(360,Math.max(180,rightWidth.value-delta))}
async function copyText(value:string){try{await navigator.clipboard.writeText(value);}catch{showNavigationError('无法访问剪贴板')}}
function shortcuts(e:KeyboardEvent){
  if(databaseTool.value)return
  if(e.key==='Escape'){e.preventDefault();if(detailDialog.value){detailDialog.value=undefined}else if(confirm.open){confirm.open=false;nextTick(()=>document.querySelector<HTMLButtonElement>('.connection-dialog .window-close')?.focus())}else if(about.value)about.value=false;else if(groupDialogOpen.value)closeGroupDialog();else if(dialogOpen.value)cancelDialog();else closeMenus();return}
  if(groupDialogOpen.value)return
  if(detailDialog.value||confirm.open||about.value)return
  if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='s'&&dialogOpen.value){e.preventDefault();void saveDialog();return}
  if(dialogOpen.value)return
  if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='n'){e.preventDefault();create()}
  if(e.key==='F5'){e.preventDefault();void refresh()}
  if(e.altKey&&e.key.toLowerCase()==='f'){e.preventDefault();showMenu('file')}
}
watch(()=>JSON.stringify(form),()=>{testResult.value=null;testError.value=''})
watch([leftWidth,rightWidth,showSidebar,showInformation,viewMode],()=>{try{localStorage.setItem('bajiu.workspace-layout',JSON.stringify({left:leftWidth.value,right:rightWidth.value,sidebar:showSidebar.value,info:showInformation.value,view:viewMode.value}))}catch{/* settings are optional */}})
onMounted(async()=>{
 document.addEventListener('keydown',shortcuts)
 try{const preferences=JSON.parse(localStorage.getItem('bajiu.workspace-layout')??'null');if(preferences){if(Number.isFinite(preferences.left))leftWidth.value=Math.min(400,Math.max(170,preferences.left));if(Number.isFinite(preferences.right))rightWidth.value=Math.min(360,Math.max(180,preferences.right));if(typeof preferences.sidebar==='boolean')showSidebar.value=preferences.sidebar;if(typeof preferences.info==='boolean')showInformation.value=preferences.info;if(['details','icons'].includes(preferences.view))viewMode.value=preferences.view}}catch{/* invalid preferences use defaults */}
 try{if(desktop.value){connectionGroups.value=await api.ListGroups();for(const g of connectionGroups.value)expandedGroups.add(g.id);profiles.value=await api.ListConnections();configPath.value=await api.ConfigPath();for(const session of await api.ListSessions())sessions[session.id]=session}}catch(e){fatalError.value=errorText(e)}finally{loading.value=false}
 poll=setInterval(async()=>{if(!desktop.value||busy.value)return;try{const list=await api.ListSessions();const ids=new Set(list.map(s=>s.id));for(const id of Object.keys(sessions))if(!ids.has(id)){delete sessions[id];clearDatabaseState(id);expandedConnections.delete(id);if(id===selectedID.value){selectedDatabase.value='';invalidateObjects()}}}catch{/* explicit refresh reports connection errors */}},15000)
})
const nativeDialogDefinitions=computed(()=>{
 const windows=[]
 if(dialogOpen.value)windows.push({key:'connection',kind:'connection' as const,title:'MySQL - '+(connectOnSave.value?'打开连接':editingID.value?'编辑连接':'新建连接'),width:580,height:580,payload:{model:form,clientVersion:connectionClientVersion.value,busy:busy.value,errors,result:testResult.value,error:testError.value,editing:!!editingID.value,connectOnSave:connectOnSave.value,fatal:!!fatalError.value,groups:connectionGroups.value}})
 if(groupDialogOpen.value)windows.push({key:'group',kind:'group' as const,title:'移至分组',width:420,height:240,payload:{model:groupForm,clientVersion:groupClientVersion.value,groups:connectionGroups.value,busy:!!busy.value,error:groupError.value}})
 if(databaseTool.value){const{profile,database,mode,table,initialSQL}=databaseTool.value;const titles={create:'新建数据库',alter:'编辑数据库',drop:'删除数据库',query:'新建查询',console:'命令行界面',run:'运行 SQL 文件',dump:'转储 SQL 文件',dictionary:'数据字典',model:'逆向数据库到模型',createTable:'新建表',import:'导入向导',export:'导出向导',generate:'数据生成',openTable:'打开表',designTable:'设计表'};const compact=['create','alter','drop'].includes(mode);windows.push({key:'database',kind:'database' as const,title:titles[mode]+' · '+profile.name+' / '+database,width:mode==='drop'?420:compact?600:1000,height:mode==='drop'?320:compact?350:650,payload:{profile,database,mode,table,initialSQL}})}
 if(about.value)windows.push({key:'about',kind:'about' as const,title:'关于 Bajiu DB · 捌玖',width:365,height:310,payload:{}})
 if(detailDialog.value)windows.push({key:'details',kind:'confirm' as const,title:detailDialog.value.title,width:760,height:520,payload:{...detailDialog.value,label:'关闭',notice:true}})
 if(confirm.open)windows.push({key:'confirm',kind:'confirm' as const,title:confirm.title,width:440,height:240,payload:{title:confirm.title,body:confirm.body,label:confirm.label}})
 return windows
})
function nativeDialogAction(key:string,event:DialogAction){
 const action=event.action,payload=event.payload??{}
 if(key==='connection'){
  if(Number(payload.clientVersion??0)>=connectionClientVersion.value){connectionClientVersion.value=Number(payload.clientVersion??0);if(action==='model'&&payload.changes)Object.assign(form,payload.changes);else if(payload.model)Object.assign(form,payload.model)}
  if(action==='save')void saveDialog();else if(action==='test')void testConnection();else if(action==='cancel')cancelDialog();else if(action==='password')passwordChanged();else if(action==='forget')forgetPassword();else if(action==='chooseCA')void chooseCA();else if(action==='chooseSSHKey')void chooseSSHKey()
 }else if(key==='group'){if(Number(payload.clientVersion??0)>=groupClientVersion.value){groupClientVersion.value=Number(payload.clientVersion??0);if(action==='model'&&payload.changes)Object.assign(groupForm,payload.changes);else if(payload.model)Object.assign(groupForm,payload.model)};if(action==='save')void saveGroupDialog();else if(action==='cancel')closeGroupDialog()}
 else if(key==='confirm'){if(action==='accept')void acceptConfirm();else if(action==='cancel')cancelConfirm()}
 else if(key==='details'&&action==='cancel')detailDialog.value=undefined
 else if(key==='about'&&action==='cancel')about.value=false
 else if(key==='database'){if(action==='changed')void databaseChanged(payload.operation,payload.name);else if(action==='cancel')closeDatabaseTool()}
}
useNativeDialogs(nativeDialogDefinitions,nativeDialogAction,showNavigationError)
onUnmounted(()=>{clearInterval(poll);document.removeEventListener('keydown',shortcuts);releaseDrag?.();transientPasswords.clear()})
</script>

<template>
  <div class="desktop-app" @click="closeMenus">
    <div class="desktop-workspace" :inert="dialogOpen||groupDialogOpen||confirm.open||about||!!detailDialog||!!databaseTool||undefined">
      <nav class="application-menubar" aria-label="主菜单">
        <div v-for="entry in [{id:'file',label:'文件(F)'},{id:'view',label:'查看(V)'},{id:'favorites',label:'收藏夹(A)'},{id:'tools',label:'工具(T)'},{id:'window',label:'窗口(W)'},{id:'help',label:'帮助(H)'}]" :key="entry.id" class="application-menu">
          <button :disabled="entry.id==='favorites'" :title="entry.id==='favorites'?'当前版本尚未提供收藏夹':''" :aria-expanded="menu===entry.id" aria-haspopup="menu" @click.stop="showMenu(entry.id)" :class="{open:menu===entry.id}">{{entry.label}}</button>
          <div v-if="menu===entry.id" class="menu-popup" role="menu" @click.stop @keydown="menuKeys"><button v-for="item in itemsFor(entry.id)" :key="item.label" role="menuitem" :disabled="item.disabled" :class="{danger:item.danger}" @click="invoke(item)"><span class="menu-check"><Check v-if="item.checked" :size="13"/></span><span>{{item.label}}</span><kbd v-if="item.shortcut">{{item.shortcut}}</kbd></button></div>
        </div>
        <span class="application-brand">Bajiu DB <span>捌玖</span></span>
      </nav>
      <div class="main-toolbar" role="toolbar" aria-label="主工具栏">
        <button class="large-tool" @click="create" :disabled="!!busy||!!fatalError" aria-label="新建连接"><Plug :size="27" class="connection-tool-icon"/><span>连接<ChevronDown :size="10"/></span></button>
        <button class="large-tool" :disabled="!toolLive||!toolDatabase" @click="openSelectedDatabaseTool('query')" title="选择数据库后新建查询"><FileCode2 :size="27" class="query-tool-icon"/><span>新建查询</span></button>
        <span class="tool-separator"></span>
        <button v-for="c in categories.slice(0,3)" :key="c[0]" class="large-tool category-tool" :class="{active:category===c[0]}" @click="changeCategory(c[0])" :disabled="!!busy"><component :is="c[2]" :size="27" :class="'category-'+c[0]"/><span>{{c[1]}}</span></button>
        <button class="large-tool" disabled title="当前版本尚未提供账户管理"><Users :size="27"/><span>用户</span></button>
        <button class="large-tool category-tool" :class="{active:category==='events'}" @click="changeCategory('events')" :disabled="!!busy"><Wrench :size="26" class="category-events"/><span>其他<ChevronDown :size="10"/></span></button>
        <span class="tool-separator"></span>
        <button class="large-tool" :disabled="!toolLive||!toolDatabase" @click="openSelectedDatabaseTool('query')"><FileText :size="26"/><span>查询</span></button><button class="large-tool" disabled title="当前版本尚未提供备份"><Archive :size="26"/><span>备份</span></button><button class="large-tool" disabled title="当前版本尚未提供自动运行"><Workflow :size="26"/><span>自动运行</span></button><button class="large-tool" :disabled="!toolLive||!toolDatabase" @click="openSelectedDatabaseTool('model')"><Network :size="26"/><span>模型</span></button><button class="large-tool" disabled title="当前版本尚未提供图表"><ChartNoAxesColumn :size="26"/><span>图表</span></button>
      </div>
      <div v-if="!desktop" class="workspace-banner">界面预览模式。请在桌面版中保存和连接真实 MySQL。</div>
      <div v-if="fatalError" class="workspace-banner error" role="alert"><AlertCircle :size="14"/>{{fatalError}}；当前已停止保存操作。</div>
      <div class="workspace-body">
        <aside v-if="showSidebar" class="navigation-pane" :style="{width:leftWidth+'px'}" aria-label="连接导航">
<div v-if="navigationError" class="navigation-error" role="alert"><AlertCircle :size="13"/><span>{{navigationError}}</span><button class="tiny-button" aria-label="关闭错误提示" @click="navigationError=''" ><X :size="12"/></button></div>
          <div class="navigation-scroll" @dragover.self="overConnectionTarget($event,'')" @dragleave.self="leaveConnectionTarget($event,'')" @drop.self.stop="dropConnection($event,'')" role="tree" aria-label="数据库连接树">
            <button class="tree-row tree-root" :class="{'drop-target':dropTarget===''}" @dragover.stop="overConnectionTarget($event,'')" @dragleave.stop="leaveConnectionTarget($event,'')" @drop.stop="dropConnection($event,'')" role="treeitem" :aria-expanded="rootExpanded" data-tree-row @contextmenu.prevent.stop="contextForRoot($event)" @click="selectRoot" @dblclick="rootExpanded=!rootExpanded" @keydown="treeKeys" @keydown.right.prevent="rootExpanded=true" @keydown.left.prevent="rootExpanded=false"><span class="tree-expander" @click.stop="rootExpanded=!rootExpanded"><ChevronDown v-if="rootExpanded" :size="12"/><ChevronRight v-else :size="12"/></span><FolderOpen :size="16" class="folder-icon"/><span>我的连接</span></button>
            <div v-if="rootExpanded" role="group">
              <InlineGroupEditor v-if="groupEditor.active&&!groupEditor.id" v-model:name="groupEditor.name" :busy="busy==='group-edit'" :error="groupEditor.error" @save="saveInlineGroup" @cancel="cancelGroupEdit"/>
              <template v-for="section in navigationSections" :key="section.id">
                <InlineGroupEditor v-if="section.id&&groupEditor.active&&groupEditor.id===section.id" v-model:name="groupEditor.name" :busy="busy==='group-edit'" :error="groupEditor.error" @save="saveInlineGroup" @cancel="cancelGroupEdit"/>
                <button v-if="section.id&&!(groupEditor.active&&groupEditor.id===section.id)" class="tree-row tree-group" :class="{selected:selectedGroupID===section.id&&!selectedID,'drop-target':dropTarget===section.id}" @dragover.stop="overConnectionTarget($event,section.id)" @dragleave.stop="leaveConnectionTarget($event,section.id)" @drop.stop="dropConnection($event,section.id)" role="treeitem" :aria-expanded="section.profiles.length?expandedGroups.has(section.id):undefined" :aria-selected="selectedGroupID===section.id&&!selectedID" data-tree-row @click="selectConnectionGroup(section.id)" @dblclick="toggleGroup(section.id)" @contextmenu.prevent.stop="contextForGroup($event,section.id)" @keydown="treeKeys" @keydown.enter.prevent="toggleGroup(section.id)" @keydown.right.prevent="expandedGroups.add(section.id)" @keydown.left.prevent="expandedGroups.delete(section.id)"><span v-if="section.profiles.length" class="tree-expander" @click.stop="toggleGroup(section.id)"><ChevronDown v-if="expandedGroups.has(section.id)" :size="12"/><ChevronRight v-else :size="12"/></span><span v-else class="tree-expander" aria-hidden="true"></span><Folder :size="16" class="folder-icon"/><span class="tree-label">{{section.name}}</span></button>
                <div v-if="!section.id||expandedGroups.has(section.id)" :class="{'group-members':!!section.id}" role="group">
              <template v-for="p in section.profiles" :key="p.id">
                <button class="tree-row tree-connection" :draggable="!busy&&!loading&&!dialogOpen&&!groupDialogOpen&&!groupEditor.active" @dragstart.stop="startConnectionDrag($event,p)" @dragend.stop="endConnectionDrag" :class="{dragging:draggedConnectionID===p.id,selected:selectedID===p.id&&!selectedDatabase&&!selectedRow?.database,connected:!!sessions[p.id]}" role="treeitem" :aria-expanded="connectionHasDatabases(p.id)?expandedConnections.has(p.id):undefined" :aria-selected="selectedID===p.id&&!selectedDatabase&&!selectedRow?.database" data-tree-row :title="p.name+' · '+address(p)" @click="selectProfile(p)" @dblclick="toggleConnection(p)" @contextmenu.prevent.stop="contextFor($event,p)" @keydown="treeKeys" @keydown.enter.prevent="openConnection(p)" @keydown.right.prevent="expandedConnections.has(p.id)||toggleConnection(p)" @keydown.left.prevent="expandedConnections.delete(p.id)"><span v-if="connectionHasDatabases(p.id)" class="tree-expander" @click.stop="toggleConnection(p)"><ChevronDown v-if="expandedConnections.has(p.id)" :size="12"/><ChevronRight v-else :size="12"/></span><span v-else class="tree-expander" aria-hidden="true"></span><Plug :size="16"/><span class="tree-label">{{p.name}}</span><LoaderCircle v-if="(busy==='connect'&&selectedID===p.id)||movingConnectionID===p.id" :size="12" class="spin"/></button>
                <div v-if="expandedConnections.has(p.id)&&connectionHasDatabases(p.id)" role="group">
                  <template v-for="db in visibleDatabases(p)" :key="db">
                    <button class="tree-row tree-database" :class="{selected:selectedID===p.id&&databaseNodeSelected&&(selectedDatabase===db||selectedRow?.database===db)}" role="treeitem" :aria-expanded="databaseIsOpen(p.id,db)?expandedDatabases.has(p.id+'/'+db):undefined" :aria-selected="selectedID===p.id&&databaseNodeSelected&&(selectedDatabase===db||selectedRow?.database===db)" data-tree-row :title="db" @click="selectDatabaseInfo(p,db)" @contextmenu.prevent.stop="contextForDatabase($event,p,db)" @dblclick="toggleDatabase(p,db)" @keydown="treeKeys" @keydown.enter.prevent="selectDatabase(p,db)" @keydown.right.prevent="selectDatabase(p,db)" @keydown.left.prevent="expandedDatabases.delete(p.id+'/'+db)"><span v-if="databaseIsOpen(p.id,db)" class="tree-expander" @click.stop="toggleDatabase(p,db)"><ChevronDown v-if="expandedDatabases.has(p.id+'/'+db)" :size="12"/><ChevronRight v-else :size="12"/></span><span v-else class="tree-expander" aria-hidden="true"></span><Database :size="15" :class="databaseIconClass(p.id,db)"/><span class="tree-label">{{db}}</span></button>
                    <div v-if="databaseIsOpen(p.id,db)&&expandedDatabases.has(p.id+'/'+db)" role="group">
                     <template v-for="c in categories" :key="c[0]">
                      <button class="tree-row tree-category" :class="{selected:selectedID===p.id&&selectedDatabase===db&&!databaseNodeSelected&&category===c[0]&&!selectedObject}" role="treeitem" :aria-expanded="categoryExpandable(p,db,c[0])?expandedCategories.has(treeKey(p.id,db,c[0])):undefined" data-tree-row @click="selectDatabase(p,db,c[0])" @dblclick="toggleCategory(p,db,c[0])" @contextmenu.prevent.stop="c[0]==='tables'&&contextForTable($event,p,db)" @keydown="treeKeys" @keydown.enter.prevent="toggleCategory(p,db,c[0])" @keydown.right.prevent="toggleCategory(p,db,c[0],true)" @keydown.left.prevent="toggleCategory(p,db,c[0],false)"><span v-if="categoryExpandable(p,db,c[0])" class="tree-expander" @click.stop="toggleCategory(p,db,c[0])"><ChevronDown v-if="expandedCategories.has(treeKey(p.id,db,c[0]))" :size="12"/><ChevronRight v-else :size="12"/></span><span v-else class="tree-expander"></span><component :is="c[2]" :size="15" :class="'category-'+c[0]"/><span>{{c[1]}}</span></button>
                      <div v-if="expandedCategories.has(treeKey(p.id,db,c[0]))" role="group">
                       <template v-for="object in treeList(p,db,c[0])" :key="object.name">
                        <button class="tree-row tree-table" role="treeitem" data-tree-row :aria-expanded="c[0]==='tables'?expandedTables.has(treeKey(p.id,db,object.name)):undefined" :class="{selected:activeDocument?.kind==='table'?activeDocument.profile.id===p.id&&activeDocument.database===db&&activeDocument.table===object.name:selectedID===p.id&&selectedDatabase===db&&category===c[0]&&selectedObject?.name===object.name}" @click="selectTreeObject(p,db,c[0],object)" @dblclick="c[0]==='tables'&&openTableTab(p,db,object.name)" @contextmenu.prevent.stop="c[0]==='tables'&&contextForTable($event,p,db,object)" @keydown="treeKeys" @keydown.enter.prevent="c[0]==='tables'?openTableTab(p,db,object.name):selectTreeObject(p,db,c[0],object)" @keydown.right.prevent="c[0]==='tables'&&toggleTable(p,db,object,true)" @keydown.left.prevent="toggleTable(p,db,object,false)"><span class="tree-expander" @click.stop="c[0]==='tables'&&toggleTable(p,db,object)"><template v-if="c[0]==='tables'"><ChevronDown v-if="expandedTables.has(treeKey(p.id,db,object.name))" :size="12"/><ChevronRight v-else :size="12"/></template></span><component :is="c[2]" :size="14" :class="'category-'+c[0]"/><span class="tree-label">{{object.name}}</span></button>
                        <div v-if="c[0]==='tables'&&expandedTables.has(treeKey(p.id,db,object.name))" role="group"><button v-for="detail in tableDetails" :key="detail[0]" class="tree-row tree-table-detail" role="treeitem" data-tree-row :class="{selected:activeDocument?.kind==='table'&&activeDocument.profile.id===p.id&&activeDocument.database===db&&activeDocument.table===object.name&&activeDocument.section===detail[0]}" @click="openTableTab(p,db,object.name,detail[0])" @keydown="treeKeys" @keydown.enter.prevent="openTableTab(p,db,object.name,detail[0])"><component :is="detail[2]" :size="13"/><span>{{detail[1]}}</span></button></div>
                       </template>
                      </div>
                     </template>
                    </div>
                  </template>
                  <p v-if="sessions[p.id].databases.length===0" class="tree-caption">没有可访问的数据库</p>
                </div>
              </template>
                </div></template>
              <p v-if="loading" class="tree-caption">正在读取连接…</p><p v-else-if="filteredProfiles.length===0&&profiles.length>0" class="tree-caption">没有匹配的连接</p>
            </div>
          </div>
          <div v-if="draggedConnection?.groupId" class="tree-root-drop-zone" :class="{'drop-target':dropTarget===''}" @dragover.stop="overConnectionTarget($event,'')" @dragleave.stop="leaveConnectionTarget($event,'')" @drop.stop="dropConnection($event,'')">移回“我的连接”</div>
          <div class="navigation-filter"><Search :size="12"/><input v-model="search" aria-label="搜索连接" placeholder="搜索"/><button v-if="search" class="tiny-button" @click="search=''" aria-label="清空连接搜索"><X :size="12"/></button><select v-model="group" aria-label="按环境筛选连接" title="按环境筛选连接"><option v-for="g in groups" :key="g.id" :value="g.id">{{g.label}}</option></select></div>
        </aside>
        <div v-if="showSidebar" class="pane-splitter" role="separator" aria-label="调整导航窗格宽度" aria-orientation="vertical" tabindex="0" :aria-valuenow="leftWidth" aria-valuemin="170" aria-valuemax="400" @pointerdown="startResize($event,'left')" @keydown="resizeKeys($event,'left')"></div>
        <main class="object-pane">
          <div class="object-tabs">
           <div class="object-tab-strip" role="tablist" aria-label="工作区标签页">
            <button id="workspace-tab-objects" class="object-tab" :class="{active:activeWorkspaceTab==='objects'}" role="tab" :aria-selected="activeWorkspaceTab==='objects'" aria-controls="workspace-panel-objects" :tabindex="activeWorkspaceTab==='objects'?0:-1" @click="activateWorkspaceTab('objects')" @keydown="workspaceTabKeys($event,'objects')"><FolderOpen :size="14"/>对象</button>
            <div v-for="tab in workspaceTabs" :key="tab.id" class="query-workspace-tab" :class="{active:activeWorkspaceTab===tab.id}" role="presentation">
             <button :id="'workspace-tab-'+tab.id" class="object-tab query-tab-button" :class="{active:activeWorkspaceTab===tab.id}" role="tab" :aria-selected="activeWorkspaceTab===tab.id" :aria-controls="'workspace-panel-'+tab.id" :tabindex="activeWorkspaceTab===tab.id?0:-1" :title="tab.title+' · '+tab.profile.name+' / '+tab.database" @click="activateWorkspaceTab(tab.id)" @keydown="workspaceTabKeys($event,tab.id)"><component :is="tab.kind==='table'?Table2:tab.kind==='design'?Pencil:FileCode2" :size="14"/><span>{{tab.title}}{{tab.kind!=='query'&&tab.dirty?' *':''}}</span></button>
             <button class="query-tab-close" :tabindex="activeWorkspaceTab===tab.id?0:-1" :aria-label="'关闭'+tab.title" :title="'关闭'+tab.title" @click.stop="closeWorkspaceTab(tab.id)"><X :size="12"/></button>
            </div>
           </div>
           <div class="object-location"><template v-if="activeDocument"><span>{{activeDocument.profile.name}}</span><ChevronRight :size="11"/><span>{{activeDocument.database}}</span></template><template v-else><span v-if="selectedConnectionGroup">{{selectedConnectionGroup.name}}</span><ChevronRight v-if="selectedConnectionGroup&&selected" :size="11"/><span v-if="selected">{{selected.name}}</span><ChevronRight v-if="selectedDatabase" :size="11"/><span v-if="selectedDatabase">{{selectedDatabase}}</span></template></div>
           <button v-if="activeWorkspaceTab==='objects'" class="tiny-button info-toggle" :class="{pressed:showInformation}" aria-label="切换信息窗格" :aria-pressed="showInformation" @click="showInformation=!showInformation"><Info :size="14"/></button>
          </div>
          <div v-show="activeWorkspaceTab==='objects'" id="workspace-panel-objects" class="object-workspace" role="tabpanel" aria-labelledby="workspace-tab-objects">
          <div class="object-toolbar" :class="{'table-object-toolbar':tableToolbarVisible}" role="toolbar" aria-label="对象工具栏">
           <template v-if="tableToolbarVisible">
            <button @click="openTableTool('openTable')" :disabled="tableToolbarDisabled||!toolbarTable"><Table2 :size="14"/>打开表</button>
            <button @click="openTableTool('designTable')" :disabled="tableToolbarDisabled||!toolbarTable"><Pencil :size="13"/>设计表</button>
            <button @click="openTableTool('createTable')" :disabled="tableToolbarDisabled"><Plus :size="14" class="add-color"/>新建表</button>
            <button @click="deleteToolbarTable" :disabled="tableToolbarDisabled||!toolbarTable"><Trash2 :size="13"/>删除表</button>
            <button @click="openTableTool('import')" :disabled="tableToolbarDisabled"><FolderOpen :size="14"/>导入向导</button>
            <button @click="openTableTool('export')" :disabled="tableToolbarDisabled"><Archive :size="14"/>导出向导</button>
           </template>
           <template v-else><button :disabled="!selectedRow" @click="showInformation=true"><Info :size="14"/>查看信息</button><button @click="edit" :disabled="!selected||!!busy"><Pencil :size="13"/>编辑连接</button><button @click="create" :disabled="!!busy||!!fatalError"><Plus :size="14" class="add-color"/>新建连接</button><button @click="disconnect" :disabled="!live||!!busy"><Unplug :size="14"/>关闭连接</button><button @click="refresh" :disabled="!live||!!busy||objectsLoading"><RefreshCw :size="13" :class="{spin:objectsLoading||busy==='refresh'}"/>刷新</button></template>
           <div class="object-search"><Search :size="12"/><input v-model="objectSearch" aria-label="筛选对象" placeholder="筛选"/></div>
          </div>
          <div v-if="objectsError" class="objects-error" role="alert"><AlertCircle :size="15"/>{{objectsError}}<button class="tiny-button" aria-label="关闭错误提示" @click="objectsError=''"><X :size="13"/></button></div>
          <div class="object-list-scroll" :aria-busy="objectsLoading">
            <table v-if="viewMode==='details'" class="object-table" aria-label="数据库对象列表"><thead><tr><th v-for="column in columns" :key="column" :title="column==='行'?'MySQL 提供的估计行数':undefined">{{column}}</th></tr></thead><tbody><tr v-for="row in workspaceRows" :key="row.key" :class="{selected:row.key===selectedRowKey}" tabindex="0" :aria-selected="row.key===selectedRowKey" @contextmenu.prevent.stop="contextForWorkspaceRow($event,row)" @click="selectWorkspaceRow(row)" @dblclick="openRow(row)" @keydown.enter.prevent="openRow(row)" @keydown.space.prevent="selectWorkspaceRow(row)"><td class="object-name"><component :is="row.kind==='connection'?Plug:row.kind==='database'?Database:row.kind==='tables'?Table2:row.kind==='views'?PanelsTopLeft:Braces" :size="15" :class="row.kind==='database'?databaseIconClass(selectedID,row.database??row.name):'category-'+row.kind"/><span>{{row.name}}</span></td><template v-if="selectedDatabase&&(category==='tables'||category==='views')"><td class="numeric">{{row.object?.autoIncrement||''}}</td><td>{{row.object?.modifiedAt||''}}</td><td class="numeric">{{bytesText(row.object?.dataLength??'')}}</td><td>{{row.object?.engine||''}}</td><td class="numeric" title="服务器提供的估计行数">{{row.object?.rows||''}}</td><td>{{row.object?.comment||''}}</td></template><template v-else-if="selectedDatabase"><td>{{row.object?.engine||''}}</td><td>{{row.object?.modifiedAt||''}}</td><td>{{row.object?.createdAt||''}}</td><td>{{row.object?.comment||''}}</td></template><template v-else-if="selected"><td>数据库</td></template><template v-else><td>{{row.profile?.host}}</td><td>{{row.profile?.port}}</td><td>{{row.profile?.username}}</td><td>{{envName(row.profile?.environment??'')}}</td><td>{{row.profile&&sessions[row.profile.id]?'已打开':'已关闭'}}</td></template></tr></tbody></table>
            <div v-else class="object-icon-grid"><button v-for="row in workspaceRows" :key="row.key" :class="{selected:row.key===selectedRowKey}" @contextmenu.prevent.stop="contextForWorkspaceRow($event,row)" @click="selectWorkspaceRow(row)" @dblclick="openRow(row)" @keydown.enter.prevent="openRow(row)"><component :is="row.kind==='connection'?Plug:row.kind==='database'?Database:row.kind==='tables'?Table2:row.kind==='views'?PanelsTopLeft:Braces" :size="28" :class="row.kind==='database'?databaseIconClass(selectedID,row.database??row.name):'category-'+row.kind"/><span>{{row.name}}</span></button></div>
            <div v-if="objectsLoading" class="object-empty" role="status"><LoaderCircle :size="20" class="spin"/><span>正在读取{{categoryName}}…</span></div><div v-else-if="workspaceRows.length===0" class="object-empty"><span v-if="objectSearch">没有匹配的对象</span><template v-else-if="!selected"><span>{{selectedGroupID?'此组还没有连接，可新建连接或将已有连接移入此组。':'请选择左侧连接，或新建一个 MySQL 连接。'}}</span><button class="link-button" @click="create" :disabled="!!fatalError">新建连接</button></template><span v-else-if="!live">双击左侧连接以打开服务器。</span><span v-else>{{selectedDatabase?'此数据库中没有可见的'+categoryName:'没有可访问的数据库'}}</span></div>
          </div>
          <div class="object-list-status"><span>{{workspaceRows.length}} 个{{scopeName}}<span v-if="truncated">（最多显示 10,000 项，请缩小范围）</span></span><span v-if="selectedDatabase&&category==='tables'">行数为服务器估计值</span></div>
          </div>
          <QueryEditor v-for="tab in queryDocuments" v-show="activeWorkspaceTab===tab.id" :id="'workspace-panel-'+tab.id" :key="tab.id" role="tabpanel" :aria-labelledby="'workspace-tab-'+tab.id" :profile="tab.profile" :database="tab.database" :initialSQL="tab.initialSQL" :active="activeWorkspaceTab===tab.id" :connected="!!sessions[tab.profile.id]"/>
          <TableDesigner v-for="tab in designDocuments" v-show="activeWorkspaceTab===tab.id" :id="'workspace-panel-'+tab.id" :key="tab.id" role="tabpanel" :aria-labelledby="'workspace-tab-'+tab.id" :profile="tab.profile" :database="tab.database" :table="tab.table" :active="activeWorkspaceTab===tab.id" :connected="!!sessions[tab.profile.id]" :sessionKey="sessions[tab.profile.id]?.connectedAt??''" @dirty="tab.dirty=$event" @saved="name=>designSaved(tab,name)" @details="detailDialog=$event" @confirm="request=>ask(request.title,request.body,request.label,request.action,true)"/>
          <TableDataEditor v-for="tab in tableDocuments" v-show="activeWorkspaceTab===tab.id" :id="'workspace-panel-'+tab.id" :key="tab.id" role="tabpanel" :aria-labelledby="'workspace-tab-'+tab.id" :profile="tab.profile" :database="tab.database" :table="tab.table" :section="tab.section" :active="activeWorkspaceTab===tab.id" :connected="!!sessions[tab.profile.id]" :sessionKey="sessions[tab.profile.id]?.connectedAt??''" @dirty="tab.dirty=$event" @section="tab.section=$event" @confirm="request=>ask(request.title,request.body,request.label,request.action,true)"/>
        </main>
        <div v-if="showInformation&&activeWorkspaceTab==='objects'" class="pane-splitter" role="separator" aria-label="调整信息窗格宽度" aria-orientation="vertical" tabindex="0" :aria-valuenow="rightWidth" aria-valuemin="180" aria-valuemax="360" @pointerdown="startResize($event,'right')" @keydown="resizeKeys($event,'right')"></div>
        <aside v-if="showInformation&&activeWorkspaceTab==='objects'" class="information-pane" :style="{width:rightWidth+'px'}" aria-label="对象信息"><div class="information-tabs"><button :class="{active:informationTab==='info'}" @click="informationTab='info'">信息</button><button v-if="canShowDDL" :class="{active:informationTab==='ddl'}" @click="showDDL">DDL</button><button class="tiny-button" aria-label="关闭信息窗格" @click="showInformation=false"><X :size="12"/></button></div><div v-if="informationTab==='ddl'&&canShowDDL" class="ddl-panel"><span v-if="ddlLoading" role="status">正在读取表定义…</span><p v-else-if="ddlError" role="alert">{{ddlError}}</p><template v-else><button class="link-button ddl-copy-button" @click="copyDDL" :disabled="!ddlText||ddlCopying">复制表定义<CheckCircle2 v-if="ddlCopied" :key="ddlCopyVersion" :size="14" class="ddl-copy-complete" aria-hidden="true"/><span v-if="ddlCopied" class="ddl-copy-status" role="status">已复制表定义</span></button><p v-if="ddlCopyError" role="alert">{{ddlCopyError}}</p><pre>{{ddlText}}</pre></template></div><div v-else class="information-scroll"><div v-if="selected||selectedRow" class="information-heading"><component :is="selectedObject?Table2:selectedDatabase||selectedRow?.database?Database:Plug" :size="48" :class="selectedObject?'category-tables':selectedDatabase||selectedRow?.database?databaseIconClass(selectedID,selectedDatabase||selectedRow?.database||''):'database-icon'"/><div><strong>{{informationName||selected?.name}}</strong><small>{{selectedObject?categoryName:selectedDatabase||selectedRow?.database?'数据库':'MySQL 连接'}}</small></div></div><div v-else class="information-placeholder"><Info :size="22"/><p>选择一个连接或对象<br/>查看详细信息</p></div><div v-if="selectedObject" class="information-context"><span><Plug :size="13"/><span>{{selected?.name}}</span></span><span><Database :size="13"/><span>{{selectedDatabase}}</span></span></div><dl v-if="selected||selectedRow" class="information-details"><template v-if="selectedObject"><dt>行</dt><dd>{{selectedObject.rows||'—'}}<small v-if="selectedObject.rows">估计值</small></dd><dt>引擎</dt><dd>{{selectedObject.engine||'—'}}</dd><dt>自动递增值</dt><dd>{{selectedObject.autoIncrement||'—'}}</dd><dt>创建日期</dt><dd>{{selectedObject.createdAt||'—'}}</dd><dt>修改日期</dt><dd>{{selectedObject.modifiedAt||'—'}}</dd><dt>数据长度</dt><dd>{{bytesText(selectedObject.dataLength)}}</dd><dt>索引长度</dt><dd>{{bytesText(selectedObject.indexLength)}}</dd><dt>注释</dt><dd>{{selectedObject.comment||'—'}}</dd></template><template v-else-if="selectedDatabase||selectedRow?.database"><dt>数据库</dt><dd>{{selectedDatabase||selectedRow?.database}}</dd><dt>连接</dt><dd>{{selected?.name}}</dd><dt>服务器版本</dt><dd>{{live?.serverVersion||'—'}}</dd><dt>当前账户</dt><dd>{{live?.currentUser||'—'}}</dd><template v-if="selectedDatabase"><dt>对象分类</dt><dd>{{categoryName}}</dd><dt>已列出的对象</dt><dd>{{objects.length}}</dd></template></template><template v-else><dt>主机</dt><dd>{{(selected||selectedRow?.profile)?.host}}</dd><dt>端口</dt><dd>{{(selected||selectedRow?.profile)?.port}}</dd><dt>用户名</dt><dd>{{(selected||selectedRow?.profile)?.username}}</dd><dt>环境</dt><dd>{{envName((selected||selectedRow?.profile)?.environment??'')}}</dd><dt>状态</dt><dd>{{live?'已打开':'已关闭'}}</dd><dt>服务器版本</dt><dd>{{live?.serverVersion||'—'}}</dd><dt>密码保存</dt><dd>{{(selected||selectedRow?.profile)?.hasPassword?'系统凭据库':'不保存'}}</dd><dt>修改日期</dt><dd>{{dateText((selected||selectedRow?.profile)?.updatedAt??'')}}</dd><dt>备注</dt><dd>{{(selected||selectedRow?.profile)?.notes||'—'}}</dd></template></dl></div></aside>
      </div>
      <footer class="application-statusbar"><span>{{activeDocument?activeDocument.title:selectedRow?'已选择 1 项':objectsLoading?'正在读取…':'就绪'}}</span><span v-if="activeDocument" class="status-context"><Plug :size="12"/>{{activeDocument.profile.name}}<Database :size="12"/>{{activeDocument.database}}</span><span class="status-context" v-else-if="selected"><Plug :size="12"/>{{selected.name}}<template v-if="selectedDatabase"><Database :size="12"/>{{selectedDatabase}}</template></span><span class="status-security" :title="configPath"><ShieldCheck :size="12"/>本机存储</span><div v-show="activeWorkspaceTab==='objects'" class="view-buttons"><button :class="{active:viewMode==='icons'}" aria-label="图标视图" :aria-pressed="viewMode==='icons'" @click="viewMode='icons'"><LayoutGrid :size="13"/></button><button :class="{active:viewMode==='details'}" aria-label="详细信息视图" :aria-pressed="viewMode==='details'" @click="viewMode='details'"><List :size="13"/></button><button :class="{active:showInformation}" aria-label="显示或隐藏信息窗格" :aria-pressed="showInformation" @click="showInformation=!showInformation"><PanelRight :size="13"/></button></div></footer>
    </div>
    <DatabaseTools v-if="!nativeWindows&&databaseTool" :profile="databaseTool.profile" :database="databaseTool.database" :mode="databaseTool.mode" :table="databaseTool.table" :initialSQL="databaseTool.initialSQL" @close="closeDatabaseTool" @changed="databaseChanged"/>
    <ConnectionDialog v-if="!nativeWindows&&dialogOpen" ref="dialogRef" :inert="confirm.open||undefined" :model="form" :groups="connectionGroups" :editing="!!editingID" :connect-on-save="connectOnSave" :busy="busy" :errors="errors" :result="testResult" :error="testError" :fatal="!!fatalError||!desktop" @save="saveDialog" @cancel="cancelDialog" @test="testConnection" @password="passwordChanged" @forget="forgetPassword" @choose-c-a="chooseCA" @choose-ssh-key="chooseSSHKey"/>
    <GroupDialog v-if="!nativeWindows&&groupDialogOpen" :model="groupForm" :groups="connectionGroups" :busy="!!busy" :error="groupError" @save="saveGroupDialog" @cancel="closeGroupDialog"/>
    <div v-if="contextMenu.open" ref="contextMenuRef" class="connection-context-menu menu-popup" role="menu" :style="{left:contextMenu.x+'px',top:contextMenu.y+'px'}" @click.stop @keydown="menuKeys"><button v-for="item in contextItems()" :key="item.label" role="menuitem" :disabled="item.disabled" :class="{danger:item.danger}" @click="invoke(item)">{{item.label}}<kbd>{{item.shortcut}}</kbd></button></div>
    <ConfirmationDialog v-if="!nativeWindows&&confirm.open" :title="confirm.title" :body="confirm.body" :label="confirm.label" @cancel="cancelConfirm" @accept="acceptConfirm"/>
    <ConfirmationDialog v-if="!nativeWindows&&detailDialog" v-bind="detailDialog" label="关闭" notice @cancel="detailDialog=undefined" @accept="detailDialog=undefined"/>
    <AboutDialog v-if="!nativeWindows&&about" @close="about=false"/>

  </div>
</template>
