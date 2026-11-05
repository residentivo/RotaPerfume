# TLS do banco (MariaDB em Docker): passo a passo

> Cards **SEC-29** e **SEC-44** (Lote 29, 2026-11-01). Reescrito no **DOC-08** (Lote 30, 2026-11-02) em torno do `deploy/mariadb-docker.sh`, que é o **único** caminho oficial do banco em produção (DEPLOY-06). Este roteiro é para **você** executar no **servidor de produção** (`ivo-inspiron-15-3530`, `192.168.168.106`). A equipe não tem acesso a ele.
>
> **Leia antes:** os comandos rodam **no HOST** (shell do servidor, via SSH ou terminal local), como root (`sudo`), **nunca dentro de um container**. O script nunca imprime senhas nem usa `set -x`; não cole o conteúdo de `*.senha` nem de `ca-key.pem` em chat, log ou ticket.
>
> **Primeira instalação da produção?** Siga a seção "Instalação inicial da produção (DEPLOY-07)" do [`docs/deploy-servidor.md`](deploy-servidor.md#11-instalação-inicial-da-produção-deploy-07). Ela usa os passos deste roteiro na ordem certa (CA nova com `--nova-ca`, seed, fingerprint e deploy).
>
> **Duas CAs, nunca misturadas (DEPLOY-07, 2026-11-04):** a CA de **produção** é gerada pelo `mariadb-docker.sh --nova-ca` (`CN=RotaPerfumes_MariaDB_CA`), fica no servidor em `/etc/mysql/ssl/ca.pem` e é a única em `deploy/db-ca.fingerprint`. A CA de **dev** é a do MariaDB 11.8 do CasaOS (porta 3307), em `deploy/ca-cert.pem` (`CN=RotaPerfumes_MySQL_CA`, fingerprint `ef31b656...`, não versionada): ela nunca vai para o `deploy/db-ca.fingerprint`, e a CA de produção nunca é copiada para `deploy/ca-cert.pem`.

## 0. Visão geral

| Item | Valor |
|---|---|
| Banco | MariaDB 11.4 LTS, container `rotaperfumes-mariadb`, imagem local `rotaperfumes-mariadb:11.4-<12 hex do digest>` construída sobre `mariadb:11.4.13@sha256:d1891ede…` (SEC-31) |
| Rede | rede Docker `rotaperfumes-db`, `172.30.42.0/24`: MariaDB `.2`, API `.10`, admin/import `.20` (dinâmicos só em `.128/25`) |
| Porta no host | **nenhuma** por padrão. Só com `--dev-ip IP`: `192.168.168.106:3306`, liberada **só** para esses IPs (chain `ROTAPERFUMES-DB` em `DOCKER-USER`) |
| TLS no servidor | `require_secure_transport = ON`, `tls_version = TLSv1.2,TLSv1.3`; cliente Go exige **TLS 1.3** (TST-12) |
| Contas | todas `IDENTIFIED VIA ed25519` + `REQUIRE SSL` (SEC-30/SEC-39); `root` só `localhost` (socket, dentro do container) |
| Certificados (host) | `/etc/mysql/ssl/` (`ca.pem` e `server-cert.pem` 644; `server-key.pem` 600, dono = uid do `mysql` da imagem), montados read-only no container |
| CA | própria, 10 anos, RSA 3072. **Chave fora do servidor** (mídia offline), lida com `--ca-key` |
| Certificado do servidor | 2 anos, SAN fixo `IP:192.168.168.106, DNS:rotaperfumes-mariadb, DNS:localhost, IP:127.0.0.1, DNS:<hostname>` (sem `host.docker.internal`) |
| Senhas | `/root/rotaperfumes-mariadb/*.senha` (700/600, só root): `root`, `rotaperfumes_app`, `rotaperfumes_admin`, `casaos` |

Quem confia na CA:

- **API** (container, uid 65532): `DB_TLS_CA=/etc/rotaperfumes/db-ca.pem`, montado do host a partir de `/etc/mysql/ssl/ca.pem`; `DB_HOST=rotaperfumes-mariadb`, `DB_TLS_SERVER_NAME=rotaperfumes-mariadb` (fixados no `docker-compose.yml`).
- **Jenkins**: estágio `CA do banco` (valida o arquivo do host contra `deploy/db-ca.fingerprint`, SEC-40, e contra o certificado servido) e `Importar dump` (cliente `mariadb --ssl-ca --ssl-verify-server-cert`).
- **Máquina do admin** (só as de `--dev-ip`): cópia da CA de produção em `deploy/ca-producao.pem` (`DB_TLS_CA` do `.env.producao`, seção 11). O `deploy/ca-cert.pem` é a CA de dev (CasaOS) e não confia neste servidor.

## 1. Opções do script usadas neste roteiro

```bash
sudo bash deploy/mariadb-docker.sh --help
```

| Opção | Uso |
|---|---|
| `--ca-key ARQ` | chave da CA existente, na mídia offline. **Lida de lá, nunca copiada.** Exigida quando o certificado do servidor precisa ser (re)emitido |
| `--nova-ca` | gera uma CA **nova** (primeira instalação ou troca de CA). Implica `--novo-cert`. Faz backup de `/etc/mysql/ssl` em `/etc/mysql/ssl.bak-AAAAMMDD-HHMMSS` |
| `--novo-cert` | reemite só o certificado do servidor (mesma CA; exige a chave) |
| `--dev-ip IP` | repetível; publica `192.168.168.106:3306` só para esses IPs (na prática, a máquina do admin) e cria `casaos@IP` |
| `--recriar` | remove e recria o container (dados ficam em `/var/lib/rotaperfumes/mariadb`) |
| `--imagem-base REF` | outra imagem base, formato `mariadb:<versão>@sha256:<64 hex>` |
| `--sem-read-only` | só se o container não subir com `--read-only` |
| `--casaos-native-motivo TEXTO` | saída de emergência: `casaos` com `mysql_native_password` (cliente sem ed25519), motivo registrado |

**Sempre repita a mesma lista de `--dev-ip`** em toda execução. Se a lista mudar, a porta publicada diverge do container e o script para pedindo `--recriar`, antes de mudar qualquer coisa. Contas `casaos` de IPs que saíram da lista são removidas (convergência).

## 2. Primeira instalação: CA e certificado do servidor

Escolha **um** caso:

| Caso | Situação | Comando |
|---|---|---|
| A | não há CA de produção (**primeira instalação**, decisão do DEPLOY-07) | `sudo bash deploy/mariadb-docker.sh --nova-ca --dev-ip <IP do admin>` (gera `CN=RotaPerfumes_MariaDB_CA`) |
| B | a CA de produção já existe em `/etc/mysql/ssl/ca.pem` e a chave dela está na mídia offline (reexecução, renovação) | `sudo bash deploy/mariadb-docker.sh --ca-key /media/<pendrive>/ca-key.pem [--dev-ip ... (a mesma lista)]` |

**Nunca** reaproveite a CA de dev (`deploy/ca-cert.pem`, CasaOS) em produção: ela não entra em `/etc/mysql/ssl/ca.pem` nem em `deploy/db-ca.fingerprint`.

`--dev-ip <IP>` é opcional: só para a máquina do admin que precisa acessar o banco de produção (seção 11). A porta 3306 do host precisa estar livre (o script aborta se outro serviço a ocupa).

O que o script faz com os certificados (etapa `[3/8]`):

1. Se o certificado atual for válido (CA confere, chave confere, **SAN exatamente** o da tabela da seção 0, mais de 30 dias de validade), ele é **reaproveitado** e nada muda.
2. Senão, emite um novo: chave RSA 3072, serial aleatório (sem `.srl` ao lado da chave da CA), 730 dias, `extendedKeyUsage = serverAuth`. Confere com `openssl verify` e confere o SAN antes de instalar.
3. Sem a chave da CA (nem `--ca-key`, nem `/root/rotaperfumes-ca/ca-key.pem`), o script **aborta** pedindo `--ca-key`. CA que vence antes de 730 dias também aborta (troque com `--nova-ca`).
4. Instala `ca.pem` e `server-cert.pem` com 644 e `server-key.pem` com 600, dono = uid/gid do `mysql` da imagem. Se esse uid existir no host (`getent passwd`), avisa: esse usuário do host leria a chave.

No fim, o script imprime o **fingerprint SHA-256 da CA** no formato do `deploy/db-ca.fingerprint` e os próximos passos.

Conferência do certificado no host:

```bash
sudo openssl x509 -in /etc/mysql/ssl/server-cert.pem -noout -subject -issuer -enddate -ext subjectAltName
sudo openssl verify -CAfile /etc/mysql/ssl/ca.pem /etc/mysql/ssl/server-cert.pem      # server-cert.pem: OK
openssl x509 -in /etc/mysql/ssl/ca.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f
```

O último valor precisa estar em `deploy/db-ca.fingerprint` (seção 6). Se não estiver, o estágio `CA do banco` do Jenkins recusa o deploy.

## 3. Chave da CA fora do servidor (mídia offline)

A chave da CA é a peça mais sensível: com ela qualquer um emite um certificado que a API aceita. Ela **não fica no servidor**.

- Caso B: a chave já está na mídia; o script só a lê (`--ca-key`) e nunca a copia.
- Caso A (`--nova-ca`): o script grava a chave nova em `/root/rotaperfumes-ca/ca-key.pem` (600) e, no fim, avisa para tirá-la de lá. Faça logo:

```bash
sudo cp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem
sudo cp /etc/mysql/ssl/ca.pem /root/rotaperfumes-ca/server-ext.cnf /media/<pendrive-cifrado>/
sudo cmp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem && \
  sudo shred -u /root/rotaperfumes-ca/ca-key.pem
```

Guarde uma segunda cópia (outro pendrive cifrado ou cofre de senhas). Sem a chave, renovar o certificado exige trocar a CA (seção 9.2). Enquanto `/root/rotaperfumes-ca/ca-key.pem` existir, toda execução do script repete o aviso.

## 4. Verificar

**Pela rede interna** (como a API e o Jenkins enxergam o banco), no host:

```bash
IMG='mariadb:11.4.13@sha256:d1891ede5a559f1086f55053bb8cc1569b0298338c4d72d71725a7f74f6ba620'   # = IMAGEM_BASE do script
docker run --rm --network rotaperfumes-db --cap-drop ALL \
  --mount type=bind,source=/etc/mysql/ssl/ca.pem,target=/ca.pem,readonly \
  --entrypoint openssl "$IMG" s_client -starttls mysql -connect rotaperfumes-mariadb:3306 \
  -CAfile /ca.pem -verify_return_error -verify_hostname rotaperfumes-mariadb </dev/null 2>/dev/null \
  | grep -E 'Verify return code|Cipher is'
# esperado: "Cipher is TLS_AES_..." (nunca "(NONE)") e "Verify return code: 0 (ok)"
```

> **DEPLOY-08:** `Verify return code: 0 (ok)` sozinho é enganoso. Se o `mariadbd` recusar o host de origem (erro 1130, `Host '...' is not allowed to connect`, enviado **antes** do greeting), o `s_client` não faz handshake e imprime `no peer certificate available`, `Cipher is (NONE)` e, mesmo assim, `Verify return code: 0 (ok)`. Exija as duas linhas. Pela rede interna, o MariaDB só aceita as origens com conta (`172.30.42.10`, `172.30.42.20`): sem `--ip 172.30.42.20` no `docker run` acima, o container pega um IP dinâmico (`172.30.42.128/25`) e recebe o erro 1130; acrescente `--ip 172.30.42.20` se isso acontecer (e não rode ao mesmo tempo que outro cliente com esse IP).

O próprio script já testa, na etapa `[8/8]`, uma conexão TLS verificada com `rotaperfumes_admin` a partir de `172.30.42.20` e **aborta** se uma conexão `--skip-ssl` for aceita.

**Contas, plugin e TLS exigido** (root pelo socket, dentro do container):

```bash
sudo docker exec rotaperfumes-mariadb mariadb --defaults-extra-file=/run/rotaperfumes/root.cnf -e "
  SELECT User, Host, JSON_VALUE(Priv,'$.plugin') AS plugin, JSON_VALUE(Priv,'$.ssl_type') AS ssl_type
  FROM mysql.global_priv WHERE User <> 'mariadb.sys' ORDER BY User, Host;
  SHOW GLOBAL VARIABLES WHERE Variable_name IN ('require_secure_transport','tls_version','have_ssl','ssl_cert');"
```

Esperado: `rotaperfumes_app@172.30.42.10` e `rotaperfumes_admin@172.30.42.20` com `ed25519` e `ssl_type = 1` (`REQUIRE SSL`); `casaos@<cada --dev-ip>` igual; `root@localhost`; `require_secure_transport = ON`.

**Da máquina do admin** (só com `--dev-ip`), com a cópia da CA de produção (seção 11):

```bash
openssl s_client -starttls mysql -connect 192.168.168.106:3306 -CAfile deploy/ca-producao.pem \
  -verify_return_error -verify_ip 192.168.168.106 </dev/null 2>/dev/null | grep -E 'Verify return code|Cipher is'
# esperado: "Cipher is TLS_AES_..." (nunca "(NONE)") e "Verify return code: 0 (ok)"
```

Só funciona de uma máquina em `--dev-ip`. Do próprio servidor o teste nunca faz handshake (a origem chega pelo DNAT sem conta e o `mariadbd` responde com o erro 1130); por isso o script, com `--dev-ip`, confere o certificado offline (`openssl verify -verify_ip`) e lê os primeiros bytes da porta publicada (greeting ou erro 1130).

De uma máquina **fora** da lista `--dev-ip`, a conexão na 3306 não pode abrir (o `DROP` da chain `ROTAPERFUMES-DB`): `timeout 5 bash -c '</dev/tcp/192.168.168.106/3306' && echo ABERTA || echo bloqueada`.

## 5. Permissão para a API e o Jenkins lerem a CA (no HOST)

A API roda em imagem distroless com **uid 65532**, e o estágio `CA do banco` valida o arquivo num container que roda com o mesmo uid. Esse uid não existe no host, então o arquivo precisa ser legível por **"outros"** (644) e os diretórios, atravessáveis (755). O script já aplica `644` no `ca.pem` e `755` em `/etc/mysql/ssl`; confira o diretório pai:

```bash
sudo chmod 755 /etc/mysql /etc/mysql/ssl
sudo chmod 644 /etc/mysql/ssl/ca.pem
sudo -u nobody cat /etc/mysql/ssl/ca.pem >/dev/null && echo "legivel por outros: OK"
```

Só o `ca.pem` (público) é montado na API. A `server-key.pem` continua 600.

## 6. Jenkins: caminho da CA e fingerprint aceito (SEC-40)

- **`DB_TLS_CA_HOST_PATH`** deixou de ser parâmetro do job: é constante no `environment` do `Jenkinsfile` (`/etc/mysql/ssl/ca.pem`, caminho do **host**, resolvido pelo daemon Docker; funciona com o Jenkins em container). `DB_TLS_SERVER_NAME` também saiu: o nome conferido é sempre `rotaperfumes-mariadb`.
- **`deploy/db-ca.fingerprint`** (versionado): SHA-256 do DER de cada certificado aceito no arquivo da CA, em hex minúsculo sem `:`, um por linha (linhas vazias e `#` são ignoradas; no máximo dois, só durante uma troca de CA). O `validar-ca-banco.sh` **falha** se algum certificado do arquivo do host não estiver na lista, mesmo que o resto passe. Isso impede que alguém troque a CA no servidor sem um commit revisado.
- Calcular o valor: `openssl x509 -in /etc/mysql/ssl/ca.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f` (o script imprime o mesmo valor no fim).
- Só a CA de produção entra na lista (DEPLOY-07). O teste `TestSEC45_FingerprintVersionadoConfereComCA` confere o formato (0 a 2 fingerprints) e **proíbe** o fingerprint da CA de dev (`ef31b656...`, `deploy/ca-cert.pem`); o `TestSEC40_ValidarCABanco_CADevRecusada` confere que o estágio `CA do banco` recusa a CA de dev. Na primeira instalação o arquivo tem só o cabeçalho, e o estágio falha com `deploy/db-ca.fingerprint sem fingerprint: rode mariadb-docker.sh --nova-ca e grave o CA_FP (DEPLOY-07)` até o commit do `CA_FP` (`docs/deploy-servidor.md` §11, passo 4).

**Secret file `rotaperfumes-api-env`** (modelo: `deploy/api.env.example`):

```
DB_HOST=rotaperfumes-mariadb
DB_PORT=3306
DB_NAME=rotaperfumes
DB_USUARIO=rotaperfumes_app
DB_SENHA=<conteúdo de /root/rotaperfumes-mariadb/rotaperfumes_app.senha>
```

`DB_HOST`, `DB_PORT`, `DB_TLS_CA` e `DB_TLS_SERVER_NAME` são fixados pelo `docker-compose.yml` (ganham do Secret file). **Nunca** coloque `DB_TLS_REQUIRED=false` no Secret file (SEC-42): ele liberaria conexão sem TLS.

**Credencial `rotaperfumes-db-admin`** (Username with password): usuário `rotaperfumes_admin`, senha = conteúdo de `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha`. Leia as senhas no terminal do servidor (`sudo cat ...`) e digite/cole direto no Jenkins.

## 7. Primeiro deploy e conferência

Pré-condições: container `rotaperfumes-mariadb` rodando e rede `rotaperfumes-db` criada (**antes** do primeiro deploy da `main` com o compose novo; sem a rede, o `docker compose up` falha com `network rotaperfumes-db declared as external, but could not be found`), fingerprint em `deploy/db-ca.fingerprint`, Secret file e credencial atualizados.

1. Leve as mudanças para a `main` (o `pollSCM` dispara o job em até ~2 min).
2. O estágio `CA do banco` deve listar os fingerprints aceitos, cada certificado com `fingerprint ...: na lista (deploy/db-ca.fingerprint)`, `s_client: Verify return code: 0 (ok)` e `CA do banco validada: /etc/mysql/ssl/ca.pem (sha256 ...)`.
3. No host, o log da API:
   ```bash
   docker logs rotaperfumes-api-1 2>&1 | grep -E 'TLS do banco|conexão MariaDB estabelecida'
   # [server] TLS do banco: ligado (ca=/etc/rotaperfumes/db-ca.pem, server_name=rotaperfumes-mariadb, min=TLS1.3)
   # [db] conexão MariaDB estabelecida (host=..., tls=ligado (custom))
   ```

Se o estágio `CA do banco` falhar, a mensagem diz o motivo (arquivo ilegível pelo uid 65532, bloco que não é certificado, CA vencida, fingerprint fora da lista, CA que não valida o certificado servido ou nome fora do SAN). Ele roda antes de parar a API ou importar: nada foi alterado.

## 8. Contas, ed25519 e `REQUIRE SSL`

Não há passo manual: a cada execução o script roda, pelo stdin (nunca no argv), `CREATE USER IF NOT EXISTS ... IDENTIFIED VIA ed25519 USING PASSWORD(...) REQUIRE SSL` e o `ALTER USER` igual, seguidos de `REVOKE ALL` + `GRANT` (converge plugin, senha, TLS e privilégios). Depois remove (`DROP USER`) toda conta `rotaperfumes_app`, `rotaperfumes_admin` ou `casaos` com host fora do conjunto desejado e avisa de contas desconhecidas. Termina com erro se alguma conta do projeto ficar sem `REQUIRE SSL`.

| Conta | Host | Privilégios | Uso |
|---|---|---|---|
| `rotaperfumes_app` | `172.30.42.10` | `SELECT, INSERT, UPDATE, DELETE ON rotaperfumes.*` | API (`DB_SENHA` do Secret file) |
| `rotaperfumes_admin` | `172.30.42.20` | `ALL ON rotaperfumes.*` | import do Jenkins (`rotaperfumes-db-admin`), `deploy/seed-producao.sh` e migrações |
| `casaos` | cada `--dev-ip` | `ALL ON rotaperfumes.*` (só esse banco, DEPLOY-07) | só se houver `--dev-ip`; **hoje não aplicável** (sem `--dev-ip`, não há host para ela; seção 11) |

**Cliente sem ed25519.** O cliente precisa do plugin `client_ed25519` (vem no cliente `mariadb` 11.x e no Connector/C; o MySQL Workbench e o cliente `mysql` do Oracle **não** têm). O driver Go (`go-sql-driver/mysql`) suporta ed25519. Só para `casaos`, existe a saída de emergência `--casaos-native-motivo "<motivo, 10 a 300 caracteres>"` (`mysql_native_password`, ainda com `REQUIRE SSL`; motivo em `/root/rotaperfumes-mariadb/casaos-native.motivo`). Volte ao ed25519 rodando de novo sem a opção.

## 9. Rotação

Antes de qualquer rotação, faça backup dos certificados (o `--novo-cert` sozinho não faz backup):

```bash
sudo cp -a /etc/mysql/ssl "/etc/mysql/ssl.bak-$(date +%Y%m%d-%H%M%S)"
```

### 9.1 Renovar só o certificado do servidor (mesma CA; a cada ~2 anos)

```bash
sudo bash deploy/mariadb-docker.sh --novo-cert --ca-key /media/<pendrive>/ca-key.pem [--dev-ip ... (a mesma lista de sempre)]
```

- O script emite o certificado novo, instala e recarrega **sem reiniciar** o banco (`FLUSH SSL`; a saída mostra `Certificado novo recarregado (FLUSH SSL).`). Conexões novas já usam o certificado novo.
- Sem `--novo-cert`, uma execução comum também reemite quando faltam menos de 30 dias ou o SAN mudou (e pede a chave).
- **Nada muda no Jenkins nem nas máquinas de dev:** a CA é a mesma, então o fingerprint e o sha256 do `ca.pem` também; a API não é recriada.
- Confira com a seção 4. O estágio `CA do banco` avisa a cada build quando falta menos de 30 dias para algum certificado do arquivo da CA vencer.

### 9.2 Trocar a CA (`--nova-ca`; a cada ~10 anos ou se a chave vazar)

O script grava em `/etc/mysql/ssl/ca.pem` **só a CA nova** e passa a servir o certificado novo na hora (`FLUSH SSL`). A API continua com a CA antiga montada (o bind fica preso ao arquivo anterior) até ser recriada pelo Jenkins, então **conexões novas da API falham** entre a troca e o deploy. Faça numa janela curta:

1. Backup (acima) e, se puder, avise os usuários.
2. `sudo bash deploy/mariadb-docker.sh --nova-ca [--dev-ip ...]`. O backup automático vai para `/etc/mysql/ssl.bak-*`. Anote o fingerprint impresso no fim.
3. No repositório, acrescente o fingerprint novo em `deploy/db-ca.fingerprint` (pode manter a linha antiga por enquanto; no máximo duas), commit e push na `main`. O job valida a CA nova, o sha256 muda, a label `rotaperfumes.db-ca-sha256` muda e a API é **recriada** com a CA nova.
4. Na máquina do admin, atualize **`deploy/ca-producao.pem`** (`scp <usuario>@192.168.168.106:/etc/mysql/ssl/ca.pem deploy/ca-producao.pem`). O `deploy/ca-cert.pem` (CA de dev, CasaOS) **não muda** numa troca de CA de produção.
5. Tire a chave nova do servidor (seção 3).
6. Num commit seguinte, remova a linha do fingerprint antigo.

Se o `ca.pem` do host for trocado à mão sem atualizar o `deploy/db-ca.fingerprint`, o estágio `CA do banco` barra o deploy (fingerprint fora da lista) e a API continua no ar com a CA anterior.

## 10. Rollback

- **Certificado renovado com problema (9.1):** restaure os arquivos e recarregue:
  ```bash
  B=/etc/mysql/ssl.bak-AAAAMMDD-HHMMSS
  sudo cp -a "$B/server-cert.pem" "$B/server-key.pem" /etc/mysql/ssl/
  sudo docker exec rotaperfumes-mariadb mariadb --defaults-extra-file=/run/rotaperfumes/root.cnf -e 'FLUSH SSL'
  ```
  (`cp -a` preserva o dono e o modo 600 da chave.) Rodar o script de novo sem `--novo-cert` não desfaz: ele reaproveita o certificado que estiver válido.
- **CA trocada com problema (9.2):** restaure `ca.pem`, `server-cert.pem` e `server-key.pem` do backup `/etc/mysql/ssl.bak-*`, rode o `FLUSH SSL` acima, garanta que o fingerprint antigo está em `deploy/db-ca.fingerprint` e rode o job (o sha256 volta e a API é recriada com a CA antiga). A chave antiga continua na mídia offline.
- **Imagem nova com problema (SEC-31):** `sudo bash deploy/mariadb-docker.sh --recriar --imagem-base '<string anterior>' [--dev-ip ...]` e reverta o commit do digest. Só volte **patch** dentro do 11.4 (o `MARIADB_AUTO_UPGRADE=1` atualiza o datadir para a versão nova; downgrade de versão menor não é suportado: restaure o backup).
- **Não há volta para "sem TLS":** a API sempre exige TLS verificado (o compose fixa `DB_TLS_CA`) e o servidor exige `require_secure_transport`. `DB_TLS_REQUIRED=false` não é rollback de produção.
- **Instalação inicial com problema (antes de haver dados reais):** o banco de produção nasce só com o seed; para refazê-lo, use o `deploy/seed-producao.sh --forcar` (`docs/deploy-servidor.md` §11, passo 6). Não há banco anterior para onde voltar.

## 11. DEV (CasaOS) e acesso do admin à produção (`--dev-ip`, `casaos`, `.env.producao`)

**Banco de desenvolvimento (oficial, DEPLOY-07):** o MariaDB 11.8 do CasaOS, no mesmo servidor, porta **3307**, com a CA de dev em `deploy/ca-cert.pem` (`CN=RotaPerfumes_MySQL_CA`, fingerprint `ef31b656...`; não versionada, `.gitignore`). É para ele que o `.env` padrão da raiz aponta (`DB_HOST=192.168.168.106`, `DB_PORT=3307`, `DB_TLS_CA=deploy/ca-cert.pem`, como no `.env.example`). Os dados dele são de desenvolvimento e não vão para a produção. Essa CA **não** muda quando a CA de produção é criada ou trocada.

> **Não aplicável hoje (2026-11-05).** Por decisão do usuário (2026-11-04), o banco de produção só responde localmente no servidor (container `rotaperfumes-api` e root), sem `--dev-ip` para a máquina do admin. Os itens 1 a 4 abaixo ficam como referência; para conferir a produção, use o passo 7 da §11 de `docs/deploy-servidor.md` (no servidor, como root). Ver também o passo 11 dessa seção.

**Acesso à produção, só para a máquina do admin.** A porta `192.168.168.106:3306` só é publicada com `--dev-ip`, e só para os IPs da lista (use IP fixo ou reserva no DHCP: a conta e o firewall são por IP). A máquina do admin usa arquivos **separados**, ambos no `.gitignore`: `deploy/ca-producao.pem` e `.env.producao`.

1. No servidor: `sudo bash deploy/mariadb-docker.sh --dev-ip <IP do admin>` (com `--recriar` se a lista mudou; repita sempre a mesma lista).
2. Na máquina do admin, copie a CA de produção (pública) e confira o fingerprint:
   ```bash
   scp <usuario>@192.168.168.106:/etc/mysql/ssl/ca.pem deploy/ca-producao.pem
   openssl x509 -in deploy/ca-producao.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f
   # precisa ser a linha de deploy/db-ca.fingerprint
   ```
   **Nunca** copie a CA de produção para `deploy/ca-cert.pem` (sobrescreveria a CA de dev).
3. `.env.producao` na raiz (modo 600):
   ```
   # ATENCAO: PRODUCAO (dados reais). Use só com ENV_ARQUIVO=.env.producao.
   DB_HOST=192.168.168.106
   DB_PORT=3306
   DB_NAME=rotaperfumes
   DB_USUARIO=casaos
   DB_SENHA=<conteúdo de /root/rotaperfumes-mariadb/casaos.senha>
   DB_TLS_CA=deploy/ca-producao.pem
   ```
   `DB_TLS_SERVER_NAME` fica vazio: `192.168.168.106` está no SAN. Com `DB_HOST` fora do loopback, `DB_TLS_CA` é obrigatório (SEC-42); **não** use `DB_TLS_REQUIRED=false`. O cliente `mariadb` precisa do `client_ed25519`.
4. Uso (na raiz do repositório; `<alvo>` é o nome de um alvo do Makefile, ex.: `make db-check-env ENV_ARQUIVO=.env.producao`): `make <alvo> ENV_ARQUIVO=.env.producao CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes`. O `ENV_ARQUIVO` (só na linha de comando) faz o `make` ler esse arquivo **no lugar** do `.env`. A guarda `db-guard-producao` recusa todo alvo que escreve no banco quando o destino é a produção (host e porta: `192.168.168.106`/`ivo-inspiron-15-3530` com `DB_PORT` vazio ou 3306, ou `rotaperfumes-mariadb`/`172.30.42.2` em qualquer porta; o CasaOS em `192.168.168.106:3307` passa), salvo o `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` na linha de comando. Para só conferir: `make db-check-env ENV_ARQUIVO=.env.producao`.

> **Aviso forte: dados reais.** Com o `.env.producao`, a conta `casaos` tem `ALL` no banco de produção. **Nunca** rode `make db-down`, `db-reset`, `db-rebuild`, `db-seed`, `db-import-*`, `db-seed-mes-atual`, `gen-hash`, `fix-hash` nem `reset-senhas-ativos` contra ele, mesmo com o `CONFIRMO_PRODUCAO`. Não use o `.env.producao` com `make dev-api`, `make test`, `go test` com `INTEGRATION=1` nem com as CLIs Go (a guarda não cobre esses casos). Testes de integração e recriação de banco só num banco de dev ou **descartável** (MariaDB 11.4 local em container, `DB_HOST=127.0.0.1`, ou o container temporário do `TestTST15_MariadbDocker_Real`).
