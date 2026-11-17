// Run with Node 22+: node --experimental-strip-types scripts/table-workflow-fixtures.mjs
import {mkdirSync,writeFileSync} from 'node:fs'
import {fileURLToPath} from 'node:url'
import {generatedSQL,importSQL,parseCSV,suggestedExpression} from '../frontend/src/tableData.ts'
const directory=new URL('../.local/table-workflows-qa/',import.meta.url)
mkdirSync(directory,{recursive:true})
writeFileSync(new URL('create.sql',directory),'CREATE TABLE `csv_data` (`id` BIGINT NOT NULL AUTO_INCREMENT, `name` VARCHAR(80) NOT NULL, `amount` DECIMAL(20,3) NULL, `computed` BIGINT GENERATED ALWAYS AS (`amount`+1) VIRTUAL, PRIMARY KEY (`id`)) ENGINE=InnoDB;')
writeFileSync(new URL('import.sql',directory),importSQL('csv_data',parseCSV('name,amount\r\n"O\'Brien, 演示\nnext",9007199254740993.125\r\nnullable,\\N\r\n'),['name','amount']))
writeFileSync(new URL('generate.sql',directory),generatedSQL('csv_data',2,7,[{name:'name',expression:suggestedExpression('varchar(80)','name')}]))
console.log('Generated table workflow integration fixtures: '+fileURLToPath(directory))
