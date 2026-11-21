import type {Profile} from './api'
export const databaseEngines=[
 {value:'mysql',label:'MySQL',port:3306,user:'root',database:''},
 {value:'mariadb',label:'MariaDB',port:3306,user:'root',database:''},
 {value:'postgresql',label:'PostgreSQL',port:5432,user:'postgres',database:'postgres'},
 {value:'sqlserver',label:'SQL Server',port:1433,user:'sa',database:'master'},
 {value:'oracle',label:'Oracle',port:1521,user:'',database:''},
 {value:'sqlite',label:'SQLite',port:0,user:'',database:'main'},
 {value:'redis',label:'Redis',port:6379,user:'',database:'0'},
 {value:'mongodb',label:'MongoDB',port:27017,user:'',database:''},
]
export const engineOf=(profile?:Pick<Profile,'engine'>)=>profile?.engine||'mysql'
export const engineLabel=(profile?:Pick<Profile,'engine'>)=>databaseEngines.find(e=>e.value===engineOf(profile))?.label??'数据库'
export const mysqlFamily=(profile?:Pick<Profile,'engine'>)=>['mysql','mariadb'].includes(engineOf(profile))
export const objectLabel=(profile?:Pick<Profile,'engine'>)=>engineOf(profile)==='redis'?'键':engineOf(profile)==='mongodb'?'集合':'表'
export const defaultQuery=(profile:Profile)=>engineOf(profile)==='redis'?'["PING"]':engineOf(profile)==='mongodb'?'{"ping":1}':''
