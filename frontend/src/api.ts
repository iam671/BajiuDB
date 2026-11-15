export interface Profile {
  id:string; revision:number; name:string; host:string; port:number; username:string; database:string;
  environment:string; notes:string; tlsMode:string; caPath:string; serverName:string; timeout:number;
  hasPassword:boolean; createdAt:string; updatedAt:string;
}
export interface ConnectionInput extends Profile { password:string; rememberPassword:boolean; passwordAction:'keep'|'replace'|'forget' }
export type ConnectionRequest = Omit<ConnectionInput,'createdAt'|'updatedAt'|'hasPassword'>
export interface SessionInfo { id:string; connected:boolean; serverVersion:string; currentUser:string; databases:string[]; latencyMs:number; connectedAt:string }
export interface TestResult {success:boolean; message:string; serverVersion:string; latencyMs:number}
type Bridge = {
  ListConnections():Promise<Profile[]>; SaveConnection(input:ConnectionRequest):Promise<Profile>;
  DeleteConnection(id:string,revision:number):Promise<void>; TestConnection(input:ConnectionRequest):Promise<TestResult>;
  Connect(input:ConnectionRequest):Promise<SessionInfo>; Disconnect(id:string):Promise<void>;
  ListSessions():Promise<SessionInfo[]>; RefreshDatabases(id:string):Promise<SessionInfo>;
  ConfigPath():Promise<string>; SelectCAFile():Promise<string>;
}
declare global { interface Window {go?:{main:{App:Bridge}}} }
export const isDesktop = () => !!window.go?.main?.App
export const api = new Proxy({} as Bridge,{get(_target,key){return (...args:unknown[])=>{
  const bridge = window.go?.main?.App
  if(!bridge) return Promise.reject(new Error('这是界面预览。请打开 Bajiu DB 桌面程序管理真实连接。'))
  const fn = bridge[key as keyof Bridge] as (...args:unknown[])=>unknown
  return fn(...args)
}}})
export function defaults():ConnectionInput {return {id:'',revision:0,name:'',host:'127.0.0.1',port:3306,username:'root',database:'',environment:'local',notes:'',tlsMode:'disabled',caPath:'',serverName:'',timeout:10,hasPassword:false,createdAt:'',updatedAt:'',password:'',rememberPassword:true,passwordAction:'replace'}}
export function fromProfile(p:Profile):ConnectionInput {return {...p,password:'',rememberPassword:p.hasPassword,passwordAction:'keep'}}
export function errorText(e:unknown):string {return e instanceof Error?e.message:String(e)}
