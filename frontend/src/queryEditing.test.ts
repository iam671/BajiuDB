import {expect,it} from 'vitest'
import {editableQuery,queryPageScope,queryProjection} from './queryEditing'
import {pageSQL,saveRowSQL,makeDraft,type TableColumn} from './tableEditing'
const column=(name:string,type='varchar(50)'):TableColumn=>({name,type,nullable:false,key:'',defaultValue:null,extra:'',comment:'',generated:false,binary:false,spatial:false})
const columns=[column('id','bigint unsigned'),column('name'),column('amount','decimal(20,2)')]
it('maps aliases to source fields and reads hidden keys/full snapshots with the original predicates, order and limit',()=>{
 const plan=editableQuery("SELECT t.name AS display_name,t.amount FROM qa.items AS t WHERE t.amount >= 5 AND t.name <> 'a''b' ORDER BY display_name DESC LIMIT 20 OFFSET 7;",'default')!
 expect(queryProjection(plan,columns)).toEqual([{index:1,label:'display_name'},{index:2,label:'amount'}])
 const sql=pageSQL(plan.database,plan.table,columns,['id'],1,{column:'name',operator:'contains',value:'draft'},'',false,plan)
 expect(sql).toContain('CONVERT(`id` USING utf8mb4) AS `id`');expect(sql).toContain('FROM `qa`.`items` AS `t`');expect(sql).toContain("WHERE (t . amount >= 5 AND t . name <> 'a''b') AND (");expect(sql).toContain('ORDER BY `name` DESC LIMIT 20 OFFSET 7;')
 const original=['18446744073709551615','old','999999999999999999.99'],draft=makeDraft(columns,original);draft[1]={mode:'value',value:'new'}
 const update=saveRowSQL('qa','items',columns,['id'],original,draft)
 expect(update).toMatch(/^UPDATE `qa`.`items` SET `name`=/);expect(update).toContain('`id` <=>');expect(update).toContain('CAST(CONVERT(`amount` USING utf8mb4) AS BINARY)');expect(update).toMatch(/LIMIT 1;$/)
})
it.each(['SELECT DISTINCT name FROM items','SELECT count(*) FROM items','SELECT name FROM items JOIN other ON items.id=other.id','SELECT name FROM items UNION SELECT name FROM other','SELECT name FROM items WHERE id IN (SELECT id FROM other)','SELECT name FROM items WHERE sleep(1)=0','SELECT name FROM items GROUP BY name','SELECT name FROM items; DELETE FROM items','SELECT name FROM items -- comment','SELECT name FROM items WHERE name="ambiguous"',"SELECT name FROM items WHERE name='a\\b'",'SELECT name FROM items FOR UPDATE','WITH t AS (SELECT * FROM items) SELECT * FROM t','SELECT name,name FROM items','SELECT name AS n,amount AS n FROM items'])('keeps ambiguous or derived query results read-only: %s',sql=>{expect(editableQuery(sql,'qa')).toBeUndefined()})
it('preserves integer LIMIT precision, bounds paging and rejects missing source mappings',()=>{
 const plan=editableQuery('SELECT * FROM items LIMIT 10,1002','qa')!;expect(queryPageScope(plan,2)).toMatchObject({limit:'2',offset:'1010'})
 expect(()=>queryPageScope(plan,3)).toThrow();expect(()=>queryProjection({...plan,projection:[{name:'missing',label:'x'}]},columns)).toThrow()
 const huge=editableQuery('SELECT name FROM items LIMIT 18446744073709551615 OFFSET 18446744073709551615','qa')!;expect(queryPageScope(huge,1).offset).toBe('18446744073709551615');expect(()=>queryPageScope(huge,2)).toThrow()
 expect(editableQuery('SELECT name FROM items LIMIT 18446744073709551616','qa')).toBeUndefined()
})
it('does not mistake bare server functions or constants for physical fields',()=>{
 for(const word of ['CURRENT_DATE','CURRENT_TIMESTAMP','CURRENT_USER','NULL','TRUE','UTC_TIME'])expect(editableQuery('SELECT '+word+' FROM items','qa')).toBeUndefined()
 expect(editableQuery('SELECT `current_date` FROM items','qa')?.projection).toEqual([{name:'current_date',label:'current_date'}])
})
