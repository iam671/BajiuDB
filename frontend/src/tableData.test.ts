import {describe,expect,it} from 'vitest'
import {csvText,generatedSQL,importSQL,parseCSV,quoteIdentifier,suggestedExpression,transferSelectSQL} from './tableData'
describe('table data transformations',()=>{
 it('exports bytes and spatial values on the server with quoted identifiers and preserved NULLs',()=>{
  expect(transferSelectSQL('a`b',[['data`','varbinary(20)'],['flags','bit(64)'],['location','point'],['name','varchar(50)']])).toBe("SELECT CONCAT('0x',HEX(`data```)) AS `data```, CONCAT('0x',HEX(`flags`)) AS `flags`, CONCAT('SRID=',ST_SRID(`location`),';WKB=0x',HEX(ST_AsWKB(`location`))) AS `location`, `name` FROM `a``b` LIMIT 1001")
 })
 it('restores HEX bytes, BIT and WKB/SRID by the mapped target type, keeping empty, NULL and literal text distinct',()=>{
  const types=new Map([['bytes','blob'],['flags','bit(64)'],['place','geometry'],['label','text']])
  const sql=importSQL('t',[['a','b','c','d'],['0x00ff8041','0x0000000000000001','SRID=4326;WKB=0x01010000000000000000003e400000000000003440','0xFF'],['0x',null,null,'SRID=0;WKB=0x']],['bytes','flags','place','label'],false,types)
  expect(sql).toContain("(X'00FF8041', X'0000000000000001', ST_GeomFromWKB(X'01010000000000000000003E400000000000003440', 4326), CONVERT(")
  expect(sql).toContain("(X'', NULL, NULL, CONVERT(")
 })
 it('rejects invalid encodings before creating executable import SQL',()=>{
  for(const value of ['abc','0x0','0xGG',"0xFF'); DROP TABLE t; --"])
   expect(()=>importSQL('t',[['x'],[value]],['data'],false,new Map([['data','binary(8)']]))).toThrow('HEX')
  for(const value of ['POINT(1 2)','SRID=4294967296;WKB=0x0101000000','SRID=1;WKB=0x','SRID=-1;WKB=0x0101000000'])
   expect(()=>importSQL('t',[['x'],[value]],['data'],false,new Map([['data','point']]))).toThrow('空间值')
 })
 it('keeps typed NULL distinct from a literal backslash-N for native file imports',()=>{const sql=importSQL('qa',[['value'],[null],['\\N']],['value'],false);expect(sql).toContain('(NULL)');expect(sql).toContain("CONVERT(X'5c4e' USING utf8mb4)")})
 it('parses UTF-8 BOM, CRLF, quoted commas, escaped quotes and embedded newlines without changing large numbers',()=>{
  expect(parseCSV('\uFEFFid,name,note\r\n9007199254740993,"张三,李四","a""b\nnext"\r\n')).toEqual([['id','name','note'],['9007199254740993','张三,李四','a"b\nnext']])
 })
 it('rejects malformed or oversized CSV rather than guessing row boundaries',()=>{
  for(const text of ['a,a\n1,2','a,b\n1','a\n"unclosed','a\n"x"oops','a\nno"pe','a,b\n1,2\n3'])expect(()=>parseCSV(text)).toThrow()
  expect(()=>parseCSV('a\n'+'x\n'.repeat(10001))).toThrow('10,000')
 })
 it('quotes unusual identifiers and encodes imported values as UTF-8 literals, supports mapping, NULL and chunked transactions',()=>{
  expect(quoteIdentifier('a`b')).toBe('`a``b`')
  const sql=importSQL('a`b',[['text','skip'],["'); DROP TABLE x; --",'ignored'],['\\N','ignored'],['','ignored']],['value',''])
  expect(sql).toContain('INSERT INTO `a``b` (`value`)');expect(sql).not.toContain('DROP TABLE');expect(sql).toContain('(NULL)');expect(sql).toContain("CONVERT(X'' USING utf8mb4)");expect(sql.startsWith('START TRANSACTION;')).toBe(true);expect(sql.endsWith('COMMIT;')).toBe(true)
  expect(()=>importSQL('t',[['a','b'],['1','2']],['same','same'])).toThrow('重复')
 })
 it('exports complete quoted CSV with explicit formula protection and preserves raw mode',()=>{
  const result={columns:['name','note'],rows:[['=1+1','a,"b"\nnext'],[null,'-2']],affected:'',truncated:false}
  expect(csvText(result)).toContain('"\'=1+1"');expect(csvText(result)).toContain('"\\N"');expect(csvText(result,false)).toContain('"=1+1"');expect(parseCSV(csvText(result,false))).toEqual([['name','note'],['=1+1','a,"b"\nnext'],['\\N','-2']])
 })
 it('generates bounded expressions and requires explicit values for unsupported types',()=>{
  expect(suggestedExpression('bigint unsigned','id')).toBe('{{n}}');expect(suggestedExpression('geometry','point')).toBe('')
  const sql=generatedSQL('t',2,7,[{name:'id',expression:'{{n}}'},{name:'name',expression:"CONCAT('test_',{{n}})"}]);expect(sql).toContain("(7, CONCAT('test_',7)),\n(8, CONCAT('test_',8))")
  expect(()=>generatedSQL('t',1001,1,[{name:'id',expression:'1'}])).toThrow();expect(()=>generatedSQL('t',2,1,[{name:'id',expression:'1); DROP TABLE t;'}])).toThrow();expect(()=>generatedSQL('t',2,1,[])).toThrow()
 })
})
