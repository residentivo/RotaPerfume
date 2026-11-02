#Requires -Version 5.1
<#
.SYNOPSIS
    SETUP-01: instala/confere as ferramentas do ambiente de desenvolvimento (Windows).

.DESCRIPTION
    Versões: Go = diretiva "go" dos go.mod; Node = frontend\.nvmrc; demais em
    scripts\versoes.conf. Idempotente: pula o que já está na versão certa.
    Instala via winget; o Go tem fallback para o .zip oficial de https://go.dev/dl/
    (sem admin, em -GoDir). O script não pede senha; o winget/MSI pode abrir o UAC.

.PARAMETER Check
    Só confere as versões (não instala nem altera nada).
.PARAMETER Deps
    No fim, baixa as dependências do projeto (go mod download e make frontend-deps).
.PARAMETER GoDir
    Destino do Go quando instalado pelo .zip oficial (padrão: %LOCALAPPDATA%\Programs\Go).
.PARAMETER GoZip
    Instala o Go pelo .zip oficial mesmo com winget disponível (sem admin).
.PARAMETER PostmanScript
    Instala o Postman CLI pelo script oficial (dl-cli.pstmn.io) em vez do npm "postman-cli".
.PARAMETER ManterGoToolchain
    Não grava GOTOOLCHAIN=local no "go env".

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\setup-windows.ps1 -Check
#>
[CmdletBinding()]
param(
    [switch]$Check,
    [switch]$Deps,
    [string]$GoDir = (Join-Path $env:LOCALAPPDATA 'Programs\Go'),
    [switch]$GoZip,
    [switch]$PostmanScript,
    [switch]$ManterGoToolchain
)

Set-StrictMode -Version 2
$ErrorActionPreference = 'Continue'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Raiz = Split-Path -Parent $PSScriptRoot
$Modo = if ($Check) { 'check' } else { 'instalar' }
$Resultados = New-Object System.Collections.ArrayList
$Script:GoExe = $null
$Script:GoPath = $null

# ---------------------------------------------------------------------------
# Saída e utilitários
# ---------------------------------------------------------------------------
function Titulo([string]$t) { Write-Host ''; Write-Host "== $t ==" -ForegroundColor Cyan }
function Info([string]$t) { Write-Host "   $t" }
function Aviso([string]$t) { Write-Host "   AVISO: $t" -ForegroundColor Yellow }
function Registrar([string]$nome, [string]$status, [string]$detalhe) {
    [void]$Resultados.Add([pscustomobject]@{ Nome = $nome; Status = $status; Detalhe = $detalhe })
}

function Extrair-Versao([string]$texto) {
    if ($texto -match '(\d+(\.\d+)+)') { return $Matches[1] }
    return $null
}
function Normalizar-Versao([string]$v) {
    if (-not $v) { return [version]'0.0' }
    $p = @(($v -split '\.') | Select-Object -First 4)
    while ($p.Count -lt 2) { $p += '0' }
    return [version]($p -join '.')
}
function Versao-Ge([string]$a, [string]$b) {
    if (-not $a) { return $false }
    return (Normalizar-Versao $a) -ge (Normalizar-Versao $b)
}
function Tem([string]$cmd) { return [bool](Get-Command $cmd -ErrorAction SilentlyContinue) }
function Caminho-De([string]$cmd) {
    $c = Get-Command $cmd -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($c) { return $c.Source }
    return $null
}

function Atualizar-PathSessao {
    $m = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $u = [Environment]::GetEnvironmentVariable('Path', 'User')
    $env:Path = (@($m, $u) | Where-Object { $_ }) -join ';'
    foreach ($n in 'NVM_HOME', 'NVM_SYMLINK') {
        $v = [Environment]::GetEnvironmentVariable($n, 'User')
        if (-not $v) { $v = [Environment]::GetEnvironmentVariable($n, 'Machine') }
        if ($v) { Set-Item -Path "env:$n" -Value $v }
    }
}

