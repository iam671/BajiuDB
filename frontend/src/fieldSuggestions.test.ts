import {expect,it} from 'vitest'
import {suggestFieldType} from './fieldSuggestions'
it('matches common names without changing unknown fields or inventing key attributes',()=>{
 expect(suggestFieldType(' USER_ID ')).toEqual({type:'bigint',length:'20',decimals:''})
 expect(suggestFieldType('price')).toEqual({type:'decimal',length:'18',decimals:'2'})
 expect(suggestFieldType('created_at')).toEqual({type:'datetime',length:'',decimals:''})
 expect(suggestFieldType('username')).toEqual({type:'varchar',length:'50',decimals:''})
 expect(suggestFieldType('is_active')?.type).toBe('tinyint');expect(suggestFieldType('arbitrary_custom_field')).toBeUndefined()
})
