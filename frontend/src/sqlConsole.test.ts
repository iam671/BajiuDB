import {expect,it} from 'vitest'
import {consoleComplete,consoleResult,consoleUse} from './sqlConsole'
it('submits only a terminator outside strings/comments and supports multiline SQL',()=>{
 for(const sql of ['SELECT 1;','SELECT 1; -- note','SELECT 1; /* note */',"SELECT 'it''s;';",'SELECT 1;\nSELECT 2;'])expect(consoleComplete(sql)).toBe(true)
 for(const sql of ['SELECT 1',"SELECT ';'", "SELECT 'unfinished;",'SELECT 1; /* unfinished','SELECT 1 -- ;','SELECT 1; SELECT 2'])expect(consoleComplete(sql)).toBe(false)
})
it('recognises only a standalone USE command without interpreting arbitrary SQL',()=>{
 expect(consoleUse('use qa_db;')).toBe('qa_db');expect(consoleUse('USE `odd``name`;')).toBe('odd`name');expect(consoleUse('\\u qa_db')).toBe('qa_db')
 expect(consoleUse('USE qa_db; SELECT 1;')).toBeUndefined();expect(consoleUse("SELECT 'USE qa_db'")).toBeUndefined();expect(consoleUse('USE ``')).toBeUndefined()
})
it('renders exact values and NULL with explicit display limits',()=>{
 const text=consoleResult({columns:['id','amount','note'],rows:[['18446744073709551615','999999999999999999.99',null]],affected:'0',truncated:false},1.236)
 expect(text).toContain('18446744073709551615');expect(text).toContain('999999999999999999.99');expect(text).toContain('NULL');expect(text).toContain('1 row(s) in set (1.24 sec)')
 expect(consoleResult({columns:['long'],rows:[['a'.repeat(100)]],affected:'0',truncated:true})).toContain('长单元格已缩略')
})
