import type {QueryResult} from './api'
export const quoteIdentifier=(value:string)=>'`'+value.replace(/`/g,'``')+'`'
export const sqlText=(value:string)=>"CONVERT(X'"+Array.from(new TextEncoder().encode(value),b=>b.toString(16).padStart(2,'0')).join('')+"' USING utf8mb4)"
export function parseCSV(source:string):string[][]{
 if(source.length>8*1024*1024)throw new Error('CSV 超过 8 MB')
 const text=source.replace(/^\uFEFF/,''),rows:string[][]=[];let row:string[]=[],field='',quoted=false,closed=false,started=false
 function pushField(){row.push(field);field='';closed=false;started=false;if(row.length>1024)throw new Error('CSV 每行最多 1,024 列')}
 function pushRow(){pushField();rows.push(row);row=[];if(rows.length>10001)throw new Error('CSV 最多导入 10,000 行数据')}
 for(let i=0;i<text.length;i++){
  const c=text[i]!
  if(quoted){if(c==='"'){if(text[i+1]==='"'){field+='"';i++}else{quoted=false;closed=true}}else field+=c;continue}
  if(c===','){pushField();continue}
  if(c==='\r'||c==='\n'){if(c==='\r'&&text[i+1]==='\n')i++;pushRow();continue}
  if(closed)throw new Error('CSV 引号结束后只能是分隔符或换行')
  if(c==='"'){if(started)throw new Error('CSV 字段中的引号必须转义');quoted=true;started=true}else{field+=c;started=true}
 }
 if(quoted)throw new Error('CSV 引号未闭合')
 if(started||closed||field||row.length)pushRow()
 if(rows.length<2)throw new Error('CSV 需要标题行及至少一行数据')
 const headers=rows[0]!
 if(headers.some(h=>!h)||new Set(headers).size!==headers.length)throw new Error('CSV 标题不能为空或重复')
 if(rows.some(r=>r.length!==headers.length))throw new Error('CSV 每行列数必须与标题一致')
 return rows
}
export function importSQL(table:string,rows:string[][],mapping:string[]){
 const targets=mapping.map((name,index)=>({name,index})).filter(c=>c.name)
 if(!targets.length||new Set(targets.map(c=>c.name)).size!==targets.length)throw new Error('请映射至少一个字段，同一字段不能重复映射')
 const commands=['START TRANSACTION;']
 for(let start=1;start<rows.length;start+=100){commands.push('INSERT INTO '+quoteIdentifier(table)+' ('+targets.map(c=>quoteIdentifier(c.name)).join(', ')+') VALUES\n'+rows.slice(start,start+100).map(row=>'('+targets.map(c=>row[c.index]==='\\N'?'NULL':sqlText(row[c.index]??'')).join(', ')+')').join(',\n')+';')}
 commands.push('COMMIT;');const sql=commands.join('\n');if(sql.length>8*1024*1024)throw new Error('生成的 SQL 超过 8 MB，请拆分 CSV 文件');return sql
}
export function csvText(result:QueryResult,protectFormulas=true){
 const cell=(value:string|null)=>{let text=value===null?'\\N':value;if(protectFormulas&&/^[=+@\-\t\r]/.test(text))text="'"+text;return '"'+text.replace(/"/g,'""')+'"'}
 return '\uFEFF'+[result.columns.map(c=>cell(c)).join(','),...result.rows.map(row=>row.map(cell).join(','))].join('\r\n')+'\r\n'
}
export function suggestedExpression(type:string,name:string){
 const t=type.toLowerCase()
 if(/^(tinyint|smallint|mediumint|int|bigint|decimal|numeric|float|double|real|bit|year)\b/.test(t))return '{{n}}'
 if(/^(var)?char\b|^(tiny|medium|long)?text\b/.test(t)){const limit=Number(t.match(/\((\d+)\)/)?.[1]??64);return 'LEFT(CONCAT('+sqlText(name+'_')+', {{n}}), '+Math.min(limit,128)+')'}
 if(t.startsWith('datetime')||t.startsWith('timestamp'))return 'CURRENT_TIMESTAMP'
 if(t==='date')return 'CURRENT_DATE'
 if(t.startsWith('time'))return 'CURRENT_TIME'
 if(t==='json')return 'JSON_OBJECT()'
 return ''
}
export function generatedSQL(table:string,count:number,start:number,fields:{name:string;expression:string}[]){
 if(!Number.isSafeInteger(count)||count<1||count>1000||!Number.isSafeInteger(start)||start<1||start>1000000000)throw new Error('生成行数须为 1–1,000，起始序号须为 1–1,000,000,000')
 if(!fields.length||fields.some(f=>!f.expression.trim()))throw new Error('请为选中的字段填写 SQL 表达式')
 if(fields.some(f=>/[;]|--|\/\*/.test(f.expression)))throw new Error('每个字段只能填写单个 SQL 表达式')
 const sql='START TRANSACTION;\nINSERT INTO '+quoteIdentifier(table)+' ('+fields.map(f=>quoteIdentifier(f.name)).join(', ')+') VALUES\n'+Array.from({length:count},(_,i)=>'('+fields.map(f=>f.expression.replace(/\{\{n\}\}/g,String(start+i))).join(', ')+')').join(',\n')+';\nCOMMIT;'
 if(sql.length>8*1024*1024)throw new Error('生成的 SQL 超过 8 MB');return sql
}
