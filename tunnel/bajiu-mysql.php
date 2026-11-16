<?php
// Bajiu DB's own metadata tunnel. Deploy with PHP 8.1+, mysqli and mysqlnd.
declare(strict_types=1);
ini_set('display_errors', '0');
header('Content-Type: application/json; charset=utf-8');
header('Cache-Control: no-store');
header('X-Content-Type-Options: nosniff');
const PROTOCOL = 'bajiu-mysql-v1';
function reply(bool $ok, mixed $data = null, int $code = 0, int $status = 200): never {
    http_response_code($status);
    $json = json_encode(['protocol'=>PROTOCOL,'ok'=>$ok,'code'=>$code,'data'=>$data], JSON_UNESCAPED_UNICODE | JSON_INVALID_UTF8_SUBSTITUTE);
    if ($json === false || strlen($json) > 8388608) { http_response_code(500); $json='{"protocol":"bajiu-mysql-v1","ok":false,"code":0,"data":null}'; }
    echo $json; exit;
}
function identifier(string $name): string { return '`'.str_replace('`','``',$name).'`'; }
function nameValue(mixed $raw): string {
    if (!is_string($raw) || $raw === '' || strlen($raw)>256 || str_contains($raw,"\0")) { throw new InvalidArgumentException('name'); }
    return $raw;
}
function queryRows(mysqli $db,string $sql,array $parameters=[]): array {
    $stmt=$db->prepare($sql);
    if ($parameters) { $types=str_repeat('s',count($parameters)); $stmt->bind_param($types,...$parameters); }
    $stmt->execute(); $result=$stmt->get_result(); $rows=$result->fetch_all(MYSQLI_ASSOC); $stmt->close(); return $rows;
}
if (($_SERVER['REQUEST_METHOD']??'')!=='POST') reply(false,null,0,405);
$token=(string)getenv('BAJIU_TUNNEL_TOKEN');
if (strlen($token)<32) reply(false,null,0,503);
$provided=(string)($_SERVER['HTTP_X_BAJIU_TOKEN']??'');
if ($provided==='' && str_starts_with((string)($_SERVER['HTTP_AUTHORIZATION']??''),'Bearer ')) $provided=substr($_SERVER['HTTP_AUTHORIZATION'],7);
if (!hash_equals($token,$provided)) reply(false,null,0,401);
$raw=file_get_contents('php://input',false,null,0,2097153);
if ($raw===false || strlen($raw)>2097152) reply(false,null,0,413);
$db=null; $caFile=null;
try {
    $request=json_decode($raw,true,32,JSON_THROW_ON_ERROR);
    if (($request['protocol']??'')!==PROTOCOL || !in_array($request['operation']??'', ['info','objects','ddl'],true)) throw new InvalidArgumentException('protocol');
    $connection=$request['connection']??[];
    $host=nameValue($connection['host']??null); $port=(int)($connection['port']??0);
    $allowed=array_map('trim',explode(',',(string)(getenv('BAJIU_TUNNEL_ALLOWED_HOSTS')?:'127.0.0.1,localhost,::1')));
    if (!in_array($host,$allowed,true)) reply(false,null,0,403);
    if ($port<1||$port>65535||!is_string($connection['username']??null)||strlen($connection['username'])>256||!is_string($connection['password']??null)||strlen($connection['password'])>8192) throw new InvalidArgumentException('connection');
    $timeout=max(1,min(30,(int)($connection['timeout']??10))); set_time_limit($timeout+5);
    mysqli_report(MYSQLI_REPORT_ERROR|MYSQLI_REPORT_STRICT);
    $db=mysqli_init();$db->options(MYSQLI_OPT_CONNECT_TIMEOUT,$timeout);$db->options(MYSQLI_OPT_READ_TIMEOUT,$timeout);$db->options(MYSQLI_OPT_LOCAL_INFILE,false);
    $verify=($connection['tlsMode']??'disabled')==='verify'; $flags=0;
    if ($verify) {
        $ca=(string)($connection['ca']??'');if(strlen($ca)>1048576)throw new InvalidArgumentException('ca');
        if ($ca!=='') { $caFile=tempnam(sys_get_temp_dir(),'bajiu-ca-');if($caFile===false||file_put_contents($caFile,$ca)===false)throw new RuntimeException('ca');chmod($caFile,0600); }
        $db->options(MYSQLI_OPT_SSL_VERIFY_SERVER_CERT,true);$db->ssl_set(null,null,$caFile?:null,null,null);$flags=MYSQLI_CLIENT_SSL;
    }
    $default=(string)($connection['database']??'');if(strlen($default)>256||str_contains($default,"\0"))throw new InvalidArgumentException('database');
    $db->real_connect($host,$connection['username'],$connection['password'],$default!==''?$default:null,$port,null,$flags);$db->set_charset('utf8mb4');
    if ($verify) { $cipher=$db->query("SHOW SESSION STATUS LIKE 'Ssl_cipher'")->fetch_row(); if (!$cipher || $cipher[1]==='') throw new RuntimeException('TLS required'); }
    $operation=$request['operation'];
    if ($operation==='info') {
        $row=$db->query('SELECT VERSION(), CURRENT_USER()')->fetch_row();$databases=[];
        $result=$db->query('SHOW DATABASES');while($entry=$result->fetch_row()){$databases[]=$entry[0];if(count($databases)>=10000)break;}
        $data=['connected'=>true,'serverVersion'=>$row[0],'currentUser'=>$row[1],'databases'=>$databases];
    } else {
        $schema=nameValue($request['database']??null);
        if ($operation==='ddl') {
            $table=nameValue($request['table']??null);
            $visible=queryRows($db,"SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND TABLE_TYPE='BASE TABLE'",[$schema,$table]);
            if(count($visible)!==1)throw new InvalidArgumentException('table');
            $result=$db->query('SHOW CREATE TABLE '.identifier($schema).'.'.identifier($table))->fetch_row();$data=$result[1];if(strlen($data)>1048576)throw new RuntimeException('DDL too large');
        } else {
            $kind=$request['kind']??'';
            if($kind==='tables'||$kind==='views'){
                $type=$kind==='tables'?'BASE TABLE':'VIEW';$alternate=$kind==='tables'?'BASE TABLE':'SYSTEM VIEW';
                $rows=queryRows($db,"SELECT TABLE_NAME AS name,COALESCE(ENGINE,'') AS engine,CAST(TABLE_ROWS AS CHAR) AS `rows`,CAST(DATA_LENGTH AS CHAR) AS dataLength,CAST(INDEX_LENGTH AS CHAR) AS indexLength,CAST(AUTO_INCREMENT AS CHAR) AS autoIncrement,COALESCE(TABLE_COMMENT,'') AS comment,CAST(CREATE_TIME AS CHAR) AS createdAt,CAST(UPDATE_TIME AS CHAR) AS modifiedAt FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_TYPE IN (?,?) ORDER BY TABLE_NAME LIMIT 10001",[$schema,$type,$alternate]);
            }elseif($kind==='functions'){
                $rows=queryRows($db,"SELECT ROUTINE_NAME AS name,ROUTINE_TYPE AS engine,ROUTINE_COMMENT AS comment,CAST(CREATED AS CHAR) AS createdAt,CAST(LAST_ALTERED AS CHAR) AS modifiedAt FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=? ORDER BY ROUTINE_NAME LIMIT 10001",[$schema]);
            }elseif($kind==='events'){
                $rows=queryRows($db,"SELECT EVENT_NAME AS name,STATUS AS engine,EVENT_COMMENT AS comment,CAST(CREATED AS CHAR) AS createdAt,CAST(LAST_ALTERED AS CHAR) AS modifiedAt FROM information_schema.EVENTS WHERE EVENT_SCHEMA=? ORDER BY EVENT_NAME LIMIT 10001",[$schema]);
            }else{throw new InvalidArgumentException('kind');}
            $truncated=count($rows)>10000;$rows=array_slice($rows,0,10000);
            foreach($rows as &$row){foreach($row as &$value){$value=(string)($value??'');}unset($value);$row['kind']=$kind;}unset($row);
            $data=['objects'=>$rows,'truncated'=>$truncated];
        }
    }
    $db->close();$db=null;if($caFile)unlink($caFile);$caFile=null;reply(true,$data);
} catch(mysqli_sql_exception $error) {
    if($db){try{$db->close();}catch(Throwable $ignored){}}if($caFile)@unlink($caFile);reply(false,null,(int)$error->getCode());
} catch(Throwable $error) {
    if($db){try{$db->close();}catch(Throwable $ignored){}}if($caFile)@unlink($caFile);reply(false,null,0);
}
