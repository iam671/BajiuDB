import {quoteIdentifier as qi} from './tableData'
export type Field={id:string;name:string;type:string;length:string;decimals:string;notNull:boolean;unsigned:boolean;zeroFill?:boolean;autoIncrement:boolean;generated:''|'VIRTUAL'|'STORED';expression:string;defaultMode:'none'|'null'|'value'|'expression';defaultValue:string;comment:string;charset:string;collation:string;advanced:string;raw?:string;defaultRaw?:string;commentRaw?:string}
export type DesignIndex={id:string;name:string;kind:'PRIMARY'|'NORMAL'|'UNIQUE'|'FULLTEXT'|'SPATIAL';fields:string;method:string;comment:string;advanced?:string;raw?:string}
export type DesignForeignKey={id:string;name:string;fields:string;database:string;table:string;references:string;onDelete:string;onUpdate:string;raw?:string}
export type DesignTrigger={id:string;name:string;timing:'BEFORE'|'AFTER';event:'INSERT'|'UPDATE'|'DELETE';body:string;sourceBody?:string;raw?:string}
export type DesignOptions={engine:string;charset:string;collation:string;autoIncrement:string;comment:string}
export type TableDesign={sqlMode:string;fields:Field[];indexes:DesignIndex[];foreignKeys:DesignForeignKey[];triggers:DesignTrigger[];options:DesignOptions;other:string[]}
export const types=['tinyint','smallint','mediumint','int','bigint','decimal','float','double','bit','boolean','char','varchar','binary','varbinary','tinytext','text','mediumtext','longtext','tinyblob','blob','mediumblob','longblob','date','time','datetime','timestamp','year','json','enum','set','geometry','point','linestring','polygon','multipoint','multilinestring','multipolygon','geometrycollection']
export const actions=['','RESTRICT','CASCADE','SET NULL','NO ACTION']
export const uid=()=>crypto.randomUUID()
export const newField=():Field=>({id:uid(),name:'',type:'varchar',length:'255',decimals:'',notNull:false,unsigned:false,autoIncrement:false,generated:'',expression:'',defaultMode:'none',defaultValue:'',comment:'',charset:'',collation:'',advanced:''})
export const newDesign=():TableDesign=>({sqlMode:'',fields:[newField()],indexes:[],foreignKeys:[],triggers:[],options:{engine:'InnoDB',charset:'utf8mb4',collation:'',autoIncrement:'',comment:''},other:[]})
export function quoteText(value:string,mode:string){const escaped=mode.toUpperCase().split(',').includes('NO_BACKSLASH_ESCAPES')?value:value.replace(/\\/g,'\\\\').replace(/\0/g,'\\0').replace(/\x1a/g,'\\Z');return "'"+escaped.replace(/'/g,"''")+"'"}
type Token={value:string;start:number;end:number;depth:number;quoted:boolean}
function tokens(text:string,mode=''){
 const result:Token[]=[];let i=0,depth=0
 while(i<text.length){
  if(/\s/.test(text[i]!)){i++;continue}
  const start=i,c=text[i]!;let quoted=false
  if(c==='`'||c==="'"||c==='"'){
   quoted=true;i++;let closed=false
   while(i<text.length){if(text[i]==='\\'&&c!=='`'&&!(c==='"'&&mode.toUpperCase().split(',').includes('ANSI_QUOTES'))&&!mode.toUpperCase().split(',').includes('NO_BACKSLASH_ESCAPES')){i+=2;continue}if(text[i]===c){if(text[i+1]===c){i+=2;continue}i++;closed=true;break}i++}
   if(!closed)throw new Error('SQL 中的引号未闭合')
  }else if(c==='#'||c==='-'&&text[i+1]==='-'&&/\s/.test(text[i+2]??'')){const end=text.indexOf('\n',i);i=end<0?text.length:end;quoted=true}
  else if(c==='/'&&text[i+1]==='*'){const end=text.indexOf('*/',i+2);if(end<0)throw new Error('SQL 注释未闭合');i=end+2;quoted=true}
  else if(/[\w$]/.test(c)){while(i<text.length&&/[\w$]/.test(text[i]!))i++}
  else{i++;if(c===')')depth--;if(depth<0)throw new Error('SQL 括号不匹配')}
  result.push({value:text.slice(start,i),start,end:i,depth,quoted})
  if(c==='(')depth++
 }
 if(depth!==0)throw new Error('SQL 括号不匹配')
 return result
}
function identifier(value:string){return value.startsWith('`')?value.slice(1,-1).replace(/``/g,'`'):value}
function unquote(value:string,mode=''){if(!value.startsWith("'")&&!value.startsWith('"'))return value;const quote=value[0]!;const text=value.slice(1,-1).split(quote+quote).join(quote);if(mode.toUpperCase().split(',').includes('NO_BACKSLASH_ESCAPES'))return text;return text.replace(/\\([0bnrtZ\\'"])/g,(_,c:string)=>({'0':'\0',b:'\b',n:'\n',r:'\r',t:'\t',Z:'\x1a'}[c]??c))}
export function convertSQLStrings(sql:string,from:string,to:string){let out=sql;const ansi=from.toUpperCase().split(',').includes('ANSI_QUOTES');for(const token of tokens(sql,from).reverse()){if(token.value.startsWith("'")||token.value.startsWith('"')&&!ansi)out=out.slice(0,token.start)+quoteText(unquote(token.value,from),to)+out.slice(token.end)}return out}
function split(text:string,mode=''){const ts=tokens(text,mode),parts:string[]=[];let start=0;for(const t of ts)if(t.value===','&&t.depth===0){parts.push(text.slice(start,t.start).trim());start=t.end}parts.push(text.slice(start).trim());return parts.filter(Boolean)}
export function fragment(value:string,mode=''){const ts=tokens(value,mode);if(ts.some((t,i)=>!t.quoted&&(t.value===';'||t.value==='#'||t.value==='-'&&ts[i+1]?.value==='-'&&ts[i+1]?.start===t.end)||t.value.startsWith('/*'))||/(?:^|\n)\s*DELIMITER\b/i.test(value))throw new Error('表达式不能包含语句分隔符或注释');return value.trim()}
function validName(value:string,label:string){if(!value||Array.from(value).length>64||/[\0\r\n]/.test(value))throw new Error(label+'须为 1–64 个字符');return qi(value)}
function keyword(value:string,label:string){if(!/^[a-z][a-z0-9_]*$/i.test(value))throw new Error(label+'格式无效');return value}
export function columnNames(text:string){return split(text).map(part=>identifier(part.trim()))}
function fieldList(text:string){const names=columnNames(text);if(!names.length)throw new Error('请选择至少一个字段');return names.map(name=>validName(name,'字段名')).join(', ')}
function indexFields(text:string,mode:string){if(!text.trim())throw new Error('索引需要至少一个字段');return split(text,mode).map(part=>{
 const simple=part.match(/^(`(?:``|[^`])+`|[\w$\u0080-\uffff]+)(?:\s*\(\s*(\d+)\s*\))?(?:\s+(ASC|DESC))?$/i)
 return simple?validName(identifier(simple[1]!),'索引字段')+(simple[2]?'('+simple[2]+')':'')+(simple[3]?' '+simple[3].toUpperCase():''):fragment(part,mode)
 }).join(', ')}
const attrWords=new Set(['NOT','NULL','DEFAULT','AUTO_INCREMENT','COMMENT','CHARACTER','COLLATE','GENERATED','AS','ON','UNSIGNED','ZEROFILL','VIRTUAL','STORED','VISIBLE','INVISIBLE','COLUMN_FORMAT','STORAGE','SRID','ENGINE_ATTRIBUTE','SECONDARY_ENGINE_ATTRIBUTE'])
function parseField(raw:string,mode:string):Field{
 const ts=tokens(raw,mode),field=newField();field.raw=raw;field.name=identifier(ts[0]!.value);field.type=ts[1]!.value.toLowerCase();let i=2
 if(ts[i]?.value==='('){const close=ts.findIndex((t,n)=>n>i&&t.value===')'&&t.depth===0);if(close<0)throw new Error('字段类型括号不完整');const args=raw.slice(ts[i]!.end,ts[close]!.start);const lengths=split(args,mode);field.length=lengths[0]??'';if(['enum','set'].includes(field.type))field.length=args;else field.decimals=lengths[1]??'';i=close+1}else field.length=''
 const advanced:string[]=[]
 while(i<ts.length){
  const token=ts[i]!,word=token.value.toUpperCase(),start=i;i++
  const nextValue=()=>{const from=i;while(i<ts.length&&(i===from||ts[i]!.depth>0||ts[i]!.quoted||!attrWords.has(ts[i]!.value.toUpperCase())))i++;return raw.slice(ts[from]?.start??token.end,ts[i-1]?.end??token.end).trim()}
  if(word==='NOT'&&ts[i]?.value.toUpperCase()==='NULL'){field.notNull=true;i++}
  else if(word==='NULL')field.notNull=false
  else if(word==='UNSIGNED')field.unsigned=true
  else if(word==='ZEROFILL')field.zeroFill=true
  else if(word==='AUTO_INCREMENT')field.autoIncrement=true
  else if(word==='DEFAULT'){const value=nextValue();field.defaultRaw='DEFAULT '+value;field.defaultMode=/^NULL$/i.test(value)?'null':value.startsWith("'")?'value':'expression';field.defaultValue=field.defaultMode==='value'?unquote(value,mode):field.defaultMode==='null'?'':value}
  else if(word==='COMMENT'){const value=nextValue();field.comment=unquote(value,mode);field.commentRaw='COMMENT '+value}
  else if(word==='CHARACTER'&&ts[i]?.value.toUpperCase()==='SET'){i++;field.charset=identifier(ts[i++]?.value??'')}
  else if(word==='COLLATE')field.collation=identifier(ts[i++]?.value??'')
  else if(word==='ON'&&ts[i]?.value.toUpperCase()==='UPDATE'){i++;advanced.push('ON UPDATE '+nextValue())}
  else if(word==='GENERATED'||word==='AS'){
   if(word==='GENERATED'){if(ts[i]?.value.toUpperCase()==='ALWAYS')i++;if(ts[i]?.value.toUpperCase()==='AS')i++}
   const open=ts[i],close=ts.findIndex((t,n)=>n>i&&t.value===')'&&t.depth===0)
   if(open?.value!=='('||close<0)throw new Error('生成列表达式不完整')
   field.expression=raw.slice(open.end,ts[close]!.start);i=close+1;field.generated=ts[i]?.value.toUpperCase()==='STORED'?'STORED':'VIRTUAL';if(['VIRTUAL','STORED'].includes(ts[i]?.value.toUpperCase()??''))i++
  }else{while(i<ts.length&&(ts[i]!.depth>0||ts[i]!.quoted||!attrWords.has(ts[i]!.value.toUpperCase())))i++;advanced.push(raw.slice(ts[start]!.start,ts[i-1]!.end))}
 }
 field.advanced=advanced.join(' ');return field
}
function groupAt(raw:string,ts:Token[],index:number){const open=ts[index],close=ts.findIndex((t,i)=>i>index&&t.value===')'&&t.depth===0);if(open?.value!=='('||close<0)throw new Error('约束字段列表不完整');return {value:raw.slice(open.end,ts[close]!.start),next:close+1}}
export function parseDesign(ddl:string,mode=''):TableDesign{
 // SHOW CREATE TABLE canonicalizes string escaping independently of session mode.
 ddl=convertSQLStrings(ddl,'',mode)
 const ts=tokens(ddl,mode),open=ts.findIndex(t=>t.value==='('&&t.depth===0),close=ts.findIndex((t,i)=>i>open&&t.value===')'&&t.depth===0)
 if(open<0||close<0)throw new Error('无法解析表定义，请使用查询编辑器查看原始 SQL')
 const model=newDesign();model.sqlMode=mode;model.fields=[]
 for(const raw of split(ddl.slice(ts[open]!.end,ts[close]!.start),mode)){
  const part=tokens(raw,mode),word=part[0]?.value.toUpperCase()??''
  if(part[0]?.value.startsWith('`')||(!['PRIMARY','UNIQUE','KEY','INDEX','CONSTRAINT','FOREIGN','CHECK','FULLTEXT','SPATIAL'].includes(word))){model.fields.push(parseField(raw,mode));continue}
  if(['PRIMARY','UNIQUE','KEY','INDEX','FULLTEXT','SPATIAL'].includes(word)){
   const start=part.findIndex(t=>t.value==='('&&t.depth===0),group=groupAt(raw,part,start),kind=word==='PRIMARY'?'PRIMARY':word==='KEY'||word==='INDEX'?'NORMAL':word as DesignIndex['kind'];let name='PRIMARY'
   if(kind!=='PRIMARY'){const nameToken=part.slice(1,start).find(t=>!['KEY','INDEX','USING','BTREE','HASH'].includes(t.value.toUpperCase()));name=identifier(nameToken?.value??'')}
   let suffix=raw.slice(part[group.next]?.start??raw.length);const suffixTokens=tokens(suffix,mode),commentAt=suffixTokens.findIndex(t=>!t.quoted&&t.value.toUpperCase()==='COMMENT'),commentToken=suffixTokens[commentAt+1],comment=commentAt>=0?unquote(commentToken?.value??'',mode):''
   if(commentAt>=0&&commentToken)suffix=suffix.slice(0,suffixTokens[commentAt]!.start)+suffix.slice(commentToken.end)
   model.indexes.push({id:uid(),name,kind,fields:group.value,method:raw.match(/\bUSING\s+(BTREE|HASH)\b/i)?.[1]?.toUpperCase()??'',comment,advanced:suffix.replace(/\bUSING\s+(BTREE|HASH)\b/i,'').trim(),raw});continue
  }
  if(/\bFOREIGN\s+KEY\b/i.test(raw)){
   const start=part.findIndex(t=>t.value==='('&&t.depth===0),local=groupAt(raw,part,start),ref=part.findIndex((t,i)=>i>=local.next&&t.value.toUpperCase()==='REFERENCES'),remoteStart=part.findIndex((t,i)=>i>ref&&t.value==='('&&t.depth===0),remote=groupAt(raw,part,remoteStart),target=part.slice(ref+1,remoteStart).filter(t=>t.value!=='.').map(t=>identifier(t.value))
   model.foreignKeys.push({id:uid(),name:word==='CONSTRAINT'?identifier(part[1]!.value):'',fields:local.value,database:target.length>1?target[0]!:'',table:target.at(-1)??'',references:remote.value,onDelete:raw.match(/\bON DELETE\s+(RESTRICT|CASCADE|SET NULL|NO ACTION)/i)?.[1]?.toUpperCase()??'',onUpdate:raw.match(/\bON UPDATE\s+(RESTRICT|CASCADE|SET NULL|NO ACTION)/i)?.[1]?.toUpperCase()??'',raw});continue
  }
  model.other.push(raw)
 }
 const tail=ddl.slice(ts[close]!.end),tailTokens=tokens(tail,mode),commentIndex=tailTokens.findIndex(t=>!t.quoted&&t.value.toUpperCase()==='COMMENT'),commentValue=tailTokens[commentIndex+1]?.value==='='?tailTokens[commentIndex+2]?.value:tailTokens[commentIndex+1]?.value
 model.options={engine:tail.match(/\bENGINE\s*=\s*(\w+)/i)?.[1]??'',charset:tail.match(/\b(?:DEFAULT\s+)?(?:CHARSET|CHARACTER\s+SET)\s*=\s*(\w+)/i)?.[1]??'',collation:tail.match(/\bCOLLATE\s*=\s*(\w+)/i)?.[1]??'',autoIncrement:tail.match(/\bAUTO_INCREMENT\s*=\s*(\d+)/i)?.[1]??'',comment:commentIndex>=0?unquote(commentValue??'',mode):''}
 return model
}
export function renameReferences(value:string,before:string,after:string,mode=''){let parts:Token[];try{parts=tokens(value,mode)}catch{return value}const replacements=parts.filter(t=>t.value.startsWith('`')&&identifier(t.value)===before||!t.quoted&&t.value===before&&t.depth===0);let out=value;for(const t of replacements.reverse())out=out.slice(0,t.start)+qi(after)+out.slice(t.end);return out}
export function same(a:unknown,b:unknown){return JSON.stringify(a)===JSON.stringify(b)}
function defaultExpression(value:string,mode:string){const text=fragment(value,mode);if(!text)throw new Error('默认值表达式不能为空');return /^(?:CURRENT_TIMESTAMP(?:\(\d*\))?|[-+]?\d+(?:\.\d+)?|NULL)$/i.test(text)?text:'('+text+')'}
function fieldSQL(field:Field,mode:string,original?:Field){
 if(original&&same(field,original)&&field.raw)return field.raw
 const type=field.type.toLowerCase().trim();if(!types.includes(type))throw new Error('暂不支持修改字段类型 '+field.type+'；可保留原定义或使用查询编辑器')
 let args=''
 if(field.length.trim()){
  if(['enum','set'].includes(type)){const values=split(field.length,mode);if(values.some(v=>!/^'(?:''|\\.|[^'])*'$/.test(v)))throw new Error('ENUM/SET 长度须填写带单引号的值列表');args='('+values.map(v=>quoteText(unquote(v,mode),mode)).join(', ')+')'}
  else{if(!/^\d+$/.test(field.length)||field.decimals&&!/^\d+$/.test(field.decimals))throw new Error('字段长度和小数点须为整数');args='('+field.length+(field.decimals?','+field.decimals:'')+')'}
 }else if(field.decimals)throw new Error('请先填写字段长度')
 let sql=validName(field.name,'字段名')+' '+type.toUpperCase()+args+(field.unsigned?' UNSIGNED':'')+(field.zeroFill?' ZEROFILL':'')
 if(field.charset)sql+=' CHARACTER SET '+keyword(field.charset,'字符集')
 if(field.collation)sql+=' COLLATE '+keyword(field.collation,'排序规则')
 if(field.generated){if(!field.expression.trim())throw new Error('请填写生成列表达式');sql+=' GENERATED ALWAYS AS ('+fragment(field.expression,mode)+') '+field.generated}
 sql+=field.notNull?' NOT NULL':' NULL'
 if(!field.generated&&field.defaultMode!=='none'){
  if(original&&field.defaultMode===original.defaultMode&&field.defaultValue===original.defaultValue&&original.defaultRaw)sql+=' '+original.defaultRaw
  else sql+=' DEFAULT '+(field.defaultMode==='null'?'NULL':field.defaultMode==='value'?quoteText(field.defaultValue,mode):defaultExpression(field.defaultValue,mode))
 }
 if(field.autoIncrement){if(field.generated)throw new Error('生成列不能自增');sql+=' AUTO_INCREMENT'}
 if(field.comment)sql+=' '+(original&&field.comment===original.comment&&original.commentRaw?original.commentRaw:'COMMENT '+quoteText(field.comment,mode))
 if(field.advanced.trim())sql+=' '+fragment(field.advanced,mode)
 return sql
}
function indexSQL(index:DesignIndex,mode:string){return (index.kind==='PRIMARY'?'PRIMARY KEY':(index.kind==='NORMAL'?'':index.kind+' ')+'KEY '+validName(index.name,'索引名'))+' ('+indexFields(index.fields,mode)+')'+(index.method?' USING '+(['BTREE','HASH'].includes(index.method)?index.method:keyword(index.method,'索引方法')):'')+(index.comment?' COMMENT '+quoteText(index.comment,mode):'')+(index.advanced?' '+fragment(index.advanced,mode):'')}
function foreignSQL(fk:DesignForeignKey){if(columnNames(fk.fields).length!==columnNames(fk.references).length)throw new Error('外键字段与引用字段数量必须相同');if(!actions.includes(fk.onDelete)||!actions.includes(fk.onUpdate))throw new Error('外键动作无效');return 'CONSTRAINT '+validName(fk.name,'外键名')+' FOREIGN KEY ('+fieldList(fk.fields)+') REFERENCES '+(fk.database?validName(fk.database,'引用数据库')+'.':'')+validName(fk.table,'引用表')+' ('+fieldList(fk.references)+')'+(fk.onDelete?' ON DELETE '+fk.onDelete:'')+(fk.onUpdate?' ON UPDATE '+fk.onUpdate:'')}
function optionSQL(options:DesignOptions,mode:string,original?:DesignOptions){const out:string[]=[];for(const key of ['engine','charset','collation','autoIncrement','comment'] as const){if(original&&options[key]===original[key])continue;const value=options[key];if(key==='comment')out.push('COMMENT='+quoteText(value,mode));else if(value){if(key==='autoIncrement'){if(!/^\d+$/.test(value))throw new Error('自增起始值须为整数');out.push('AUTO_INCREMENT='+value)}else out.push(({engine:'ENGINE',charset:'DEFAULT CHARACTER SET',collation:'COLLATE'}[key])+'='+keyword(value,key))}}return out}
export function designSQL(database:string,table:string,model:TableDesign,original?:TableDesign){
 const target=validName(database,'数据库名')+'.'+validName(table,'表名'),names=model.fields.map(f=>f.name)
 if(!names.length)throw new Error('至少需要一个字段');if(new Set(names.map(n=>n.toLowerCase())).size!==names.length)throw new Error('字段名不能重复')
 if(model.fields.filter(f=>f.autoIncrement).length>1)throw new Error('一张表只能有一个自增字段')
 const primary=model.indexes.filter(i=>i.kind==='PRIMARY');if(primary.length>1)throw new Error('一张表只能有一个主键')
 if(primary.length&&columnNames(primary[0]!.fields).some(name=>!model.fields.some(f=>f.name===name&&f.notNull)))throw new Error('主键字段须存在并设置为不是 NULL')
 const compile=(field:Field)=>fieldSQL(field,model.sqlMode,original?.fields.find(f=>f.id===field.id))
 if(!original)return 'CREATE TABLE '+target+' (\n  '+[...model.fields.map(compile),...model.indexes.map(index=>indexSQL(index,model.sqlMode)),...model.foreignKeys.map(foreignSQL),...model.other].join(',\n  ')+'\n) '+optionSQL(model.options,model.sqlMode).join(' ')+';'
 const parts:string[]=[],dropForeign:string[]=[]
 for(const old of original.foreignKeys)if(!model.foreignKeys.some(v=>v.id===old.id&&same(v,old)))dropForeign.push('DROP FOREIGN KEY '+validName(old.name,'外键名'))
 for(const old of original.indexes)if(!model.indexes.some(v=>v.id===old.id&&same(v,old)))parts.push(old.kind==='PRIMARY'?'DROP PRIMARY KEY':'DROP INDEX '+validName(old.name,'索引名'))
 for(const old of original.fields)if(!model.fields.some(f=>f.id===old.id))parts.push('DROP COLUMN '+validName(old.name,'字段名'))
 model.fields.forEach((field,index)=>{const old=original.fields.find(f=>f.id===field.id),previous=model.fields[index-1],oldPosition=original.fields.findIndex(f=>f.id===field.id),moved=oldPosition>=0&&(original.fields[oldPosition-1]?.id!==previous?.id);const position=index?' AFTER '+validName(previous!.name,'字段名'):' FIRST';if(!old)parts.push('ADD COLUMN '+compile(field)+position);else if(!same(field,old)||moved)parts.push((field.name===old.name?'MODIFY COLUMN ':'CHANGE COLUMN '+validName(old.name,'字段名')+' ')+compile(field)+position)})
 for(const index of model.indexes)if(!original.indexes.some(v=>v.id===index.id&&same(v,index)))parts.push('ADD '+indexSQL(index,model.sqlMode))
 for(const fk of model.foreignKeys)if(!original.foreignKeys.some(v=>v.id===fk.id&&same(v,fk)))parts.push('ADD '+foreignSQL(fk))
 parts.push(...optionSQL(model.options,model.sqlMode,original.options))
 return [dropForeign.length?'ALTER TABLE '+target+'\n  '+dropForeign.join(',\n  ')+';':'',parts.length?'ALTER TABLE '+target+'\n  '+parts.join(',\n  ')+';':''].filter(Boolean).join('\n\n')
}
export function triggerStatement(database:string,table:string,trigger:DesignTrigger){validName(trigger.name,'触发器名');if(!['BEFORE','AFTER'].includes(trigger.timing)||!['INSERT','UPDATE','DELETE'].includes(trigger.event)||!trigger.body.trim()||/\0|(?:^|\n)\s*DELIMITER\b/i.test(trigger.body))throw new Error('请填写有效的触发器事件和 SQL 主体');const prefix=trigger.raw?.match(/^CREATE\s+(DEFINER\s*=\s*.+?)\s+TRIGGER\b/i)?.[1];return 'CREATE '+(prefix?prefix+' ':'')+'TRIGGER '+qi(database)+'.'+qi(trigger.name)+' '+trigger.timing+' '+trigger.event+' ON '+qi(database)+'.'+qi(table)+' FOR EACH ROW\n'+trigger.body.trim()}
export function triggerScript(database:string,table:string,model:TableDesign,original?:TableDesign){const out:string[]=[];for(const old of original?.triggers??[])if(!model.triggers.some(t=>t.id===old.id&&same(t,old)))out.push('DROP TRIGGER '+qi(database)+'.'+qi(old.name)+';');for(const trigger of model.triggers)if(!original?.triggers.some(t=>t.id===trigger.id&&same(t,trigger))){const sql=triggerStatement(database,table,trigger);let number=0,delimiter='$bajiu0$';while(sql.includes(delimiter)){if(++number>10000)throw new Error('无法为触发器选择安全分隔符');delimiter='$bajiu'+number+'$'};out.push('DELIMITER '+delimiter+'\n'+sql+'\n'+delimiter+'\nDELIMITER ;')}return out.join('\n\n')}
