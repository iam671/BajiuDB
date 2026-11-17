import {describe,it,expect} from 'vitest'
import {parseDesign,designSQL,newDesign,newField,quoteText,convertSQLStrings,renameReferences,triggerScript,uid,fieldDefaultInput,setFieldDefaultInput} from './tableDesigner'
const fixture="CREATE TABLE `items` (\n `id` bigint unsigned NOT NULL AUTO_INCREMENT,\n `title` varchar(80) NOT NULL DEFAULT 'O''Brien' COMMENT 'name',\n `calc` int GENERATED ALWAYS AS ((length(`title`) + 1)) VIRTUAL,\n PRIMARY KEY (`id`),\n KEY `idx_title` (`title`(20)) COMMENT 'prefix',\n CONSTRAINT `check_title` CHECK ((length(`title`) > 0))\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='table'"
const clone=<T,>(v:T):T=>JSON.parse(JSON.stringify(v))
describe('table designer SQL',()=>{
 it('reads and removes timestamp auto-update without losing precision or other attributes',()=>{
  const original=parseDesign("CREATE TABLE `items` (`changed` timestamp(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT 'time')"),draft=clone(original),field=draft.fields[0]!
  expect(field.onUpdate.toUpperCase()).toBe('CURRENT_TIMESTAMP(3)');expect(field.advanced).toBe('');expect(designSQL('qa','items',draft,original)).toBe('')
  field.comment='updated';expect(designSQL('qa','items',draft,original)).toContain('ON UPDATE CURRENT_TIMESTAMP(3)')
  field.length='6';const sql=designSQL('qa','items',draft,original);expect(sql).toContain('TIMESTAMP(6) NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)')
  field.onUpdate='';expect(designSQL('qa','items',draft,original)).not.toContain('ON UPDATE')
 })
 it('generates numeric zero fill and rejects timestamp updating for other or generated types',()=>{
  const draft=newDesign(),field=draft.fields[0]!;Object.assign(field,{name:'id',type:'int',length:'8',unsigned:true,zeroFill:true});expect(designSQL('qa','items',draft)).toContain('INT(8) UNSIGNED ZEROFILL')
  field.onUpdate='CURRENT_TIMESTAMP';expect(()=>designSQL('qa','items',draft)).toThrow('TIMESTAMP/DATETIME')
  Object.assign(field,{type:'datetime',length:'7'});expect(()=>designSQL('qa','items',draft)).toThrow('时间精度')
  Object.assign(field,{length:'3',generated:'VIRTUAL',expression:'NOW()'});expect(()=>designSQL('qa','items',draft)).toThrow('TIMESTAMP/DATETIME')
 })
 it('distinguishes cleared defaults, empty strings, NULL and exact typed values',()=>{
  const draft=newDesign(),field=draft.fields[0]!;field.name='value'
  for(const [input,expected] of [['EMPTY STRING',"DEFAULT ''"],['NULL','DEFAULT NULL'],['9007199254740993.125',"DEFAULT '9007199254740993.125'"],["O'Brien\\path", "DEFAULT 'O''Brien\\\\path'"],["'NULL'","DEFAULT 'NULL'"]]){
   setFieldDefaultInput(field,input!,'');expect(designSQL('qa','defaults',draft)).toContain(expected!);expect(fieldDefaultInput(field,'')).toBe(input)
  }
  setFieldDefaultInput(field,'','');expect(field.defaultMode).toBe('none');expect(designSQL('qa','defaults',draft).split('\n')[1]).not.toContain(' DEFAULT ')
 })
 it('preserves existing defaults on display and supports explicit expressions and mode-aware literal input',()=>{
  const original=parseDesign("CREATE TABLE `items` (`created` timestamp DEFAULT CURRENT_TIMESTAMP(3), `literal` varchar(80) DEFAULT 'NULL')"),draft=clone(original)
  for(const field of draft.fields)setFieldDefaultInput(field,fieldDefaultInput(field,draft.sqlMode),draft.sqlMode)
  expect(designSQL('qa','items',draft,original)).toBe('')
  const field=draft.fields[0]!
  setFieldDefaultInput(field,'(1); DROP TABLE x; (2)','');expect(()=>designSQL('qa','items',draft,original)).toThrow()
  setFieldDefaultInput(field,'CURRENT_TIMESTAMP','');expect(designSQL('qa','items',draft,original)).toContain('DEFAULT CURRENT_TIMESTAMP')
  setFieldDefaultInput(field,"'C:\\path'",'NO_BACKSLASH_ESCAPES');expect(field.defaultValue).toBe('C:\\path')
 })
 it('preserves nested generated expressions, literal commas and checks without inventing changes',()=>{const original=parseDesign(fixture);expect(original.fields[2]?.expression).toBe('(length(`title`) + 1)');expect(original.other).toHaveLength(1);expect(designSQL('qa','items',clone(original),original)).toBe('')})
 it('alters only edited columns and leaves independent constraints and generated columns alone',()=>{const original=parseDesign(fixture),draft=clone(original);draft.fields[1]!.length='160';const sql=designSQL('qa','items',draft,original);expect(sql).toContain('MODIFY COLUMN `title` VARCHAR(160)');expect(sql).toContain("DEFAULT 'O''Brien'");expect(sql).not.toContain('DROP');expect(sql).not.toContain('`calc`')})
 it('uses original names for rename and SQL identifiers for references',()=>{const original=parseDesign(fixture),draft=clone(original);draft.fields[1]!.name='display`title';const sql=designSQL('qa','items',draft,original);expect(sql).toContain('CHANGE COLUMN `title` `display``title`');expect(renameReferences('`title`(20) DESC, `id`','title','display`title')).toBe('`display``title`(20) DESC, `id`')})
 it('does not rewrite string literals while renaming expression references',()=>{expect(renameReferences("(concat(`title`, 'title')), `id`",'title','caption')).toBe("(concat(`caption`, 'title')), `id`")})
 it('keeps the full original column definition when only column order changes',()=>{const original=parseDesign(fixture),draft=clone(original),column=draft.fields.splice(1,1)[0]!;draft.fields.push(column);const sql=designSQL('qa','items',draft,original);expect(sql).toContain('MODIFY COLUMN '+original.fields[1]!.raw+' AFTER `calc`')})
 it('splits dropping and rebuilding same-name foreign keys into ordered ALTER statements',()=>{const original=parseDesign('CREATE TABLE `c` (`id` int NOT NULL, `parent` int, PRIMARY KEY (`id`), CONSTRAINT `fk_c` FOREIGN KEY (`parent`) REFERENCES `p` (`id`) ON DELETE RESTRICT) ENGINE=InnoDB'),draft=clone(original);draft.foreignKeys[0]!.onDelete='CASCADE';const sql=designSQL('qa','c',draft,original);expect(sql).toMatch(/DROP FOREIGN KEY `fk_c`;\n\nALTER TABLE/);expect(sql).toContain('ON DELETE CASCADE')})
 it('retains canonical SHOW CREATE string meanings in NO_BACKSLASH_ESCAPES mode',()=>{const ddl="CREATE TABLE `a` (`path` varchar(80) DEFAULT 'C:\\\\files' COMMENT 'end\\\\') COMMENT='C:\\\\folder'",model=parseDesign(ddl,'NO_BACKSLASH_ESCAPES');expect(model.fields[0]!.defaultValue).toBe('C:\\files');expect(model.fields[0]!.comment).toBe('end\\');expect(model.options.comment).toBe('C:\\folder');expect(quoteText("a'\\b",model.sqlMode)).toBe("'a''\\b'")})
 it('rejects extra SQL statements in column expressions and duplicate names',()=>{const draft=newDesign();Object.assign(draft.fields[0]!,{name:'a',defaultMode:'expression',defaultValue:'1); DROP TABLE x;'});expect(()=>designSQL('qa','a',draft)).toThrow();draft.fields[0]!.defaultMode='none';draft.fields.push({...newField(),name:'A'});expect(()=>designSQL('qa','a',draft)).toThrow('字段名不能重复')})
 it('quotes table comments and exact string defaults without numeric coercion',()=>{const draft=newDesign();Object.assign(draft.fields[0]!,{name:'value',defaultMode:'value',defaultValue:'9007199254740993.125'});draft.options.comment="O'Brien\\path";const sql=designSQL('qa','a',draft);expect(sql).toContain("DEFAULT '9007199254740993.125'");expect(sql).toContain("COMMENT='O''Brien\\\\path'")})
 it('builds bounded trigger delimiters and preserves the original definer on replacement',()=>{const draft=newDesign(),before=clone(draft);const trigger={id:uid(),name:'qa_trigger',timing:'BEFORE' as const,event:'INSERT' as const,body:"BEGIN SET NEW.a='hello'; END",raw:'CREATE DEFINER=`qa`@`localhost` TRIGGER `qa_trigger` BEFORE INSERT ON `a` FOR EACH ROW SET NEW.a=1'};before.triggers=[clone(trigger)];draft.triggers=[trigger];draft.triggers[0]!.body="BEGIN SET NEW.a='$bajiu0$'; END";const sql=triggerScript('qa','a',draft,before);expect(sql).toContain('DROP TRIGGER `qa`.`qa_trigger`;');expect(sql).toContain('DEFINER=`qa`@`localhost`');expect(sql).toContain('DELIMITER $bajiu1$')})
 it('keeps incomplete index input editable during column renames',()=>{expect(renameReferences('(`title`','title','caption')).toBe('(`title`')})
 it('translates trigger strings between modes and ignores quoted text inside comments',()=>{expect(convertSQLStrings("BEGIN -- don't alter this\n SET NEW.a='end\\'; END",'NO_BACKSLASH_ESCAPES','')).toContain("SET NEW.a='end\\\\'")})
})
