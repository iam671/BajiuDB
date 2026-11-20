$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
$connection = $null
$transaction = $null
$created = $false
$qualified = ''
try {
    Add-Type -AssemblyName System.Data
    $request = [Console]::In.ReadToEnd() | ConvertFrom-Json
    $options = $request.options
    $provider = ''
    foreach ($candidate in @('Microsoft.ACE.OLEDB.12.0', 'Microsoft.ACE.OLEDB.16.0')) {
        if ([Type]::GetTypeFromProgID($candidate)) { $provider = $candidate; break }
    }
    if ($request.operation -eq 'capabilities') {
        [Console]::Out.Write((@{ ace = [bool]$provider; odbc = $true } | ConvertTo-Json -Compress))
        exit 0
    }
    $format = [string]$options.format
    $writing = $request.operation -eq 'write'
    if ($format -eq 'odbc') {
        if (-not $options.connectionString -or $options.connectionString.Length -gt 8192) { throw '请输入有效的 ODBC 连接字符串' }
        $connection = [Data.Odbc.OdbcConnection]::new([string]$options.connectionString)
        $builder = [Data.Odbc.OdbcCommandBuilder]::new()
    } else {
        if (-not $provider) { throw '未安装匹配位数的 Microsoft Access Database Engine (ACE)，请安装后重试' }
        $cs = [Data.OleDb.OleDbConnectionStringBuilder]::new()
        $cs['Provider'] = [string]$provider
        $cs['Data Source'] = [string]$options.path
        if (-not $writing) { $cs['Mode'] = 'Read' }
        if ($format -eq 'xls') { $cs['Extended Properties'] = $(if ($writing) { 'Excel 8.0;HDR=YES' } else { 'Excel 8.0;HDR=YES;IMEX=1' }) }
        if ($format -eq 'dbf') { $cs['Data Source'] = [IO.Path]::GetDirectoryName([string]$options.path); $cs['Extended Properties'] = 'dBASE IV' }
        $connection = [Data.OleDb.OleDbConnection]::new($cs.ConnectionString)
        $builder = [Data.OleDb.OleDbCommandBuilder]::new()
    }
    $connection.Open()
    function Quote([string]$name) {
        if ($format -eq 'odbc') { return $builder.QuoteIdentifier($name, $connection) }
        return ('[' + $name.Replace(']', ']]') + ']')
    }
    function SourceTables {
        $items = [Collections.Generic.List[object]]::new()
        foreach ($row in $connection.GetSchema('Tables').Rows) {
            if ([string]$row.TABLE_TYPE -notin @('TABLE', 'VIEW')) { continue }
            $name = [string]$row.TABLE_NAME
            if ($format -eq 'xls') { if(-not $name.Trim("'").EndsWith('$')){continue};$name=$name.Trim("'").Replace("''","'") }
            if ($name.StartsWith('MSys') -or $name.StartsWith('_xlnm')) { continue }
            $schema = ''
            if ($row.Table.Columns.Contains('TABLE_SCHEM')) { $schema = [string]$row.TABLE_SCHEM }
            $label = $name
            if ($schema) { $label = $schema + '.' + $name }
            $items.Add(@{ name=$name; schema=$schema; label=$label })
            if ($items.Count -gt 1000) { throw '来源表数超过 1,000' }
        }
        return ,$items
    }
    function Qualified($item) {
        if ($item.schema) { return (Quote $item.schema) + '.' + (Quote $item.name) }
        return Quote $item.name
    }
    function ReadRows([string]$query) {
        $command = $connection.CreateCommand(); $command.CommandTimeout = 60; $command.CommandText = $query
        if ($transaction) { $command.Transaction=$transaction }
        $reader = $command.ExecuteReader()
        try {
            if ($reader.FieldCount -gt 1024) { throw '来源列数超过 1,024' }
            $names = @(); for ($i=0; $i -lt $reader.FieldCount; $i++) { $names += $reader.GetName($i) }
            $rows = [Collections.Generic.List[object]]::new(); $size=0
            while ($reader.Read()) {
                $values = @()
                for ($i=0; $i -lt $reader.FieldCount; $i++) {
                    if ($reader.IsDBNull($i)) { $values += $null; continue }
                    $value = $reader.GetValue($i)
                    if ($value -is [byte[]]) { throw '来源包含二进制字段，不能按文本无损导入' }
                    if ($value -is [DateTime]) { $value=$value.ToString('yyyy-MM-dd HH:mm:ss.fffffff',[Globalization.CultureInfo]::InvariantCulture) }
                    elseif ($value -is [IFormattable]) { $value=$value.ToString($null,[Globalization.CultureInfo]::InvariantCulture) }
                    else { $value=[string]$value }
                    if ($format -eq 'xls') {
                        if ($value -ceq '\N') { $values += $null; continue }
                        if ($value -ceq '\E') { $value='' }
                        elseif ($value.StartsWith('\\') -and $value.TrimStart([char]92) -cin @('N','E')) { $value=$value.Substring(1) }
                    }
                    $values += $value; $size += [Text.Encoding]::UTF8.GetByteCount($value)
                }
                $rows.Add($values)
                if ($rows.Count -gt 10000 -or $rows.Count*$reader.FieldCount -gt 200000 -or $size -gt 8388608) { throw '来源数据超过导入上限，已停止而非截断' }
            }
            return @{ columns=$names; rows=$rows.ToArray() }
        } finally { $reader.Dispose(); $command.Dispose() }
    }
    if (-not $writing) {
        $tables = SourceTables
        if ($request.operation -eq 'inspect') {
            [Console]::Out.Write((@{ tables=@($tables | ForEach-Object {$_.label}) } | ConvertTo-Json -Compress)); exit 0
        }
        $selected = @($tables | Where-Object {$_.label -ceq [string]$options.table})
        if ($selected.Count -ne 1) { throw '请选择来源中的有效表/工作表；不允许执行自定义 SQL' }
        $data = ReadRows ('SELECT * FROM ' + (Qualified $selected[0]))
        [Console]::Out.Write((@{ data=$data } | ConvertTo-Json -Depth 8 -Compress)); exit 0
    }
    $data = $request.data
    $name = [string]$options.table
    if (-not $name -or $name.Length -gt 64 -or $name.Contains([char]0)) { throw '请输入 1–64 字符的新表名' }
    if ($format -eq 'xls' -and ($name.Length -gt 31 -or $name -match '[\s\[\]:*?/\\]')) { throw 'XLS 导出工作表名须不超过 31 字符，不能包含空白或 []:*?/\，请使用下划线' }
    if ($format -eq 'dbf') {
        $name = [IO.Path]::GetFileNameWithoutExtension([string]$options.path)
        foreach ($column in $data.columns) { if ($column -notmatch '^[A-Za-z_][A-Za-z0-9_]{0,9}$') { throw 'DBF 字段名须为 1–10 位 ASCII 字母、数字或下划线，请改用其它格式保留长字段名' } }
        foreach ($row in $data.rows) { foreach ($value in $row) { if ($null -eq $value -or [Text.Encoding]::Default.GetByteCount([string]$value) -gt 254 -or $value -match '[\x00-\x1F]') { throw 'DBF 仅导出不含 NULL、控制字符且在系统代码页下不超过 254 字节的值；请使用 XLSX/JSON 保留原值' } } }
    }
    $qualified = Quote $name
    if ($format -eq 'odbc') {
        foreach ($item in (SourceTables)) { if ($item.name -ieq $name) { throw 'ODBC 导出只创建新表，目标表已存在，请更换名称' } }
        $transaction = $connection.BeginTransaction()
    }
    $definitions = @()
    for ($ci=0;$ci -lt $data.columns.Count;$ci++) {
        $column=$data.columns[$ci]
        $type = 'MEMO'
        if ($format -eq 'odbc') {
            $length=1;foreach($record in $data.rows){if($null -ne $record[$ci]){$length=[Math]::Max($length,([string]$record[$ci]).Length)}}
            if($length -gt 4000){throw 'ODBC 文本超过 4,000 字符，请改用文件导出'}
            $type='VARCHAR('+$length+')'
            if($connection.Driver -match '(?i)ACEODBC|ODBCJT' -and $length -gt 255){$type='MEMO'}
            if($connection.Driver -match '(?i)msodbcsql|sqlncli|sqlsrv'){$type='NVARCHAR('+$length+')'}
        }
        if ($format -eq 'dbf') { $type='CHAR(254)' }
        $definitions += (Quote ([string]$column)) + ' ' + $type
    }
    $command=$connection.CreateCommand();$command.CommandTimeout=60
    if ($transaction) { $command.Transaction=$transaction }
    $command.CommandText='CREATE TABLE '+$qualified+' ('+($definitions -join ',')+')'
    [void]$command.ExecuteNonQuery();$created=$true
    $quotedColumns=@($data.columns | ForEach-Object {Quote ([string]$_)})
    $marks=@($data.columns | ForEach-Object {'?'})
    $command.CommandText='INSERT INTO '+$qualified+' ('+($quotedColumns -join ',')+') VALUES ('+($marks -join ',')+')'
    foreach ($column in $data.columns) {
        $parameter=$command.CreateParameter();$parameter.DbType=[Data.DbType]::String
        $parameter.Size=32767;[void]$command.Parameters.Add($parameter)
    }
    foreach ($row in $data.rows) {
        for ($i=0; $i -lt $data.columns.Count; $i++) {
            $value=$row[$i]
            if($format -eq 'xls'){
                if($null -eq $value){$value='\N'}
                elseif($value -ceq ''){$value='\E'}
                elseif($value.StartsWith('\') -and $value.TrimStart([char]92) -cin @('N','E')){$value='\'+$value}
            }
            if ($null -ne $value -and (($format -eq 'odbc' -and $value.Length -gt 4000) -or $value.Length -gt 32767)) { throw '单元格超过目标驱动文本容量，已停止导出' }
            $command.Parameters[$i].Value=$(if ($null -eq $value) {[DBNull]::Value} else {[string]$value})
        }
        [void]$command.ExecuteNonQuery()
    }
    $command.Dispose()
    if ($format -eq 'odbc') { $check=ReadRows ('SELECT * FROM '+$qualified) }
    else {
        $items=SourceTables
        $match=@($items | Where-Object {$_.name -ieq $name -or $_.name -ieq ($name+'$') -or $_.name -ieq ($name+'.dbf')})
        if ($match.Count -ne 1) { throw ('驱动未生成可核对的表，停止保存。可见表：'+(($items | ForEach-Object {$_.label}) -join ', ')) }
        $check=ReadRows ('SELECT * FROM '+(Qualified $match[0]))
    }
    if ($check.rows.Count -ne $data.rows.Count) { throw '驱动导出行数不一致，停止保存' }
    $expectedRows=@($data.rows | ForEach-Object {ConvertTo-Json -InputObject ([object[]]$_) -Compress -Depth 4} | Sort-Object -CaseSensitive)
    $actualRows=@($check.rows | ForEach-Object {ConvertTo-Json -InputObject ([object[]]$_) -Compress -Depth 4} | Sort-Object -CaseSensitive)
    for($i=0;$i -lt $expectedRows.Count;$i++){if($expectedRows[$i] -cne $actualRows[$i]){throw '驱动改变了数据（精度、编码、NULL 或文本），已停止保存，请改用 XLSX/JSON'}}
    if ($transaction) { $transaction.Commit();$transaction.Dispose();$transaction=$null }
    [Console]::Out.Write('{}')
} catch {
    $message=$_.Exception.Message
    if ($transaction) { try {$transaction.Rollback()} catch {}; $transaction.Dispose();$transaction=$null }
    if ($created -and $format -eq 'odbc') { $message += '；驱动的结构操作可能已提交，新目标表可能保留，请核对数据源。' }
    if($options.connectionString){
        $message=$message.Replace([string]$options.connectionString,'[连接字符串]')
        try{$redactor=[Data.Odbc.OdbcConnectionStringBuilder]::new([string]$options.connectionString);foreach($key in $redactor.Keys){if($key -match '(?i)pwd|password|user|uid'){$secret=[string]$redactor[$key];if($secret){$message=$message.Replace($secret,'***')}}}}catch{}
    }
    $message=$message -replace '(?i)(pwd|password)\s*=\s*[^;]*','$1=***'
    [Console]::Out.Write((@{error=$message} | ConvertTo-Json -Compress))
} finally {
    if ($connection) { $connection.Close();$connection.Dispose() }
}
