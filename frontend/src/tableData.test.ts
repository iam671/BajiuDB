import {describe,expect,it} from 'vitest'
import {csvText,generatedSQL,importSQL,parseCSV,quoteIdentifier,suggestedExpression} from './tableData'
describe('table data transformations',()=>{
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
