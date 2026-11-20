export const APP_ICON = '/appicon.png'
export interface Profile {
  id:string; revision:number; name:string; groupId?:string; host:string; port:number; username:string; database:string;
  environment:string; notes:string; tlsMode:string; caPath:string; serverName:string; timeout:number;
  hasPassword:boolean; createdAt:string; updatedAt:string;
  tunnelMode:'direct'|'ssh'|'http';sshHost:string;sshPort:number;sshUsername:string;sshAuth:'password'|'key';sshKeyPath:string;sshKnownHostsPath:string;sshFingerprint:string;httpUrl:string;
}
export interface ConnectionInput extends Profile { password:string; rememberPassword:boolean; passwordAction:'keep'|'replace'|'forget';sshPassword:string;sshKeyPassphrase:string;httpToken:string;sshPasswordAction:'keep'|'replace';sshKeyPassphraseAction:'keep'|'replace';httpTokenAction:'keep'|'replace' }
export type ConnectionRequest = Omit<ConnectionInput,'createdAt'|'updatedAt'|'hasPassword'>
export interface SessionInfo { id:string; connected:boolean; serverVersion:string; currentUser:string; databases:string[]; latencyMs:number; connectedAt:string }
export interface TestResult {success:boolean; message:string; serverVersion:string; latencyMs:number}
export interface DatabaseObject {name:string;kind:string;engine:string;rows:string;dataLength:string;indexLength:string;autoIncrement:string;comment:string;createdAt:string;modifiedAt:string}
export interface ObjectList {objects:DatabaseObject[];truncated:boolean}
export interface ConnectionGroup {id:string;name:string}
export interface AdminRequest {operation:string;name?:string;charset?:string;collation?:string;sql?:string;sqlMode?:string;includeData?:boolean;transactionId?:string}
export interface QueryResult {columns:string[];rows:(string|null)[][];affected:string;truncated:boolean}
export interface AdminResult {results:QueryResult[];completed:number;error:string;errorCode?:number;text:string;transactionId?:string;transactionActive?:boolean}
export interface DialogDetails {title:string;body:string;details:string;sql?:string}
export interface WindowDialog {id:string;kind:'connection'|'group'|'confirm'|'about'|'database'|'index-fields'|'cell-value';title:string;width:number;height:number;payload:any;revision?:number}
export interface DialogAction {id:string;action:string;payload:any}
export interface TransferFormat {id:string;label:string;available:boolean;note:string}
export interface TransferData {columns:string[];rows:(string|null)[][]}
export interface TransferOptions {format:string;path:string;table:string;delimiter:string;connectionString:string;protectFormulas:boolean}
type Bridge = {
  ListGroups():Promise<ConnectionGroup[]>; SaveGroup(id:string,name:string):Promise<ConnectionGroup>;
  DeleteGroup(id:string):Promise<void>;
  MoveConnectionToGroup(id:string,revision:number,groupId:string):Promise<Profile>; DisconnectAll():Promise<void>;
  ListConnections():Promise<Profile[]>; SaveConnection(input:ConnectionRequest):Promise<Profile>;
  DeleteConnection(id:string,revision:number):Promise<void>; TestConnection(input:ConnectionRequest):Promise<TestResult>;
  Connect(input:ConnectionRequest):Promise<SessionInfo>; Disconnect(id:string):Promise<void>;
  ListSessions():Promise<SessionInfo[]>; RefreshDatabases(id:string):Promise<SessionInfo>;
  ListObjects(id:string,database:string,kind:string):Promise<ObjectList>;
  ConfigPath():Promise<string>; SelectCAFile():Promise<string>;
  SelectSSHKey():Promise<string>;TableDDL(id:string,database:string,table:string):Promise<string>;
  QuitApp():Promise<void>;ConfirmQuitApp():Promise<void>;MainWindowReady():Promise<void>;
  DatabaseAdmin(task:string,id:string,database:string,request:AdminRequest):Promise<AdminResult>;
  CancelDatabaseTask(task:string):Promise<void>;OpenSQLFile():Promise<{path:string;text:string;statements:number}>;
  SaveDatabaseReport(name:string,text:string):Promise<string>;
  DumpDatabaseToFile(task:string,id:string,database:string,includeData:boolean):Promise<string>;
  OpenCSVFile():Promise<{path:string;text:string}>;
  TransferFormats():Promise<TransferFormat[]>;SelectTransferFile(format:string):Promise<string>;
  InspectTransferSource(task:string,options:TransferOptions):Promise<string[]>;
  ReadTransferSource(task:string,options:TransferOptions):Promise<TransferData>;
  SaveTransferData(task:string,options:TransferOptions,data:TransferData):Promise<string>;
  OpenWindowDialog(state:WindowDialog):Promise<void>;GetWindowDialog(id:string):Promise<WindowDialog>;UpdateWindowDialog(state:WindowDialog):Promise<void>;CloseWindowDialog(id:string):Promise<void>;SendWindowDialogAction(action:DialogAction):Promise<void>;
}
declare global { interface Window {go?:{main:{App:Bridge}}} }
let nativeCall:((name:string,...args:unknown[])=>Promise<unknown>)|undefined
export function installNativeBridge(call:typeof nativeCall){nativeCall=call}
export const hasNativeWindows=()=>!!nativeCall
export const isDesktop = () => !!window.go?.main?.App||!!nativeCall
export const api = new Proxy({} as Bridge,{get(_target,key){return (...args:unknown[])=>{
  const bridge = window.go?.main?.App
  if(!bridge){if(nativeCall)return nativeCall('main.App.'+String(key),...args);return Promise.reject(new Error('这是界面预览。请打开 Bajiu DB 桌面程序管理真实连接。'))}
  const fn = bridge[key as keyof Bridge] as (...args:unknown[])=>unknown
  return fn(...args)
}}})
export function defaults():ConnectionInput {return {id:'',revision:0,name:'',groupId:'',host:'127.0.0.1',port:3306,username:'root',database:'',environment:'local',notes:'',tlsMode:'disabled',caPath:'',serverName:'',timeout:10,hasPassword:false,createdAt:'',updatedAt:'',password:'',rememberPassword:true,passwordAction:'replace',tunnelMode:'direct',sshHost:'',sshPort:22,sshUsername:'',sshAuth:'password',sshKeyPath:'',sshKnownHostsPath:'',sshFingerprint:'',httpUrl:'',sshPassword:'',sshKeyPassphrase:'',httpToken:'',sshPasswordAction:'replace',sshKeyPassphraseAction:'replace',httpTokenAction:'replace'}}
export function fromProfile(p:Profile):ConnectionInput {return {...defaults(),...p,groupId:p.groupId??'',password:'',sshPassword:'',sshKeyPassphrase:'',httpToken:'',rememberPassword:p.hasPassword,passwordAction:'keep',sshPasswordAction:'keep',sshKeyPassphraseAction:'keep',httpTokenAction:'keep'}}
export function errorText(e:unknown):string {return e instanceof Error?e.message:String(e)}
