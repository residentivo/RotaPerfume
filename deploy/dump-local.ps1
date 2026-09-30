<#
.SYNOPSIS
  Gera um dump .sql.gz do banco LOCAL (Windows) para importar no servidor.

.DESCRIPTION
  Le DB_HOST/DB_PORT/DB_NAME/DB_USUARIO/DB_SENHA do .env da raiz do projeto
  (a senha nunca e exibida; vai para o mysqldump via variavel MYSQL_PWD),
  roda o mysqldump e compacta em deploy/dumps/<db>-<data>.sql.gz.

  Depois copie para o servidor, por exemplo:
    scp deploy\dumps\rotaperfumes-AAAAMMDD-HHMMSS.sql.gz usuario@192.168.168.106:/opt/rotaperfumes/dumps/rotaperfumes.sql.gz
  e rode o job do Jenkins com IMPORTAR_DUMP=true.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File deploy\dump-local.ps1
#>
[CmdletBinding()]
param(
    [string]$EnvFile = '',
    [string]$OutDir = '',
    [string]$MysqlDump = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Defaults relativos ao script (resolvidos aqui: no Windows PowerShell 5.1
# $PSScriptRoot pode vir vazio no bloco param).
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $EnvFile) { $EnvFile = Join-Path $scriptDir '..\.env' }
if (-not $OutDir) { $OutDir = Join-Path $scriptDir 'dumps' }

function Read-DotEnv([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { throw "Arquivo .env nao encontrado: $Path" }
    $vars = @{}
    foreach ($line in Get-Content -LiteralPath $Path) {
        $l = $line.TrimEnd("`r")
        if ($l -match '^\s*(#|$)') { continue }
        if ($l -notmatch '^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=(.*)$') { continue }
        $value = $Matches[2].Trim()
        if ($value.Length -ge 2 -and (($value[0] -eq '"' -and $value[-1] -eq '"') -or ($value[0] -eq "'" -and $value[-1] -eq "'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        $vars[$Matches[1]] = $value
    }
    return $vars
}

function Get-EnvValue([hashtable]$Vars, [string]$Key, [string]$Default = '') {
    if ($Vars.ContainsKey($Key) -and $Vars[$Key] -ne '') { return $Vars[$Key] }
    return $Default
}

function Find-MysqlDump([string]$Explicit) {
    if ($Explicit) {
        if (Test-Path -LiteralPath $Explicit) { return $Explicit }
        throw "mysqldump informado nao existe: $Explicit"
    }
    $cmd = Get-Command mysqldump.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    # Prefere o do Server (mesma versao do banco); depois Workbench.
    $candidates = @(Get-ChildItem -Path 'C:\Program Files\MySQL\MySQL Server*\bin\mysqldump.exe' -ErrorAction SilentlyContinue) +
                  @(Get-ChildItem -Path 'C:\Program Files\MySQL\MySQL Workbench*\mysqldump.exe' -ErrorAction SilentlyContinue)
    if ($candidates.Count -gt 0) { return $candidates[0].FullName }
    throw 'mysqldump.exe nao encontrado (instale o MySQL Server/Workbench ou use -MysqlDump <caminho>).'
}

function Compress-Gzip([string]$Source, [string]$Destination) {
    $in = [System.IO.File]::OpenRead($Source)
    try {
        $out = [System.IO.File]::Create($Destination)
        try {
            $gz = New-Object System.IO.Compression.GZipStream($out, [System.IO.Compression.CompressionLevel]::Optimal)
            try { $in.CopyTo($gz) } finally { $gz.Dispose() }
        } finally { $out.Dispose() }
    } finally { $in.Dispose() }
}

$vars = Read-DotEnv (Resolve-Path -LiteralPath $EnvFile).Path
$dbHost = Get-EnvValue $vars 'DB_HOST' 'localhost'
$dbPort = Get-EnvValue $vars 'DB_PORT' '3306'
$dbName = Get-EnvValue $vars 'DB_NAME' 'rotaperfumes'
$dbUser = Get-EnvValue $vars 'DB_USUARIO'
$dbPass = Get-EnvValue $vars 'DB_SENHA'
if (-not $dbUser -or -not $dbPass) { throw 'DB_USUARIO/DB_SENHA ausentes no .env' }

$dumpExe = Find-MysqlDump $MysqlDump
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$sqlFile = Join-Path $OutDir "$dbName-$stamp.sql"
$gzFile = "$sqlFile.gz"

Write-Host "mysqldump: $dumpExe"
Write-Host "Banco: $dbName em ${dbHost}:$dbPort (usuario $dbUser)"

$dumpArgs = @(
    "--host=$dbHost", "--port=$dbPort", "--user=$dbUser",
    '--single-transaction', '--routines', '--triggers', '--no-tablespaces',
    '--set-gtid-purged=OFF', '--default-character-set=utf8mb4',
    '--hex-blob', '--quick',
    "--result-file=$sqlFile",
    $dbName
)

$prevPwd = $env:MYSQL_PWD
$prevEap = $ErrorActionPreference
try {
    $env:MYSQL_PWD = $dbPass
    # stderr do mysqldump (avisos) nao deve virar exception; checamos o exit code.
    $ErrorActionPreference = 'Continue'
    & $dumpExe @dumpArgs
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($code -ne 0) { throw "mysqldump falhou (exit $code)" }

    Compress-Gzip $sqlFile $gzFile
} finally {
    $ErrorActionPreference = $prevEap
    if ($null -eq $prevPwd) { Remove-Item Env:MYSQL_PWD -ErrorAction SilentlyContinue } else { $env:MYSQL_PWD = $prevPwd }
    if (Test-Path -LiteralPath $sqlFile) { Remove-Item -LiteralPath $sqlFile -Force }
}

$size = [math]::Round((Get-Item -LiteralPath $gzFile).Length / 1KB, 1)
Write-Host "Dump gerado: $gzFile ($size KB)"
Write-Host 'Copie para o servidor (ex. /opt/rotaperfumes/dumps/rotaperfumes.sql.gz) e rode o Jenkins com IMPORTAR_DUMP=true.'
