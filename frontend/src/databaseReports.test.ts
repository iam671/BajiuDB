import {describe,it,expect} from 'vitest'
import {dictionaryHTML,modelSVG} from './databaseReports'
import type {AdminResult} from './api'
describe('database report escaping',()=>{it('keeps database metadata as escaped text in HTML and SVG reports',()=>{const attack='</text><script>alert(1)</script>';const data={results:[{rows:[[attack,'BASE TABLE','InnoDB',attack]]},{rows:[[attack,attack,'varchar(20)','YES',null,'PRI','',attack]]},{rows:[]}]} as AdminResult;for(const report of [dictionaryHTML(attack,data),modelSVG(attack,data)]){expect(report).not.toContain('<script>');expect(report).toContain('&lt;script&gt;');expect(report).toContain('&lt;/text&gt;')}})})
