<#
.SYNOPSIS
  Gera um dump .sql.gz do banco LOCAL (Windows) para importar no servidor.

.DESCRIPTION
  Le DB_HOST/DB_PORT/DB_NAME/DB_USUARIO/DB_SENHA/DB_TLS_CA do .env da raiz do
  projeto (a senha nunca e exibida; vai para o cliente via variavel MYSQL_PWD,
  nome lido pelo cliente MariaDB), roda o mariadb-dump e compacta em
  deploy/dumps/<db>-<data>.sql.gz. O dump e importado no MariaDB 11.4 do
  servidor (container rotaperfumes-mariadb, deploy/mariadb-docker.sh) pelo
  estagio "Importar dump" do Jenkinsfile.

  SEC-41: so o mariadb-dump (sem fallback para outro cliente). Procura, nesta
  ordem: -DumpExe <caminho>; mariadb-dump.exe no PATH;
  C:\Program Files\MariaDB *\bin\mariadb-dump.exe. O executavel precisa se
  identificar como MariaDB em --version.

  TLS (SEC-41):
    - DB_TLS_CA definido: --ssl-ca=<caminho resolvido a partir da raiz do
      projeto> e --ssl-verify-server-cert (certificado e nome conferidos).
    - DB_TLS_CA vazio: so com DB_HOST loopback (localhost, 127.x, ::1), sem
      TLS e com AVISO; host remoto sem DB_TLS_CA aborta.
  O .sql intermediario e sempre apagado (finally); so o .gz fica.

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
    [string]$DumpExe = ''
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

function Find-DumpExe([string]$Explicit) {
    if ($Explicit) {
        if (Test-Path -LiteralPath $Explicit) { return $Explicit }
        throw "Executavel de dump informado nao existe: $Explicit"
    }
    $cmd = Get-Command mariadb-dump.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $candidates = @(Get-ChildItem -Path 'C:\Program Files\MariaDB *\bin\mariadb-dump.exe' -ErrorAction SilentlyContinue |
                    Sort-Object FullName -Descending)
    if ($candidates.Count -gt 0) { return $candidates[0].FullName }
    throw 'mariadb-dump.exe nao encontrado (instale o cliente do MariaDB ou use -DumpExe <caminho>).'
}

# SEC-41: so o mariadb-dump (o --version precisa dizer MariaDB).
function Assert-DumpIsMariaDB([string]$Exe) {
    $prev = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $ver = (& $Exe --version 2>&1 | Out-String)
    } finally { $ErrorActionPreference = $prev }
    if ($ver -notmatch 'MariaDB') { throw "O executavel nao e o mariadb-dump do MariaDB: $Exe" }
}

# Loopback literal (sem DNS): localhost, 127.0.0.0/8 ou ::1.
function Test-HostLoopback([string]$HostName) {
    $h = $HostName.Trim().Trim('[', ']')
    if ($h -ieq 'localhost') { return $true }
    $ip = $null
    if ([System.Net.IPAddress]::TryParse($h, [ref]$ip)) { return [System.Net.IPAddress]::IsLoopback($ip) }
    return $false
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

$dumpExe = Find-DumpExe $DumpExe
Assert-DumpIsMariaDB $dumpExe

# SEC-41: TLS. Caminho relativo do DB_TLS_CA = a partir da raiz do projeto.
$projRoot = (Resolve-Path -LiteralPath (Join-Path $scriptDir '..')).Path
$dbTlsCa = Get-EnvValue $vars 'DB_TLS_CA'
$tlsArgs = @()
if ($dbTlsCa) {
    $caPath = $dbTlsCa
    if (-not [System.IO.Path]::IsPathRooted($caPath)) { $caPath = Join-Path $projRoot $caPath }
    if (-not (Test-Path -LiteralPath $caPath -PathType Leaf)) { throw "DB_TLS_CA nao encontrado: $caPath" }
    $caPath = (Resolve-Path -LiteralPath $caPath).Path
    $tlsArgs = @("--ssl-ca=$caPath", '--ssl-verify-server-cert')
} elseif (Test-HostLoopback $dbHost) {
    Write-Warning "DB_TLS_CA vazio: dump SEM TLS (permitido so porque DB_HOST=$dbHost e loopback)."
    $tlsArgs = @('--skip-ssl')
} else {
    throw "DB_TLS_CA vazio e DB_HOST=$dbHost nao e loopback: defina DB_TLS_CA (ex.: deploy/ca-cert.pem) para o dump usar TLS verificado."
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$sqlFile = Join-Path $OutDir "$dbName-$stamp.sql"
$gzFile = "$sqlFile.gz"

Write-Host "dump: $dumpExe"
Write-Host "Banco: $dbName em ${dbHost}:$dbPort (usuario $dbUser; TLS: $(if ($dbTlsCa) { 'verificado' } else { 'desligado (loopback)' }))"

$dumpArgs = @(
    "--host=$dbHost", "--port=$dbPort", "--user=$dbUser"
) + $tlsArgs + @(
    '--single-transaction', '--routines', '--triggers', '--no-tablespaces',
    '--default-character-set=utf8mb4',
    '--hex-blob', '--quick',
    "--result-file=$sqlFile",
    $dbName
)

$prevPwd = $env:MYSQL_PWD
$prevEap = $ErrorActionPreference
try {
    $env:MYSQL_PWD = $dbPass
    # stderr do dump (avisos) nao deve virar exception; checamos o exit code.
    $ErrorActionPreference = 'Continue'
    & $dumpExe @dumpArgs
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($code -ne 0) { throw "dump falhou (exit $code): $dumpExe" }

    Compress-Gzip $sqlFile $gzFile
} finally {
    $ErrorActionPreference = $prevEap
    if ($null -eq $prevPwd) { Remove-Item Env:MYSQL_PWD -ErrorAction SilentlyContinue } else { $env:MYSQL_PWD = $prevPwd }
    if (Test-Path -LiteralPath $sqlFile) { Remove-Item -LiteralPath $sqlFile -Force }
}

$size = [math]::Round((Get-Item -LiteralPath $gzFile).Length / 1KB, 1)
Write-Host "Dump gerado: $gzFile ($size KB)"
Write-Host 'Copie para o servidor (ex. /opt/rotaperfumes/dumps/rotaperfumes.sql.gz) e rode o Jenkins com IMPORTAR_DUMP=true.'