# Põe $dir no início do PATH do usuário (persistente) e da sessão.
# Lê/grava o Path do usuário cru no registro (preserva entradas como %NVM_SYMLINK%).
function Adicionar-PathUsuario([string]$dir) {
    $k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
    try {
        $u = [string]$k.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $itens = @()
        if ($u) { $itens = @($u -split ';' | Where-Object { $_ -and ($_.TrimEnd('\') -ne $dir.TrimEnd('\')) }) }
        $novo = (@($dir) + $itens) -join ';'
        if ($novo -ne $u) {
            $k.SetValue('Path', $novo, [Microsoft.Win32.RegistryValueKind]::ExpandString)
            # Avisa o Explorer da mudança (WM_SETTINGCHANGE) sem mexer em outra variável.
            [Environment]::SetEnvironmentVariable('ROTAPERFUME_SETUP_TMP', '1', 'User')
            [Environment]::SetEnvironmentVariable('ROTAPERFUME_SETUP_TMP', $null, 'User')
            Info "PATH do usuário: $dir no início (abra um terminal novo)"
        }
    }
    finally { $k.Close() }
    if (-not (($env:Path -split ';') -contains $dir)) { $env:Path = "$dir;$env:Path" }
}

function Ler-Versoes {
    $arq = Join-Path $Raiz 'scripts\versoes.conf'
    if (-not (Test-Path $arq)) { throw "Não encontrei $arq" }
    $h = @{}
    foreach ($linha in Get-Content -Path $arq -Encoding UTF8) {
        if ($linha -match '^\s*([A-Z_][A-Z0-9_]*)=(.*)$') { $h[$Matches[1]] = $Matches[2].Trim() }
    }
    return $h
}

function Versao-GoMod {
    $maior = $null
    foreach ($m in 'backend\rotaperfumes-api\go.mod', 'backend\shared\go.mod') {
        $l = Get-Content -Path (Join-Path $Raiz $m) | Where-Object { $_ -match '^go\s+(\S+)' } | Select-Object -First 1
        if ($l -match '^go\s+(\S+)') {
            $v = $Matches[1]
            if ($maior -and $v -ne $maior) { Aviso "os go.mod pedem versões diferentes ($maior e $v): usando a maior." }
            if (-not $maior -or (Versao-Ge $v $maior)) { $maior = $v }
        }
    }
    return $maior
}

$TemWinget = Tem 'winget'
function Instalar-Winget([string]$id, [string]$versao) {
    if (-not $TemWinget) { Aviso "winget não encontrado (instale o 'App Installer' da Microsoft Store)"; return $false }
    $a = @('install', '--id', $id, '-e', '--silent', '--accept-source-agreements', '--accept-package-agreements')
    if ($versao) { $a += @('--version', $versao) }
    Info "winget $($a -join ' ')"
    & winget @a | Out-Host
    $rc = $LASTEXITCODE
    Atualizar-PathSessao
    # 0x8A15002B: já instalado / sem atualização aplicável.
    return ($rc -eq 0 -or $rc -eq -1978335189)
}

function Baixar([string]$url, [string]$destino) {
    try { Invoke-WebRequest -Uri $url -OutFile $destino -UseBasicParsing; return $true }
    catch { Aviso "falha ao baixar ${url}: $($_.Exception.Message)"; return $false }
}

# ---------------------------------------------------------------------------
# Ferramentas básicas (git, make, openssl)
# ---------------------------------------------------------------------------
function Etapa-Basicas {
    Titulo 'Ferramentas básicas'
    $itens = @(
        @{ Nome = 'git'; Id = 'Git.Git'; Min = $V.GIT_MIN; Arg = '--version' },
        @{ Nome = 'make'; Id = 'ezwinports.make'; Min = $V.MAKE_MIN; Arg = '--version' }
    )
    foreach ($i in $itens) {
        $v = $null
        if (Tem $i.Nome) { $v = Extrair-Versao ((& $i.Nome $i.Arg 2>$null) | Select-Object -First 1) }
        if ($v -and (Versao-Ge $v $i.Min)) { Registrar $i.Nome 'OK' $v; continue }
        if ($Check) {
            if ($v) { Registrar $i.Nome 'AVISO' "$v (mínimo $($i.Min))" } else { Registrar $i.Nome 'FALTA' "winget install $($i.Id)" }
            continue
        }
        [void](Instalar-Winget $i.Id $null)
        if (Tem $i.Nome) { Registrar $i.Nome 'OK' "$(Extrair-Versao ((& $i.Nome $i.Arg 2>$null) | Select-Object -First 1)) (instalado)" }
        else { Registrar $i.Nome 'FALTA' "winget install $($i.Id)" }
    }
    Info 'O Makefile usa sh/grep/awk: rode o make pelo Git Bash.'

    # openssl: o Git for Windows já traz um (usr\bin), usado pelo Git Bash.
    $ossl = Caminho-De 'openssl'
    $osslGit = Join-Path $env:ProgramFiles 'Git\usr\bin\openssl.exe'
    if (-not $ossl -and (Test-Path $osslGit)) { $ossl = $osslGit }
    if (-not $ossl -and -not $Check) {
        [void](Instalar-Winget 'ShiningLight.OpenSSL.Light' $null)
        $ossl = Caminho-De 'openssl'
        $osslSL = Join-Path $env:ProgramFiles 'OpenSSL-Win64\bin\openssl.exe'
        if (-not $ossl -and (Test-Path $osslSL)) { $ossl = $osslSL }
    }
    if ($ossl) {
        $v = Extrair-Versao ((& $ossl version 2>$null) | Select-Object -First 1)
        $st = if (Versao-Ge $v $V.OPENSSL_MIN) { 'OK' } else { 'AVISO' }
        Registrar 'openssl' $st "$v ($ossl)"
    }
    else { Registrar 'openssl' 'FALTA' 'winget install ShiningLight.OpenSSL.Light (ou use o do Git Bash)' }
}

# ---------------------------------------------------------------------------
# Go + GOTOOLCHAIN + covdata
# ---------------------------------------------------------------------------
function Versao-GoDe([string]$exe) {
    $ant = $env:GOTOOLCHAIN
    $env:GOTOOLCHAIN = 'local'
    try { $v = (& $exe env GOVERSION 2>$null | Select-Object -First 1) } finally { $env:GOTOOLCHAIN = $ant }
    if ($v) { return ($v -replace '^go', '') }
    return $null
}

function Go-Aceitavel([string]$v) {
    if (-not $v) { return $false }
    if ($v -eq $GoVersao) { return $true }
    if ($V.GO_ACEITAR_PATCH_MAIOR -ne '1') { return $false }
    $serieV = ($v -split '\.')[0..1] -join '.'
    $serieM = ($GoVersao -split '\.')[0..1] -join '.'
    return ($serieV -eq $serieM -and (Versao-Ge $v $GoVersao))
}

function Procurar-Go {
    $cands = @((Join-Path $GoDir 'bin\go.exe'), (Join-Path $env:ProgramFiles 'Go\bin\go.exe'),
        (Join-Path $env:USERPROFILE "sdk\go$GoVersao\bin\go.exe"), (Caminho-De 'go'))
    $vistos = @{}
    foreach ($c in $cands) {
        if (-not $c -or -not (Test-Path $c)) { continue }
        $c = (Resolve-Path $c).Path
        if ($vistos.ContainsKey($c)) { continue }
        $vistos[$c] = $true
        $v = Versao-GoDe $c
        Info "Go local encontrado: $c ($v)"
        if (Go-Aceitavel $v) { $Script:GoExe = $c; return $true }
    }
    return $false
}

function Instalar-GoZip {
    $arq = switch ($env:PROCESSOR_ARCHITECTURE) { 'ARM64' { 'arm64' } 'x86' { '386' } default { 'amd64' } }
    $nome = "go$GoVersao.windows-$arq.zip"
    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("setup-go-" + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Info "Baixando $nome de https://go.dev/dl/ ..."
        if (-not (Baixar "https://dl.google.com/go/$nome" (Join-Path $tmp $nome))) { return $false }
        if (-not (Baixar "https://dl.google.com/go/$nome.sha256" (Join-Path $tmp 'sha256'))) { return $false }
        $esperado = (Get-Content (Join-Path $tmp 'sha256') -Raw).Trim().ToLower()
        $obtido = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $nome)).Hash.ToLower()
        if ($esperado -ne $obtido) { Aviso "SHA-256 do zip não confere (esperado $esperado, obtido $obtido)"; return $false }
        Expand-Archive -Path (Join-Path $tmp $nome) -DestinationPath (Join-Path $tmp 'x') -Force
        if (Test-Path $GoDir) {
            $verArq = Join-Path $GoDir 'VERSION'
            if (-not ((Test-Path $verArq) -and ((Get-Content $verArq -TotalCount 1) -match '^go'))) {
                Aviso "$GoDir existe e não parece uma instalação do Go: não vou sobrescrever"; return $false
            }
            Remove-Item -Recurse -Force $GoDir
        }
        New-Item -ItemType Directory -Path (Split-Path -Parent $GoDir) -Force | Out-Null
        Move-Item -Path (Join-Path $tmp 'x\go') -Destination $GoDir
        $Script:GoExe = Join-Path $GoDir 'bin\go.exe'
        Info "Go $GoVersao instalado em $GoDir"
        return $true
    }
    finally { Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue }
}

function Smoke-Covdata([string]$exe) {
    # Exercita o covdata como o "make test": -coverpkg com um pacote sem testes.
    $d = Join-Path ([IO.Path]::GetTempPath()) ("smoke-covdata-" + [guid]::NewGuid())
    New-Item -ItemType Directory -Path (Join-Path $d 'a'), (Join-Path $d 't') -Force | Out-Null
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [IO.File]::WriteAllText((Join-Path $d 'go.mod'), "module smoke`n`ngo $GoVersao`n", $utf8)
    [IO.File]::WriteAllText((Join-Path $d 'a\a.go'), "package a`n`nfunc F() int { return 1 }`n", $utf8)
    [IO.File]::WriteAllText((Join-Path $d 't\t_test.go'), "package t_test`n`nimport (`n`t`"testing`"`n`n`t`"smoke/a`"`n)`n`nfunc TestF(t *testing.T) {`n`tif a.F() != 1 {`n`t`tt.Fatal()`n`t}`n}`n", $utf8)
    $antTc = $env:GOTOOLCHAIN; $antFl = $env:GOFLAGS
    $env:GOTOOLCHAIN = 'local'; $env:GOFLAGS = ''
    Push-Location $d
    try {
        $saida = & $exe test -count=1 '-coverpkg=./...' ./... 2>&1
        $ok = ($LASTEXITCODE -eq 0)
        if (-not $ok) { $saida | Select-Object -Last 5 | ForEach-Object { Info "$_" } }
        return $ok
    }
    finally {
        Pop-Location
        $env:GOTOOLCHAIN = $antTc; $env:GOFLAGS = $antFl
        Remove-Item -Recurse -Force $d -ErrorAction SilentlyContinue
    }
}

function Etapa-Go {
    Titulo "Go (go.mod: $GoVersao)"
    if (-not (Procurar-Go)) {
        if ($Check) {
            Registrar 'go' 'FALTA' "nenhum Go local $GoVersao"
            if (Tem 'go') { Registrar 'GOTOOLCHAIN' 'AVISO' "'$(& go env GOTOOLCHAIN 2>$null)': sem Go local, o go troca para um toolchain baixado (TST-08)" }
            Registrar 'covdata' 'FALTA' "sem Go local $GoVersao para conferir (TST-08)"
            return
        }
        $ok = $false
        if ($TemWinget -and -not $GoZip) {
            # O MSI do winget instala em C:\Program Files\Go (pode abrir o UAC).
            if (Instalar-Winget 'GoLang.Go' $GoVersao) { $ok = Procurar-Go }
            if (-not $ok) { Aviso 'winget não deixou o Go certo no PATH; usando o .zip oficial' }
        }
        if (-not $ok) { $ok = Instalar-GoZip }
        if (-not $ok) { Registrar 'go' 'FALTA' "falha ao instalar Go $GoVersao"; return }
    }
    $dirGo = Split-Path -Parent $Script:GoExe
    $v = Versao-GoDe $Script:GoExe
    $ant = $env:GOTOOLCHAIN; $env:GOTOOLCHAIN = 'local'
    $Script:GoPath = ((& $Script:GoExe env GOPATH) -split ';')[0]
    $env:GOTOOLCHAIN = $ant

    $atual = Caminho-De 'go'
    if ($atual) { $atual = (Resolve-Path $atual).Path }
    if ($atual -ne $Script:GoExe) {
        if ($Check) { Registrar 'go' 'AVISO' "$v em $dirGo, mas o 'go' do PATH é $atual" }
        else {
            Adicionar-PathUsuario $dirGo
            $maq = [Environment]::GetEnvironmentVariable('Path', 'Machine')
            if ($maq -match '(?i)\\Go\\bin' -and $dirGo -notlike "$env:ProgramFiles*") {
                Aviso 'há um Go no PATH da máquina (vem antes do PATH do usuário): desinstale-o ou remova-o do PATH do sistema.'
            }
            Registrar 'go' 'OK' "$v em $dirGo (PATH ajustado)"
        }
    }
    else { Registrar 'go' 'OK' "$v em $dirGo" }

    $tc = (& $Script:GoExe env GOTOOLCHAIN 2>$null | Select-Object -First 1)
    if ($tc -eq 'local') { Registrar 'GOTOOLCHAIN' 'OK' 'local' }
    elseif ($env:GOTOOLCHAIN) { Registrar 'GOTOOLCHAIN' 'AVISO' "'$tc' vem da variável de ambiente GOTOOLCHAIN; remova-a (o go env -w não a sobrepõe)" }
    elseif (-not $Check -and -not $ManterGoToolchain) {
        $env:GOTOOLCHAIN = 'local'
        & $Script:GoExe env -w GOTOOLCHAIN=local
        $rc = $LASTEXITCODE
        Remove-Item env:GOTOOLCHAIN
        if ($rc -eq 0) { Registrar 'GOTOOLCHAIN' 'OK' "local (antes: $tc)" } else { Registrar 'GOTOOLCHAIN' 'AVISO' 'rode: go env -w GOTOOLCHAIN=local' }
    }
    else { Registrar 'GOTOOLCHAIN' 'AVISO' "'$tc' (recomendado: go env -w GOTOOLCHAIN=local)" }

    $env:GOTOOLCHAIN = 'local'
    & $Script:GoExe tool -n covdata *> $null
    $temCov = ($LASTEXITCODE -eq 0)
    if ($ant) { $env:GOTOOLCHAIN = $ant } else { Remove-Item env:GOTOOLCHAIN -ErrorAction SilentlyContinue }
    if (-not $temCov) { Registrar 'covdata' 'FALTA' 'go tool -n covdata falhou (TST-08): reinstale o Go oficial' }
    elseif (Smoke-Covdata $Script:GoExe) { Registrar 'covdata' 'OK' 'go tool -n covdata e go test -coverpkg funcionam' }
    else { Registrar 'covdata' 'FALTA' 'go test -coverpkg falhou no teste rápido (TST-08)' }
}

# ---------------------------------------------------------------------------
# Node (nvm-windows, versão do frontend\.nvmrc)
# ---------------------------------------------------------------------------
function Etapa-Node {
    $alvo = ((Get-Content (Join-Path $Raiz 'frontend\.nvmrc') -TotalCount 1).Trim()) -replace '^v', ''
    Titulo "Node (frontend\.nvmrc: $alvo, mínimo $($V.NODE_MIN))"
    $temNvm = Tem 'nvm'
    if ($temNvm) { Registrar 'nvm-windows' 'OK' "$((& nvm version 2>$null) | Select-Object -First 1)" }
    elseif ($Check) { Registrar 'nvm-windows' 'AVISO' 'não encontrado (winget install CoreyButler.NVMforWindows)' }
    else {
        if (Instalar-Winget 'CoreyButler.NVMforWindows' $null) { $temNvm = Tem 'nvm' }
        if ($temNvm) { Registrar 'nvm-windows' 'OK' 'instalado' } else { Registrar 'nvm-windows' 'AVISO' 'não instalado; tentando o Node pelo winget' }
    }

    $v = $null
    if (Tem 'node') { $v = ((& node --version 2>$null) | Select-Object -First 1) -replace '^v', '' }
    if (-not $Check -and $v -ne $alvo) {
        if ($temNvm) {
            Info "nvm install $alvo; nvm use $alvo (o 'nvm use' pode abrir o UAC)"
            & nvm install $alvo | Out-Host
            & nvm use $alvo | Out-Host
        }
        else { [void](Instalar-Winget 'OpenJS.NodeJS.LTS' $alvo) }
        Atualizar-PathSessao
        if (Tem 'node') { $v = ((& node --version 2>$null) | Select-Object -First 1) -replace '^v', '' }
    }
    if ($v -eq $alvo) { Registrar 'node' 'OK' "$v ($(Caminho-De 'node'))" }
    elseif ($v -and (($v -split '\.')[0] -eq ($alvo -split '\.')[0]) -and (Versao-Ge $v $V.NODE_MIN)) {
        Registrar 'node' 'AVISO' "$v (aceito: >= $($V.NODE_MIN); o .nvmrc pede $alvo)"
    }
    elseif ($v) { Registrar 'node' 'FALTA' "$v (precisa $alvo: nvm install $alvo; nvm use $alvo)" }
    else { Registrar 'node' 'FALTA' 'não encontrado' }
    if (Tem 'npm') { Registrar 'npm' 'OK' "$((& npm --version 2>$null) | Select-Object -First 1)" } else { Registrar 'npm' 'FALTA' 'vem com o Node' }
}

# ---------------------------------------------------------------------------
# govulncheck
# ---------------------------------------------------------------------------
function Etapa-Govulncheck {
    Titulo 'govulncheck'
    $gp = $Script:GoPath
    if (-not $gp -and (Tem 'go')) { $gp = ((& go env GOPATH 2>$null) -split ';')[0] }
    if (-not $gp) { $gp = Join-Path $env:USERPROFILE 'go' }
    $gobin = Join-Path $gp 'bin'
    $exe = Caminho-De 'govulncheck'
    if (-not $exe -and (Test-Path (Join-Path $gobin 'govulncheck.exe'))) { $exe = Join-Path $gobin 'govulncheck.exe' }
    if (-not $exe -and -not $Check -and $Script:GoExe) {
        Info "go install golang.org/x/vuln/cmd/govulncheck@$($V.GOVULNCHECK_VERSAO)"
        $env:GOTOOLCHAIN = 'local'
        & $Script:GoExe install "golang.org/x/vuln/cmd/govulncheck@$($V.GOVULNCHECK_VERSAO)"
        Remove-Item env:GOTOOLCHAIN -ErrorAction SilentlyContinue
        if (Test-Path (Join-Path $gobin 'govulncheck.exe')) { $exe = Join-Path $gobin 'govulncheck.exe' }
    }
    if (-not $exe) { Registrar 'govulncheck' 'FALTA' "go install golang.org/x/vuln/cmd/govulncheck@$($V.GOVULNCHECK_VERSAO)"; return }
    $ver = (& $exe -version 2>$null | Where-Object { $_ -match '^Scanner: ' } | Select-Object -First 1) -replace '^Scanner: ', ''
    if (Tem 'govulncheck') { Registrar 'govulncheck' 'OK' "$ver ($exe)" }
    elseif (-not $Check) { Adicionar-PathUsuario $gobin; Registrar 'govulncheck' 'OK' "$ver ($gobin no PATH)" }
    else { Registrar 'govulncheck' 'AVISO' "$ver, mas $gobin não está no PATH" }
}

# ---------------------------------------------------------------------------
# Cliente MariaDB
# ---------------------------------------------------------------------------
function Procurar-Mariadb {
    $c = Caminho-De 'mariadb'
    if ($c) { return $c }
    $dirs = Get-ChildItem -Path $env:ProgramFiles -Directory -Filter 'MariaDB *' -ErrorAction SilentlyContinue |
        Sort-Object { Normalizar-Versao (Extrair-Versao $_.Name) } -Descending
    foreach ($d in $dirs) {
        $e = Join-Path $d.FullName 'bin\mariadb.exe'
        if (Test-Path $e) { return $e }
    }
    return $null
}
function Versao-Mariadb([string]$exe) {
    $s = (& $exe --version 2>$null | Select-Object -First 1)
    if ($s -match '(\d+\.\d+\.\d+)-MariaDB') { return $Matches[1] }
    return $null
}

function Etapa-Mariadb {
    Titulo "Cliente MariaDB (mínimo $($V.MARIADB_CLIENTE_MIN))"
    $exe = Procurar-Mariadb
    if (-not $exe -and -not $Check) {
        Aviso 'o winget do MariaDB instala o pacote completo (cliente + servidor); no instalador dá para desmarcar o serviço.'
        [void](Instalar-Winget 'MariaDB.Server' $null)
        $exe = Procurar-Mariadb
    }
    if (-not $exe) { Registrar 'mariadb' 'FALTA' "cliente mariadb não encontrado (winget install MariaDB.Server, série $($V.MARIADB_SERIE)+)"; return }
    $v = Versao-Mariadb $exe
    if (-not (Tem 'mariadb')) {
        if ($Check) { Registrar 'mariadb' 'AVISO' "$v em $exe, fora do PATH"; return }
        Adicionar-PathUsuario (Split-Path -Parent $exe)
    }
    if (Versao-Ge $v $V.MARIADB_CLIENTE_MIN) { Registrar 'mariadb' 'OK' "$v ($exe)" }
    else { Registrar 'mariadb' 'AVISO' "$v (abaixo do $($V.MARIADB_CLIENTE_MIN) documentado)" }
}

# ---------------------------------------------------------------------------
# Postman CLI
# ---------------------------------------------------------------------------
function Versao-Postman {
    if (-not (Tem 'postman')) { return $null }
    $s = (& postman --version 2>$null | Select-Object -First 1)
    if ($s -match '^v?(\d+\.\d+\.\d+)') { return $Matches[1] }
    return $null
}
function Etapa-Postman {
    Titulo 'Postman CLI'
    $v = Versao-Postman
    if ($v -and (Versao-Ge $v $V.POSTMAN_CLI_MIN)) { Registrar 'postman' 'OK' "$v ($(Caminho-De 'postman'))"; return }
    if ((Tem 'postman') -and -not $v) { Aviso "'$(Caminho-De 'postman')' não responde a 'postman --version' (é o app desktop?)" }
    if ($Check) { Registrar 'postman' 'FALTA' 'Postman CLI não encontrado'; return }
    if (-not $PostmanScript -and (Tem 'npm')) {
        Info 'npm install -g postman-cli (pacote oficial do Postman)'
        & npm install -g postman-cli | Out-Null
    }
    else {
        Info 'Instalador oficial do Postman CLI (https://dl-cli.pstmn.io/install/win64.ps1)'
        & powershell.exe -NoProfile -InputFormat None -ExecutionPolicy AllSigned -Command "[System.Net.ServicePointManager]::SecurityProtocol = 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://dl-cli.pstmn.io/install/win64.ps1'))"
    }
    Atualizar-PathSessao
    $v = Versao-Postman
    if ($v) { Registrar 'postman' 'OK' "$v (instalado)" } else { Registrar 'postman' 'FALTA' 'veja https://learning.postman.com/docs/postman-cli/postman-cli-installation/' }
}

# ---------------------------------------------------------------------------
# Docker (opcional, deploy): só confere
# ---------------------------------------------------------------------------
function Etapa-Docker {
    Titulo 'Docker (opcional, deploy)'
    if (-not (Tem 'docker')) { Registrar 'docker' 'AVISO' 'não encontrado (opcional; ver docs\deploy-servidor.md)'; return }
    $v = Extrair-Versao ((& docker --version 2>$null) | Select-Object -First 1)
    $c = (& docker compose version --short 2>$null | Select-Object -First 1)
    if ($LASTEXITCODE -eq 0 -and $c) { Registrar 'docker' 'OK' "$v, compose $c" } else { Registrar 'docker' 'AVISO' "$v, sem 'docker compose' (opcional)" }
}

# ---------------------------------------------------------------------------
# Dependências do projeto (-Deps)
# ---------------------------------------------------------------------------
function Etapa-Deps {
    Titulo 'Dependências do projeto (-Deps)'
    if ($Check) { Info '(-Check: pulado)'; return }
    $ok = $true
    $env:GOTOOLCHAIN = 'local'
    foreach ($m in 'backend\shared', 'backend\rotaperfumes-api') {
        Info "go mod download em $m"
        Push-Location (Join-Path $Raiz $m)
        & go mod download; if ($LASTEXITCODE -ne 0) { $ok = $false }
        Pop-Location
    }
    Remove-Item env:GOTOOLCHAIN -ErrorAction SilentlyContinue
    if (Tem 'make') { Info 'make frontend-deps'; & make -C $Raiz frontend-deps | Out-Host }
    else { Info 'npm install (frontend)'; Push-Location (Join-Path $Raiz 'frontend'); & npm install | Out-Host; Pop-Location }
    if ($LASTEXITCODE -ne 0) { $ok = $false }
    if ($ok) { Registrar 'deps' 'OK' 'go mod download + npm install' } else { Registrar 'deps' 'FALTA' 'veja os erros acima' }
}

# ---------------------------------------------------------------------------
$V = Ler-Versoes
$GoVersao = Versao-GoMod
Write-Host "RotaPerfume - setup do ambiente (Windows) - modo: $Modo"
Info "Repositório: $Raiz"
Info "PowerShell $($PSVersionTable.PSVersion)"
if (-not $TemWinget) { Aviso 'winget não encontrado: só o Go (zip oficial), govulncheck e Postman CLI (npm) podem ser instalados.' }

Etapa-Basicas
Etapa-Go
Etapa-Node
Etapa-Govulncheck
Etapa-Mariadb
Etapa-Postman
Etapa-Docker
if ($Deps) { Etapa-Deps }

Titulo "Resumo ($Modo)"
$faltou = $false
foreach ($r in $Resultados) {
    $cor = switch ($r.Status) { 'OK' { 'Green' } 'AVISO' { 'Yellow' } default { 'Red' } }
    if ($r.Status -eq 'FALTA') { $faltou = $true }
    Write-Host ('  {0,-14} ' -f $r.Nome) -NoNewline
    Write-Host ('{0,-5}' -f $r.Status) -ForegroundColor $cor -NoNewline
    Write-Host "  $($r.Detalhe)"
}
Write-Host ''
if ($faltou) {
    if ($Check) { Write-Host 'Há itens em FALTA. Rode sem -Check para instalar.' } else { Write-Host 'Há itens em FALTA: veja as mensagens acima.' }
}
if (-not $Check) { Write-Host 'Se o PATH mudou, abra um terminal novo antes de usar go/node/make.' }
if ($faltou) { exit 1 } else { exit 0 }
