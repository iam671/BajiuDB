<?php
// Included only after token/host verification by bajiu-mysql.php.
declare(strict_types=1);
function adminQuery(mysqli $db,string $sql,int $limit=1000): array {
    $result=$db->query($sql,MYSQLI_USE_RESULT);
    $reply=['columns'=>[],'rows'=>[],'affected'=>'','truncated'=>false];
    if($result===true){$reply['affected']=(string)$db->affected_rows;return $reply;}
    foreach($result->fetch_fields() as $field)$reply['columns'][]=$field->name;
    $bytes=0;
    try{while($row=$result->fetch_row()){
        if(count($reply['rows'])>=$limit){$reply['truncated']=true;break;}
        foreach($row as &$value){if($value!==null){$value=(string)$value;if(preg_match('//u',$value)!==1)$value='0x'.bin2hex($value);$bytes+=strlen($value);if($bytes>8388608)throw new RuntimeException('Result exceeds 8 MB');}}unset($value);$reply['rows'][]=$row;
    }}finally{$result->free();while($db->more_results()){$db->next_result();$extra=$db->store_result();if($extra instanceof mysqli_result)$extra->free();}}
    return $reply;
}
function adminText(string $value): string{return "CONVERT(X'".bin2hex($value)."' USING utf8mb4)";}
function adminAppend(string &$text,string $more):void{if(strlen($text)+strlen($more)>50331648)throw new RuntimeException('Dump too large');$text.=$more;}
function adminDump(mysqli $db,string $schema,bool $data):string{
    $tables=adminQuery($db,'SHOW FULL TABLES FROM '.identifier($schema),10000);if($tables['truncated'])throw new RuntimeException('Too many tables');
    $text="-- Bajiu DB: tables and views only; routines/events/triggers excluded.\n-- InnoDB snapshot; concurrent DDL and nontransactional engines are not covered.\nSET NAMES utf8mb4;\nSET @BAJIU_OLD_FK=@@FOREIGN_KEY_CHECKS;\nSET FOREIGN_KEY_CHECKS=0;\n";
    $db->query('SET TRANSACTION ISOLATION LEVEL REPEATABLE READ');$db->begin_transaction(MYSQLI_TRANS_START_READ_ONLY|MYSQLI_TRANS_START_WITH_CONSISTENT_SNAPSHOT);
    try{foreach([false,true] as $view){if($view){foreach($tables['rows'] as $table){if($table[1]!=='VIEW')continue;$name=$table[0];$columns=adminQuery($db,'SELECT COLUMN_NAME,COLUMN_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='.adminText($schema).' AND TABLE_NAME='.adminText($name).' ORDER BY ORDINAL_POSITION',10000);$definitions=array_map(fn($c)=>identifier($c[0]).' '.$c[1],$columns['rows']);if(!$definitions)throw new RuntimeException('Empty view');adminAppend($text,'CREATE TABLE '.identifier($name).' ('.implode(',',$definitions).");\n");}}foreach($tables['rows'] as $table){if(($table[1]==='VIEW')!==$view)continue;$name=$table[0];$kind=$view?'VIEW':'TABLE';
        $create=adminQuery($db,'SHOW CREATE '.$kind.' '.identifier($schema).'.'.identifier($name),1);if($view)adminAppend($text,"DROP TABLE ".identifier($name).";\n");adminAppend($text,"\n".$create['rows'][0][1].";\n");if(!$data||$view)continue;
        $columns=adminQuery($db,'SELECT COLUMN_NAME,DATA_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='.adminText($schema).' AND TABLE_NAME='.adminText($name)." AND GENERATION_EXPRESSION='' ORDER BY ORDINAL_POSITION",10000);
        $names=array_map(fn($column)=>identifier($column[0]),$columns['rows']);if(!$names)continue;
        $rows=$db->query('SELECT '.implode(',',$names).' FROM '.identifier($schema).'.'.identifier($name),MYSQLI_USE_RESULT);
        try{while($row=$rows->fetch_row()){$values=[];foreach($row as $i=>$value){$type=strtolower($columns['rows'][$i][1]);$numeric=in_array($type,['tinyint','smallint','mediumint','int','bigint','decimal','float','double','year'],true);$binary=in_array($type,['binary','varbinary','tinyblob','blob','mediumblob','longblob','bit','geometry','point','linestring','polygon','multipoint','multilinestring','multipolygon','geometrycollection'],true);$literal="X'".bin2hex((string)$value)."'";$values[]=$value===null?'NULL':($numeric?(string)$value:($binary?$literal:'CONVERT('.$literal.' USING utf8mb4)'));}adminAppend($text,'INSERT INTO '.identifier($name).' ('.implode(',',$names).') VALUES ('.implode(',',$values).");\n");}}finally{$rows->free();}
    }}}finally{$db->rollback();}
    return $text."\nSET FOREIGN_KEY_CHECKS=@BAJIU_OLD_FK;\n";
}
function databaseAdmin(mysqli $db,string $schema,array $request):array{
    $operation=$request['operation']??'';$reply=['results'=>[],'completed'=>0,'error'=>'','text'=>''];
    if(in_array($operation,['query','create','alter','drop'],true)&&getenv('BAJIU_TUNNEL_ALLOW_SQL')!=='1')throw new RuntimeException('Set BAJIU_TUNNEL_ALLOW_SQL=1');
    if($operation!=='catalog'&&$operation!=='create'&&$schema!=='')$db->select_db($schema);
    if($operation==='catalog'){$reply['results'][]=adminQuery($db,'SHOW COLLATION',10000);return $reply;}
    if($operation==='schema'){$reply['results'][]=adminQuery($db,'SELECT DEFAULT_CHARACTER_SET_NAME,DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='.adminText($schema),1);return $reply;}
    if(in_array($operation,['create','alter','drop'],true)){
        $name=nameValue($request['name']??null);if(preg_match('/[.\/\\\\\r\n]/',$name)||strtolower($name)==='mysql'||in_array(strtolower($name),['sys','information_schema','performance_schema'],true))throw new InvalidArgumentException('name');if($operation!=='create'&&$name!==$schema)throw new InvalidArgumentException('target');
        $sql=strtoupper($operation).' DATABASE '.identifier($name);
        if($operation!=='drop'){$charset=(string)($request['charset']??'');$collation=(string)($request['collation']??'');if(!preg_match('/^[A-Za-z0-9_]+$/',$charset)||!preg_match('/^[A-Za-z0-9_]+$/',$collation))throw new InvalidArgumentException('charset');$found=queryRows($db,'SELECT COLLATION_NAME FROM information_schema.COLLATIONS WHERE CHARACTER_SET_NAME=? AND COLLATION_NAME=?',[$charset,$collation]);if(count($found)!==1)throw new InvalidArgumentException('collation');$sql.=' CHARACTER SET '.$charset.' COLLATE '.$collation;}
        $db->query($sql);$reply['completed']=1;return $reply;
    }
    if($operation==='query'){
        $statements=$request['statements']??[];if(!is_array($statements)||count($statements)===0||count($statements)>10000)throw new InvalidArgumentException('statements');$bytes=0;$resultBytes=0;
        foreach($statements as $sql){if(!is_string($sql)||($bytes+=strlen($sql))>8388608)throw new InvalidArgumentException('sql');try{$result=adminQuery($db,$sql);$reply['completed']++;foreach($result['rows'] as $row){foreach($row as $value){$resultBytes+=strlen((string)$value);}}if($resultBytes>16777216){$reply['error']='结果累计超过 16 MB，已停止继续执行；已提交操作不会自动回滚';break;}if(count($reply['results'])<100)$reply['results'][]=$result;}catch(mysqli_sql_exception $error){$reply['error']='第 '.($reply['completed']+1).' 条语句失败（MySQL '.$error->getCode().'）；已提交操作不会自动回滚';break;}}
        return $reply;
    }
    if($operation==='dictionary'||$operation==='model'){
        $s=adminText($schema);$queries=["SELECT TABLE_NAME,TABLE_TYPE,COALESCE(ENGINE,''),COALESCE(TABLE_COMMENT,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA=$s ORDER BY TABLE_NAME","SELECT TABLE_NAME,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,COLUMN_DEFAULT,COLUMN_KEY,EXTRA,COLUMN_COMMENT FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=$s ORDER BY TABLE_NAME,ORDINAL_POSITION","SELECT TABLE_NAME,COLUMN_NAME,REFERENCED_TABLE_SCHEMA,REFERENCED_TABLE_NAME,REFERENCED_COLUMN_NAME,CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=$s AND REFERENCED_TABLE_NAME IS NOT NULL ORDER BY TABLE_NAME,CONSTRAINT_NAME,ORDINAL_POSITION"];
        foreach($queries as $sql){$result=adminQuery($db,$sql,10000);if($result['truncated'])throw new RuntimeException('Structure too large');$reply['results'][]=$result;}return $reply;
    }
    if($operation==='dump'){$reply['text']=adminDump($db,$schema,(bool)($request['includeData']??false));return $reply;}
    throw new InvalidArgumentException('operation');
}
