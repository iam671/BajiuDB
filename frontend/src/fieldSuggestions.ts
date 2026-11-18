export type FieldTypeSuggestion={type:string;length:string;decimals:string}
const rules:{matches:RegExp;type:string;length:string;decimals?:string}[]=[
 {matches:/^(?:id|.+_id)$/,type:'bigint',length:'20'},
 {matches:/^(?:uuid|.+_uuid)$/,type:'char',length:'36'},
 {matches:/^(?:created_at|updated_at|deleted_at|create_time|update_time|delete_time|.+_datetime)$/,type:'datetime',length:''},
 {matches:/^(?:timestamp|.+_timestamp)$/,type:'timestamp',length:''},
 {matches:/^(?:birthday|birth_date|.+_date)$/,type:'date',length:''},
 {matches:/^(?:is_.+|has_.+|status|enabled|active|deleted|flag)$/,type:'tinyint',length:'1'},
 {matches:/^(?:price|amount|balance|salary|cost|total_amount|.+_(?:price|amount|balance|cost))$/,type:'decimal',length:'18',decimals:'2'},
 {matches:/^(?:sort|sort_order|position|quantity|count|.+_(?:count|num|quantity))$/,type:'int',length:'11'},
 {matches:/^(?:phone|mobile|telephone|.+_(?:phone|mobile))$/,type:'varchar',length:'20'},
 {matches:/^(?:username|user_name|nickname|nick_name)$/,type:'varchar',length:'50'},
 {matches:/^(?:name|.+_name)$/,type:'varchar',length:'100'},
 {matches:/^(?:ip|ip_address|.+_ip)$/,type:'varchar',length:'45'},
 {matches:/^(?:url|.+_url)$/,type:'varchar',length:'500'},
 {matches:/^(?:email|password|password_hash|title|path|avatar|.+_(?:email|path))$/,type:'varchar',length:'255'},
 {matches:/^(?:content|description|remark|remarks|comment|notes)$/,type:'text',length:''},
 {matches:/^(?:json|metadata|settings|.+_json)$/,type:'json',length:''},
]
export function suggestFieldType(name:string):FieldTypeSuggestion|undefined{const rule=rules.find(rule=>rule.matches.test(name.trim().toLowerCase()));return rule?{type:rule.type,length:rule.length,decimals:rule.decimals??''}:undefined}
