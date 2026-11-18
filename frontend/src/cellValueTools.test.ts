import {expect,it} from 'vitest'
import {cellValueError,enumValues,formatJSON,quickKind,stepNumber,validDate,validTime} from './cellValueTools'
import type {TableColumn} from './tableEditing'
const col=(type:string,binary=false):TableColumn=>({name:'value',type,binary,nullable:true,key:'',defaultValue:null,extra:'',comment:'',generated:false,spatial:false})
it('recognises date/time and other useful quick editor types',()=>{
 expect(['date','datetime(6)','timestamp','time(3)','year','enum(\'a\')','set(\'a\')','tinyint(1)','bit(1)','bigint','decimal(30,5)','json','varchar(50)'].map(t=>quickKind(col(t,t.startsWith('bit'))))).toEqual(['date','datetime','datetime','time','year','enum','set','boolean','boolean','number','number','json','text'])
 expect(validDate('2024-02-29')).toBe(true);expect(validDate('2025-02-29')).toBe(false);expect(validDate('2026-04-31')).toBe(false)
 expect(validTime('-838:59:59.123456','time(6)')).toBe(true);expect(validTime('-839:00:00','time')).toBe(false);expect(validTime('25:00:00','datetime',true)).toBe(false);expect(validTime('12:34:56.123','time(2)')).toBe(false)
})
it('retains exact big integers, decimals and JSON number tokens',()=>{
 expect(stepNumber('18446744073709551615',1)).toBe('18446744073709551616');expect(stepNumber('999999999999999999.99',-1)).toBe('999999999999999998.99');expect(stepNumber('-0.50',1)).toBe('0.50')
 const json='{"id":18446744073709551615,"decimal":12345678901234567890.123456,"text":" a \\" b"}'
 expect(formatJSON(json)).toContain('18446744073709551615');expect(formatJSON(json)).toContain('12345678901234567890.123456');expect(JSON.parse(formatJSON(json)).text).toBe(' a " b')
 expect(()=>formatJSON('['.repeat(65)+'0'+']'.repeat(65))).toThrow()
})
it('decodes enum values and validates staged values without conflating NULL and empty strings',()=>{
 expect(enumValues("enum('a','it''s','line\\nnext','x\\\\y')")).toEqual(['a',"it's",'line\nnext','x\\y'])
 const payload={column:col('enum(\'a\',\'\')'),isNew:false,cell:{mode:'value' as const,value:''}};expect(cellValueError(payload)).toBe('');expect(cellValueError({...payload,cell:{mode:'value',value:'missing'}})).not.toBe('')
 expect(cellValueError({...payload,column:col('bit(1)',true),cell:{mode:'value',value:'01'}})).toBe('');expect(cellValueError({...payload,column:col('bit(1)',true),cell:{mode:'value',value:'1'}})).not.toBe('')
 expect(cellValueError({...payload,column:col('tinyint(1)'),cell:{mode:'value',value:'2'}})).toBe('')
 expect(cellValueError({...payload,column:{...col('date'),nullable:false},cell:{mode:'null',value:''}})).not.toBe('')
})
