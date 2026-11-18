<?php
declare(strict_types=1);
// Pure in-process fake: no database connection is created.
require __DIR__.'/../tunnel/admin.php';
final class SyntaxFailureDatabase extends mysqli {
    public int $calls=0;
    public function __construct() {}
    public function query(string $query,int $result_mode=MYSQLI_STORE_RESULT): mysqli_result|bool {
        $this->calls++;
        throw new mysqli_sql_exception("Syntax error near 'INVALID' at line 2",1064);
    }
}
putenv('BAJIU_TUNNEL_ALLOW_SQL=1');
$db=new SyntaxFailureDatabase();
$reply=databaseAdmin($db,'',['operation'=>'query','statements'=>['ALTER TABLE `qa` INVALID','SELECT 1']]);
if(($reply['errorCode']??0)!==1064||$reply['completed']!==0||$db->calls!==1||!str_contains($reply['error'],'1064')||!str_contains($reply['error'],"near 'INVALID' at line 2")){
    throw new RuntimeException('SQL failure must preserve details and stop before subsequent statements');
}
echo "PHP SQL error details check passed\n";
