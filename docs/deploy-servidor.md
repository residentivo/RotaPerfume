# Deploy no servidor da LAN (`ivo-inspiron-15-3530`) com Docker + Jenkins

> Card **DEPLOY-01** (Lote 13, 2026-09-30); banco em Docker desde o Lote 30 (DEPLOY-06, DOC-08, 2026-11-02); instalação inicial da produção reescrita no **DEPLOY-07** (Lote 33, 2026-11-04). Arquivos envolvidos: `Jenkinsfile`, `deploy/docker-compose.yml`, `deploy/Caddyfile`, `deploy/Dockerfile.caddy`, `deploy/api.env.example`, `deploy/mariadb-docker.sh`, `deploy/seed-producao.sh`, `deploy/validar-ca-banco.sh`, `deploy/db-ca.fingerprint`, `deploy/dump-local.ps1`, `deploy/setup-servidor.sh`, `sql/ordem-ddl.txt`, `backend/Dockerfile`, `frontend/Dockerfile`. O antigo `deploy/mariadb-setup.sql` saiu do repositório (SEC-39).
>
> **Primeira instalação da produção?** Siga a **[seção 11 (Instalação inicial da produção, DEPLOY-07)](#11-instalação-inicial-da-produção-deploy-07)** **antes** do primeiro deploy da `main`: MariaDB 11.4 novo com CA própria, seed com o admin e os 42 vendedores inativos, fingerprint da CA commitado e credenciais do Jenkins. Sem a rede `rotaperfumes-db` e o container `rotaperfumes-mariadb`, o estágio `CA do banco` para o job (e o `docker compose up` falharia).
>
> **Dev x produção:** o banco de **desenvolvimento** é o MariaDB 11.8 do CasaOS, no mesmo servidor, porta 3307 (`192.168.168.106:3307`), com a CA de dev em `deploy/ca-cert.pem` (`.env` padrão). A **produção** é o container `rotaperfumes-mariadb` (porta 3306, CA de produção, fingerprint em `deploy/db-ca.fingerprint`). As duas CAs nunca se misturam.

## 0. Instalação automática (recomendado)

A instalação tem **duas partes**, nesta ordem:

1. **Banco:** `deploy/mariadb-docker.sh` (seção 3). É o **único** caminho oficial do banco em produção: cria a imagem, a CA e o certificado, a rede `rotaperfumes-db`, o container `rotaperfumes-mariadb` e as contas. TLS em detalhe: [`docs/Instal_SSL.md`](Instal_SSL.md).
2. **Aplicação e Jenkins:** `deploy/setup-servidor.sh`, que **não instala nem configura banco**. Ele confere que o container do banco está no ar, lê a senha do `rotaperfumes_app` gerada pelo passo 1 e faz o resto (Jenkins, firewall da 8443, dump, `api.env`, job, primeiro build).

As **duas credenciais do Jenkins** continuam sendo cadastradas **à mão**: o `setup-servidor.sh` não cria, não altera e não apaga credenciais. Ele pausa, mostra as instruções e depois só **confere** (por leitura) se elas existem.

**Antes:** faça commit e push dos arquivos de deploy para o `main`, gere o dump (seção 7.1), se houver dados a levar, e deixe o Jenkins com os plugins Pipeline, Git e Credentials Binding. O Jenkins pode ser **nativo** (serviço systemd `jenkins`) ou **em container** (DEPLOY-04; o da produção roda em container), com os requisitos da seção 2.

1. **No servidor, o banco** (uma vez; reexecutável):
   ```bash
   sudo bash deploy/mariadb-docker.sh --nova-ca                              # primeira instalação, sem CA
   # ou, com uma CA existente cuja chave está na mídia offline:
   sudo bash deploy/mariadb-docker.sh --ca-key /media/<pendrive>/ca-key.pem
   ```
   Acrescente `--dev-ip <IP>` só para a máquina do admin que precisa acessar o banco de produção (seção 3.3 e seção 11, passo 11). Numa instalação nova, o seed inicial é o `deploy/seed-producao.sh` (seção 11, passo 6), nunca o `make db-seed`.
2. **No Windows**, na raiz do projeto, copie o dump e o `.env` para o `/tmp` do servidor:
   ```powershell
   scp deploy\dumps\rotaperfumes-AAAAMMDD-HHMMSS.sql.gz .env SEU_USUARIO@ivo-inspiron-15-3530:/tmp/
   ```
3. **No servidor**, baixe e rode o script:
   ```bash
   curl -fsSLO https://raw.githubusercontent.com/residentivo/RotaPerfume/main/deploy/setup-servidor.sh
   sudo bash setup-servidor.sh --env /tmp/.env --dump /tmp/rotaperfumes-AAAAMMDD-HHMMSS.sql.gz \
     --turnstile-site-key SUA_SITE_KEY --remove-inputs
   ```
   Opções: `--site-host` (padrão `ivo-inspiron-15-3530`), `--site-ip` (`192.168.168.106`), `--https-port` (`8443`), `--jenkins-url` (`http://localhost:8888`), `--jenkins-container NOME` (padrão: detectado pela imagem `jenkins/*`; sem container = Jenkins nativo), `--db-container` (`rotaperfumes-mariadb`), `--db-segredos` (`/root/rotaperfumes-mariadb`), `--skip-build` e `--remove-inputs` (apaga o `.env` e o dump originais no final). As antigas `--apenas-banco` e `--skip-mysql` **saíram** (DEPLOY-06): o script recusa e aponta para o `mariadb-docker.sh`. Veja `bash setup-servidor.sh --help`.
4. O script executa 9 etapas e mostra o andamento como `[n/9]` (numeração contínua com Jenkins nativo ou em container):
   1. **Pré-checagens:** root via `sudo` (a partir do seu usuário), comandos (`docker`, `curl`, `openssl`, `gzip`, `install`, `getent`), `docker compose` ≥ 2.17, integridade do dump (`gzip -t`), Jenkins nativo ou em container e o **banco do `mariadb-docker.sh`**: container `rotaperfumes-mariadb` rodando, na rede `rotaperfumes-db`, e as senhas `rotaperfumes_app.senha`/`rotaperfumes_admin.senha` em `/root/rotaperfumes-mariadb`. Faltando algo, para e manda rodar o `mariadb-docker.sh`.
   2. **Jenkins com acesso ao Docker:** nativo, coloca o usuário `jenkins` no grupo `docker` (só reinicia o Jenkins se o grupo mudou); em container, só confere que ele alcança o Docker do host e tem o `docker compose` ≥ 2.17 (não altera o container).
   3. **Firewall (ufw):** só libera a `HTTPS_PORT` (8443) se o ufw estiver ativo; inativo, só avisa e sugere comandos (nunca ativa o ufw). O banco não passa pelo ufw: a porta do MariaDB só é publicada com `--dev-ip` e é restringida na chain `DOCKER-USER` (o ufw não enxerga portas publicadas pelo Docker).
   4. **Dump:** Jenkins nativo, instala em `/opt/rotaperfumes/dumps/rotaperfumes.sql.gz` (`root:jenkins`, 640). Em container, usa esse caminho se ele estiver num bind do Jenkins; senão copia para `/var/jenkins_home/rotaperfumes-dumps/` (dono do Jenkins, 600) e ajusta o `DUMP_PATH` do build.
   5. **`api.env`:** gera `~/rotaperfumes-api.env` (600) a partir do seu `.env`. Define `DB_HOST=rotaperfumes-mariadb`, `DB_PORT=3306`, `DB_NAME=rotaperfumes`, `DB_USUARIO=rotaperfumes_app`, `DB_SENHA` (lida de `/root/rotaperfumes-mariadb/rotaperfumes_app.senha`, nunca impressa), um `JWT_SECRET` novo (48 bytes aleatórios em base64), `CORS_ALLOWED_ORIGINS`, `TRUST_PROXY_HEADERS=true` e `COOKIE_SECURE=true` (SEC-21: sempre `true` no servidor). As demais chaves, inclusive `PASSWORD_PEPPER`, vêm do seu `.env` (o pepper precisa ser o que gerou os hashes do dump). Avisa (só o nome da chave) de valores com `$` sem aspas simples, de `TURNSTILE_SECRET_KEY` vazia e de secret de teste do Turnstile.
   6. **Job Jenkins:** pede seu usuário e um **API token** (Jenkins → seu usuário → Security → API Token), confere os plugins e cria ou atualiza o job `rotaperfumes-deploy`.
   7. **Pausa para as credenciais manuais.** Siga as instruções da tela:
      1. No Windows: `scp SEU_USUARIO@ivo-inspiron-15-3530:rotaperfumes-api.env .`
      2. Jenkins → Gerenciar Jenkins → Credentials → System → Global credentials → **Add Credentials**. Kind **Secret file**, ID `rotaperfumes-api-env`, arquivo `rotaperfumes-api.env`.
      3. **Add Credentials** de novo. Kind **Username with password**, ID `rotaperfumes-db-admin`, usuário `rotaperfumes_admin`, senha = conteúdo de `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha` (leia no servidor com `sudo cat`, sem colar em chat ou log).
      4. Apague o `rotaperfumes-api.env` do Windows.

      Tecle ENTER. O script confere as duas credenciais e, se estiverem cadastradas, apaga o `~/rotaperfumes-api.env` do servidor.
   8. **Primeiro build:** dispara com `IMPORTAR_DUMP=true` (quando há `--dump`) e mostra o log ao vivo. Se o build não terminar em `SUCCESS`, o script sai com erro.
   9. **Resumo:** URL de acesso, exporta a CA do Caddy para `~/rotaperfumes-caddy-root.crt` (instale no Windows como na seção 8), lembra do Turnstile e de apagar o dump.
5. **Reexecução:** os dois scripts podem ser rodados de novo. O `mariadb-docker.sh` não troca senhas (ficam nos arquivos). O `setup-servidor.sh` sempre gera um `api.env` com `JWT_SECRET` novo (derruba as sessões abertas): **atualize** a credencial no Jenkins (abra a credencial → **Update** → envie o arquivo novo). Ele só confere se as credenciais existem, não o conteúdo.
6. **Confira o `JWT_SECRET` do Secret file (SEC-18, Lote 24):** a API **não sobe** com `JWT_SECRET` menor que 32 bytes ou igual ao placeholder do `.env.example` (log `config: JWT_SECRET é obrigatório (mínimo de 32 bytes)`), nem com `JWT_TTL` menor ou igual a zero. O `api.env` gerado pelo script já atende. Se o Secret file foi criado antes do Lote 24 ou editado à mão, confira antes do próximo deploy (para gerar: `openssl rand -base64 48`). Confira também que `COOKIE_SECURE` não está `false` e que **não** há `DB_TLS_REQUIRED=false` (SEC-42).

As seções abaixo descrevem o mesmo processo **manualmente**, como alternativa ou para diagnóstico.

## 1. Visão geral

```
Navegador (LAN)                          Navegador (internet)
   │  https://ivo-inspiron-15-3530:8443       │ https://<dominio do tunnel>
   │                                          ▼
   │                                 cloudflared (no host)
   │                                          │ https://localhost:8443 (noTLSVerify)
   ▼                                          ▼
┌──────────── docker compose -p rotaperfumes ────────────┐
│  caddy (tls internal, 443 no container → 8443 no host) │
│     hosts: localhost, 127.0.0.1, SITE_HOST, SITE_IP     │
│            + curinga (qualquer Host, ex. o do tunnel)  │
│     ├── /api/health → api:8080/health                  │
│     ├── /api/*      → api:8080       (API Go)          │
│     └── resto       → frontend:3000  (Next standalone) │
└──────────────────────────┬─────────────────────────────┘
                           │ api 172.30.42.10 → rotaperfumes-mariadb:3306 (TLS 1.3, CA própria)
                           ▼
        rede Docker externa "rotaperfumes-db" (172.30.42.0/24)
        ┌─────────────────────────────────────────────────┐
        │ rotaperfumes-mariadb  172.30.42.2  (MariaDB 11.4)│
        │ cliente do import     172.30.42.20 (Jenkins)     │
        └─────────────────────────────────────────────────┘
        porta 192.168.168.106:3306 só com --dev-ip (chain ROTAPERFUMES-DB em DOCKER-USER)
```

- O **Jenkins** (`http://ivo-inspiron-15-3530:8888/`) clona `residentivo/RotaPerfume` (branch `main`) e roda o `Jenkinsfile`, com estes estágios: `Checkout` → `Testes Go` (vet + test dentro de `GO_IMAGE`, fixada por digest) → `Testes Frontend` (TST-18: `npm ci`, `npm run lint` e `npm test` do `frontend/` dentro de `NODE_IMAGE`, fixada por digest; código por stdin, sem bind mount nem segredos) → `Build` (imagens do compose) → `CA do banco` (valida a CA do banco no host contra `deploy/db-ca.fingerprint` e contra o certificado servido pela rede interna, seção 3.5) → `Importar dump` (só com `IMPORTAR_DUMP=true`) → `Deploy` (`up -d`) → `Smoke test` (`/api/health` e `/login` pelo Caddy, acessando direto `https://SITE_IP:HTTPS_PORT`, e `/api/health` no mesmo endereço com um Host qualquer, como faz o tunnel).
- A API e o frontend **não** publicam porta no host. Só o Caddy publica a `HTTPS_PORT` (8443), que atende a LAN, `https://localhost:8443` no próprio servidor e o **Cloudflare Tunnel**. O banco não publica porta, salvo para as máquinas de `--dev-ip`.
- **Cloudflare Tunnel:** no `cloudflared` do host, aponte o hostname público para a porta 8443 do servidor (hoje `https://192.168.168.106:8443`) com **No TLS Verify** ligado (`noTLSVerify: true`), porque o certificado é da CA interna do Caddy. Hosts desconhecidos caem no site curinga do Caddy. O Caddy só aceita o header `CF-Connecting-IP` (IP real do visitante) de conexões vindas do próprio servidor (o `SITE_IP` ou o gateway docker `172.16.0.0/12`). Cadastre também o domínio do tunnel no Turnstile.
- **HTTPS é obrigatório:** os cookies de autenticação são `Secure` (`COOKIE_SECURE=true`, o default; desde o SEC-21 a decisão não depende mais do `Host` da requisição). Por HTTP o login "funciona", mas a sessão não se mantém. (Pelo tunnel o navegador fala HTTPS com a Cloudflare, então funciona.)
- O frontend e a API ficam na **mesma origem**. O frontend chama `/api/...` com URL relativa (`PUBLIC_URL` padrão `/`, embutido no bundle **no build**), então o mesmo build serve a LAN e o domínio do tunnel.

### Parâmetros do job (definidos no `Jenkinsfile`)

| Parâmetro | Padrão | Uso |
|---|---|---|
| `SITE_HOST` | `ivo-inspiron-15-3530` | Hostname de acesso na LAN (Caddy) |
| `SITE_IP` | `192.168.168.106` | IP da LAN (o Caddy também aceita esse IP; usado no smoke test) |
| `HTTPS_PORT` | `8443` | Porta HTTPS publicada pelo Caddy |
| `TURNSTILE_SITE_KEY` | `0x4AAAAAAFACjc-LtM9NWlvP` | Site key **pública** do Turnstile, embutida no bundle |
| `IMPORTAR_DUMP` | `false` | Importa o dump no MariaDB do container `rotaperfumes-mariadb`. **Sobrescreve as tabelas** |
| `DUMP_PATH` | `/opt/rotaperfumes/dumps/rotaperfumes.sql.gz` | Caminho do dump visível para o agente Jenkins |

**Constantes do `environment` (não são parâmetros):** `DB_TLS_CA_HOST_PATH=/etc/mysql/ssl/ca.pem` (SEC-40; caminho absoluto no **host**), `DB_REDE=rotaperfumes-db`, `DB_HOST_INTERNO=rotaperfumes-mariadb`, `DB_HOST_PORT=3306` (porta interna do container), `DB_CLIENTE_IP=172.30.42.20`, `GO_IMAGE`, `NODE_IMAGE` e `DB_CLIENT_IMAGE` (fixadas por digest, SEC-31, seção 3.6). Os antigos parâmetros `DB_TLS_CA_HOST_PATH` e `DB_TLS_SERVER_NAME` saíram: o nome conferido no certificado é sempre `rotaperfumes-mariadb`.

### Credenciais Jenkins exigidas

| ID | Tipo | Conteúdo |
|---|---|---|
| `rotaperfumes-api-env` | Secret file | `.env` de produção da API (modelo: `deploy/api.env.example`), com `DB_USUARIO=rotaperfumes_app` e a senha de `/root/rotaperfumes-mariadb/rotaperfumes_app.senha` |
| `rotaperfumes-db-admin` | Username with password | `rotaperfumes_admin` + senha de `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha` (usada só com `IMPORTAR_DUMP=true`) |

---

## 2. Pré-requisitos no servidor

Rode no servidor (via SSH ou no terminal local dele):

1. Instale o Docker Engine e o plugin compose. O compose precisa ser **≥ 2.17** (checado pelo `setup-servidor.sh`). O build funciona com o builder clássico ou com BuildKit; a imagem do Caddy usa `deploy/Dockerfile.caddy`:
   ```bash
   docker --version
   docker compose version     # precisa ser v2.17 ou superior
   ```
   Para usar `--dev-ip` no `mariadb-docker.sh`, o Docker precisa gerenciar o firewall com **iptables** (chain `DOCKER-USER`; confira com `sudo iptables -S DOCKER-USER`) e o host precisa de `iptables`, `iptables-restore` e `systemctl`. Sem isso o script aborta com `--dev-ip` (fail closed); sem `--dev-ip` não há porta publicada e nada disso é necessário.
2. **Jenkins com acesso ao Docker do host:**
   - **Nativo:** coloque o usuário do Jenkins no grupo `docker` e **reinicie o Jenkins** (sem o restart, o processo continua sem o grupo novo):
     ```bash
     sudo usermod -aG docker jenkins
     sudo systemctl restart jenkins
     sudo -u jenkins docker ps          # deve listar sem "permission denied"
     ```
   - **Em container (produção):** monte o socket `/var/run/docker.sock`, instale na imagem o `docker-ce-cli` e o `docker-compose-plugin` (≥ 2.17) e dê ao usuário do Jenkins acesso ao socket. Confira com `docker exec <container_jenkins> docker ps` e `docker exec <container_jenkins> docker compose version`. Os caminhos montados pelo pipeline (`/etc/mysql/ssl/ca.pem`) são resolvidos pelo daemon do **host**: o container do Jenkins não precisa enxergá-los.
3. Confirme as ferramentas usadas pelo pipeline (no agente): `bash`, `curl`, `gzip`/`zcat`, `install` (coreutils) e `openssl`.
   ```bash
   which bash curl gzip zcat install openssl
   ```
4. No Jenkins, confirme que os plugins **Pipeline**, **Git** e **Credentials Binding** estão instalados (vêm nos "suggested plugins").
5. Confirme que a porta **8443** está livre:
   ```bash
   sudo ss -ltnp | grep ':8443' || echo "8443 livre"
   ```

---

## 3. Banco: MariaDB em Docker (`deploy/mariadb-docker.sh`)

O banco do projeto é o **MariaDB 11.4 LTS** (DB-03), em container criado **fora** do compose pelo `deploy/mariadb-docker.sh` (DEPLOY-06). O `setup-servidor.sh` não instala nem configura banco (sem drop-in, `sql_mode` ou `systemctl mysql` no host).

> **Driver Go:** a API continua usando o `go-sql-driver/mysql`, que é compatível com o MariaDB (mesmo protocolo) e suporta ed25519. Os pacotes `<ctx>mysql` mantêm o nome.

### 3.1 Instalar ou atualizar

```bash
sudo bash deploy/mariadb-docker.sh [--ca-key <mídia>/ca-key.pem | --nova-ca] [--dev-ip IP ...] [--recriar]
```

Etapas (`[n/8]`):

1. **Pré-requisitos:** root, `docker`/`openssl`/`ss`/`sha256sum`, formato da `--imagem-base`, IPs e caminhos; com `--dev-ip`, `iptables` e a chain `DOCKER-USER`. Se o container já existe com rede, IP, portas, `--read-only` ou usuário diferentes do desejado, **para** pedindo `--recriar` (nada foi mudado). Com `--dev-ip`, aborta se a porta estiver ocupada por outro serviço.
2. **Imagem** `rotaperfumes-mariadb:11.4-<12 hex do digest>`: `FROM` a imagem base fixada por digest (SEC-31) + `zz-rotaperfumes.cnf` (TLS obrigatório, `tls_version = TLSv1.2,TLSv1.3`, `plugin_load_add = auth_ed25519`, `local_infile = 0`, `sql_mode` estrito, `utf8mb4_unicode_ci` com `character_set_collations`, `skip_name_resolve`). A chave privada **não** entra na imagem.
3. **Certificados** em `/etc/mysql/ssl` (CA própria com a chave fora do servidor; ver [`docs/Instal_SSL.md`](Instal_SSL.md)).
4. **Senhas** em `/root/rotaperfumes-mariadb/` (700; arquivos 600, só root): `root.senha`, `rotaperfumes_app.senha`, `rotaperfumes_admin.senha` e, **só com `--dev-ip`**, `casaos.senha` (CHORE-17). Geradas uma vez (32 caracteres `[A-Za-z0-9]`); a reexecução **não** troca senha. Nada é impresso. Sem `--dev-ip` o `casaos.senha` não é gerado; se já existir (de uma execução anterior com `--dev-ip`), é **mantido** e o script avisa (`Sem --dev-ip: conta casaos removida; ... mantido`): a mesma senha volta a valer se o `--dev-ip` voltar. Para descartá-lo: `sudo shred -u /root/rotaperfumes-mariadb/casaos.senha`.
5. **Rede `rotaperfumes-db` e firewall** (seções 3.2 e 3.3).
6. **Container** `rotaperfumes-mariadb`: só na rede `rotaperfumes-db` com IP `172.30.42.2`, `--user` do `mysql` da imagem, `--cap-drop ALL`, `no-new-privileges`, `--read-only` (com `tmpfs` em `/run/mysqld` e `/tmp`; `--sem-read-only` só se não subir), `restart unless-stopped`, datadir em `/var/lib/rotaperfumes/mariadb`, `MARIADB_ROOT_HOST=localhost`, `MARIADB_AUTO_UPGRADE=1`, logs `json-file` 5 × 10 MB.
7. **Banco `rotaperfumes` e contas** (seção 3.4), convergidas a cada execução.
8. **Teste de conexão TLS:** cliente descartável em `172.30.42.20` conecta com `--ssl-verify-server-cert`; uma conexão `--skip-ssl` aceita **aborta** o script. Confere também que o certificado **em uso** pelo `mariadbd` é o de `/etc/mysql/ssl/server-cert.pem` (mesma validade). Com `--dev-ip` (DEPLOY-08): `openssl verify -verify_ip 192.168.168.106` do certificado e leitura dos primeiros bytes de `192.168.168.106:3306` (greeting do MariaDB ou erro 1130 = porta publicada servida pelo `mariadbd`; nada, timeout ou outra resposta **aborta**). Não usa `openssl s_client` na porta publicada: do próprio servidor a conexão chega ao `mariadbd` pelo DNAT com uma origem sem conta (gateway da rede ou IP do host), e o `mariadbd` responde com o erro 1130 antes do greeting, sem handshake TLS. O teste TLS real da porta publicada é feito na máquina do `--dev-ip` (o script imprime o comando; ver §11 passo 11). As contas `healthcheck@localhost`, `@127.0.0.1` e `@::1`, criadas pelo entrypoint da imagem oficial (só `USAGE`), são esperadas; o script só avisa se tiverem mais que `USAGE`.

No fim, imprime o fingerprint da CA (formato do `deploy/db-ca.fingerprint`), onde está cada senha e os próximos passos. Avisa se a chave da CA ainda estiver no servidor.

`ONLY_FULL_GROUP_BY` segue **desligado** (DB-04/DB-06, decisão de 2026-11-02): o MariaDB não casa `SELECT expr ... GROUP BY expr` com a lista do SELECT, e a série de vendas do dashboard seria recusada. Ligar depois de o backend agrupar pelo alias e de um teste de integração em banco descartável (seção 3.8).

### 3.2 Rede `rotaperfumes-db`

- `docker network create --subnet 172.30.42.0/24 --gateway 172.30.42.1 --ip-range 172.30.42.128/25`, idempotente (o script aborta se a rede já existir com outra subnet; `--rede-docker` muda a faixa).
- IPs fixos: **MariaDB `.2`**, **API `.10`** (`ipv4_address` no `docker-compose.yml`, rede declarada `external: true`), **admin/import `.20`** (cliente do estágio `Importar dump` e do teste do script). IPs dinâmicos só em `.128-.255`, para nada ocupar os fixos.
- A API fala com `rotaperfumes-mariadb:3306` (nome do container, que está no SAN); `DB_HOST`, `DB_PORT`, `DB_TLS_CA` e `DB_TLS_SERVER_NAME` são fixados no `environment` do compose e ganham do `api.env`.
- **A rede e o container precisam existir antes do `docker compose up`**: sem a rede externa o compose falha (`network rotaperfumes-db declared as external, but could not be found`).

### 3.3 Porta para as máquinas de dev (`--dev-ip`) e firewall `DOCKER-USER`

- Sem `--dev-ip`: **nenhuma** porta publicada e nenhuma conta `casaos`.
- Com `--dev-ip IP` (repetível, um IPv4 por máquina): publica `-p 192.168.168.106:3306:3306` (nunca `0.0.0.0`) e cria a chain `ROTAPERFUMES-DB` com `-s <IP>/32 -j RETURN` por máquina e `-j DROP` no fim, com um salto no topo de `DOCKER-USER` (`-p tcp -m conntrack --ctorigdst 192.168.168.106 --ctorigdstport 3306 --ctdir ORIGINAL -j ROTAPERFUMES-DB`). A recarga é atômica (`iptables-restore --noflush`).
- **`--ctdir ORIGINAL` é obrigatório (DEPLOY-08):** o tuple original (`--ctorigdst`/`--ctorigdstport`) é o mesmo nos pacotes de ida e de volta. Sem o `--ctdir ORIGINAL`, a resposta do container (origem `172.30.42.2`) também salta para a chain, não casa nenhum `-s <IP>/32` e cai no `DROP`: o SYN-ACK morre e a máquina liberada recebe timeout na 3306. A reexecução do script insere o salto novo **antes** de remover o antigo (sem `--ctdir`), então não há janela aberta.
- Persistência: `/usr/local/sbin/rotaperfumes-db-firewall` (root, 700) + unit `rotaperfumes-db-firewall.service` (`After=docker.service`, `PartOf=docker.service`, habilitada), que reaplica a regra depois de um reboot ou restart do Docker. Rodar sem `--dev-ip` depois de já ter usado a opção recarrega a chain só com o `DROP` (fail closed).
- O ufw **não** filtra portas publicadas pelo Docker; a proteção é a chain acima. Confira: `sudo iptables -S ROTAPERFUMES-DB` e `sudo iptables -S DOCKER-USER | grep ROTAPERFUMES-DB`, que deve mostrar **uma** linha, com `--ctdir ORIGINAL` (`-A DOCKER-USER -p tcp -m conntrack --ctorigdst 192.168.168.106 --ctorigdstport 3306 --ctdir ORIGINAL -j ROTAPERFUMES-DB`). Um salto sem `--ctdir` é de antes do DEPLOY-08 e bloqueia as respostas: rode o script de novo.
- **Repita sempre a mesma lista de `--dev-ip`.** Lista diferente = porta diferente do container = o script pede `--recriar`; IPs que saíram perdem a conta `casaos`.

### 3.4 Contas e senhas

Todas `IDENTIFIED VIA ed25519` + `REQUIRE SSL` (SEC-30/SEC-39), criadas e alteradas pelo stdin do cliente (nunca no argv):

| Conta | Host | Privilégios | Senha | Onde vai |
|---|---|---|---|---|
| `rotaperfumes_app` | `172.30.42.10` | `SELECT, INSERT, UPDATE, DELETE ON rotaperfumes.*` | `rotaperfumes_app.senha` | `DB_SENHA` do Secret file `rotaperfumes-api-env` |
| `rotaperfumes_admin` | `172.30.42.20` | `ALL ON rotaperfumes.*` | `rotaperfumes_admin.senha` | credencial `rotaperfumes-db-admin` |
| `casaos` | cada `--dev-ip` | `ALL ON rotaperfumes.*` (só esse banco, DEPLOY-07) | `casaos.senha` | `.env.producao` da máquina do admin (seção 11, passo 11) |
| `root` | `localhost` | (todos) | `root.senha` | só `docker exec` (socket) |

A cada execução: `CREATE USER IF NOT EXISTS` + `ALTER USER` (converge plugin, senha e TLS), `REVOKE ALL` + `GRANT` (converge privilégios), `DROP USER` de toda conta do projeto com host fora do desejado, remoção de contas anônimas e do banco `test`, e aviso de contas desconhecidas. Root no banco: `sudo docker exec -it rotaperfumes-mariadb mariadb --defaults-extra-file=/run/rotaperfumes/root.cnf`.

### 3.5 Como o pipeline usa a CA do banco (SEC-29/SEC-40)

A CA **não** fica no repositório nem passa pelo workspace do Jenkins. Ela fica **no host**, em `DB_TLS_CA_HOST_PATH` (constante `/etc/mysql/ssl/ca.pem`, modo 644; ver `docs/Instal_SSL.md`, passo 5), e quem a monta é o **daemon Docker do host**. Por isso funciona com o Jenkins em container. O que o repositório guarda é só o **fingerprint** aceito (`deploy/db-ca.fingerprint`, seção 3.7).

A cada build, o estágio **`CA do banco`**:

1. confere que `deploy/db-ca.fingerprint` existe e imprime os fingerprints aceitos;
2. roda `deploy/validar-ca-banco.sh` (enviado por stdin) num container descartável de `DB_CLIENT_IMAGE` (traz `bash` e `openssl`), como **uid 65532** (o da API distroless), na rede `rotaperfumes-db`, com o arquivo do host em `--mount type=bind,...,readonly` (caminho inexistente é erro, em vez de virar diretório). Se o uid 65532 não conseguir ler, a API também não conseguiria, e o job para;
3. o script recusa qualquer bloco PEM que não seja `CERTIFICATE` (uma chave privada não pode ser montada na API), separa **cada** certificado e valida todos: parse, `-checkend 0` (vencido = falha), aviso de 30 dias e de `CA:TRUE`, subject e fingerprint no log, e **todo** certificado precisa estar em `deploy/db-ca.fingerprint` (SEC-40; um fora da lista = falha);
4. **confere a CA contra o servidor**, antes de parar a API ou importar: `openssl s_client -starttls mysql -connect rotaperfumes-mariadb:3306 -CAfile <CA> -verify_return_error -verify_hostname rotaperfumes-mariadb`. O container de validação usa `--ip 172.30.42.20` (conta `rotaperfumes_admin`; de um IP sem conta o `mariadbd` responde com o erro 1130 antes do greeting) e exige handshake real (`Cipher is` ≠ `(NONE)`, certificado do peer presente) além do `Verify return code: 0 (ok)` (DEPLOY-08). Se a CA não validar o certificado servido hoje para esse nome, o job para e nada é alterado;
5. grava o sha256 do arquivo em `deploy/db-ca.sha256` (apagado no `post`).

No estágio **Deploy**, o pipeline exporta `DB_CA_SHA256` e o compose o grava na label `rotaperfumes.db-ca-sha256` do serviço `api`. O `docker compose up -d` não recria o container quando só o conteúdo de um arquivo montado muda (e o bind fica preso ao inode antigo quando o arquivo é substituído); com a label, CA nova significa container novo, já com a CA atual.

O estágio **`Importar dump`** roda o cliente `mariadb` de `DB_CLIENT_IMAGE` com `--network rotaperfumes-db --ip 172.30.42.20`, `-h rotaperfumes-mariadb -P 3306`, `--ssl-ca=/etc/rotaperfumes/db-ca.pem --ssl-verify-server-cert`, `--local-infile=0` e a senha por `MYSQL_PWD` (nunca no argv).

Na API, `DB_TLS_CA` definido liga o TLS verificado com **TLS 1.3 mínimo** (TST-12); arquivo ilegível ou sem certificado derruba a subida (`config: DB_TLS_CA inválido ...`), e CA errada, vencida ou nome fora do SAN derrubam a conexão (`x509: ...`); não há queda para texto claro. Sem `DB_TLS_CA` e com `DB_HOST` fora do loopback, a API não sobe (SEC-42, `ErrDBTLSObrigatorio`), salvo `DB_TLS_REQUIRED=false`, que **nunca** deve ir para produção. O log de subida mostra `[server] TLS do banco: ligado (ca=..., server_name=rotaperfumes-mariadb, min=TLS1.3)` e `[db] conexão MariaDB estabelecida (host=<DSN mascarado>, tls=ligado (custom))`.

**Rotação** (certificado e CA): `docs/Instal_SSL.md`, passo 9. Resumo: certificado novo com a mesma CA (`--novo-cert`, `FLUSH SSL`) não exige nada no pipeline; CA nova (`--nova-ca`) exige o fingerprint novo em `deploy/db-ca.fingerprint` e um deploy.

### 3.6 Atualizar a imagem do MariaDB (digest, SEC-31)

As imagens base são fixadas pelo digest do **índice multi-arch** (a tag sozinha pode ser republicada). A imagem do MariaDB aparece em **dois** lugares, com a **mesma string** (um teste confere): `IMAGEM_BASE` no `deploy/mariadb-docker.sh` e `DB_CLIENT_IMAGE` no `Jenkinsfile`. O `GO_IMAGE` e o `NODE_IMAGE` do `Jenkinsfile` seguem o mesmo formato (`golang:<patch>-alpine@sha256:...` com a versão do `go.mod`; `node:<patch>-alpine@sha256:...` com a versão do `frontend/.nvmrc`, conferida por teste). Para eles basta trocar a string no `Jenkinsfile` (`docker buildx imagetools inspect node:22.23-alpine --format '{{.Manifest.Digest}}'`) e rodar `make test`; os passos 2, 4 e 5 abaixo são só do MariaDB.

1. Veja se há digest novo: `make deps-audit-imagens` (só informa, nunca falha; usa `docker buildx imagetools` ou o registry do Docker Hub via `curl`). Ou, numa máquina com Docker:
   ```bash
   docker buildx imagetools inspect mariadb:11.4 --format '{{.Manifest.Digest}}'
   ```
   e anote a tag do patch (ex.: `11.4.13`).
2. Troque `mariadb:<patch>@sha256:<digest>` no `IMAGEM_BASE` do script **e** no `DB_CLIENT_IMAGE` do `Jenkinsfile`.
3. `make test` (os testes de `tests/deploy` conferem o formato e a igualdade das duas strings).
4. Backup do banco (seção 9, "Backup do banco").
5. Depois do merge, no servidor: `sudo bash deploy/mariadb-docker.sh --recriar [--dev-ip ... (a mesma lista)]`. O banco fica fora do ar alguns segundos; os dados ficam no datadir. Sem `--recriar`, o script só avisa que o container usa uma imagem diferente.

Só suba **patch** do 11.4 assim. Mudança de versão menor/maior é card próprio (backup, teste em banco descartável).

### 3.7 Pinagem da CA (`deploy/db-ca.fingerprint`, SEC-40)

- Conteúdo: SHA-256 do DER de cada certificado aceito no arquivo da CA do host, hex minúsculo sem `:`, um por linha (`#` e linhas vazias ignoradas). Normal: **um**; durante uma troca de CA, **dois** (o antigo sai num commit seguinte).
- Calcular: `openssl x509 -in /etc/mysql/ssl/ca.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f` (o `mariadb-docker.sh` imprime o mesmo valor no fim).
- Toda troca de CA no servidor exige um commit revisado desse arquivo; sem ele, o estágio `CA do banco` barra o deploy e a API continua com a CA anterior. Isso impede que um arquivo trocado no host (por engano ou por um atacante) passe a ser a CA confiada pela API.
- Só a CA de **produção** entra na lista (DEPLOY-07). O `deploy/ca-cert.pem` é a CA de **dev** (MariaDB do CasaOS, porta 3307, `CN=RotaPerfumes_MySQL_CA`, fingerprint `ef31b656...`) e **não** pode estar no arquivo: os testes (`TestSEC45_FingerprintVersionadoConfereComCA`, `TestDEPLOY07_CADevNuncaEmProducao`) garantem que ele não está, e o `mariadb-docker.sh` avisa se encontrar essa linha. Na primeira instalação o arquivo só tem o cabeçalho; sem nenhuma linha útil, o estágio `CA do banco` falha com `deploy/db-ca.fingerprint sem fingerprint: rode mariadb-docker.sh --nova-ca e grave o CA_FP (DEPLOY-07)`.

### 3.8 Regra: integração só em banco descartável

- `INTEGRATION=1`, `make db-down`, `make db-reset`, `make db-rebuild`, `make db-seed` e `make db-import-*` **apagam ou sobrescrevem dados**. Só rode contra um banco **descartável**: um MariaDB 11.4 local em container (`DB_HOST=127.0.0.1`, `.env` separado) ou o container temporário do `TestTST15_MariadbDocker_Real`.
- **Nunca** contra o banco de produção (conta `casaos` do `.env.producao`, que tem `ALL` em `rotaperfumes.*`, com dados reais).
- **Guarda `db-guard-producao` (DEPLOY-07):** todo alvo do `Makefile` que escreve no banco (`db-create`, `db-up`, `db-seed`, `db-down`, `db-reset`, `db-rebuild`, `db-fix-*`, `db-revert-*`, `db-import-*`, `db-seed-mes-atual`, `db-gerar-imagens-produtos`, `gen-hash`, `fix-hash`, `fix-admin`, `reset-senhas-ativos`, `test-integration`) depende dela. A guarda olha **host e porta**: é produção `DB_HOST` = `192.168.168.106` ou `ivo-inspiron-15-3530` (`PRODUCAO_HOSTS_LAN`) com `DB_PORT` vazio ou `3306` (a porta publicada com `--dev-ip`), ou `DB_HOST` = `rotaperfumes-mariadb` ou `172.30.42.2` (`PRODUCAO_HOSTS_REDE`, rede Docker) em qualquer porta. O MariaDB de dev do CasaOS, no mesmo host, porta `3307` (`192.168.168.106:3307`), **passa**. Em produção o alvo é recusado, salvo `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` **na linha de comando** (no `.env` ou no ambiente não vale). O seed de produção é o `deploy/seed-producao.sh`, nunca `make db-seed`.
- **SEC-46:** `db-down`, `db-reset`, `db-rebuild` e `db-seed` também dependem de `db-guard-sem-producao`, que os recusa na produção **mesmo com** `CONFIRMO_PRODUCAO` (sem válvula; recriar a produção é backup + `mariadb-docker.sh --recriar` + `seed-producao.sh` no servidor). E `db-down`/`db-reset`/`db-rebuild` dependem de `db-guard-drop`: com `DB_NAME=rotaperfumes` (sem diferenciar maiúsculas) e `DB_HOST` fora do loopback (ex.: o dev do CasaOS, `192.168.168.106:3307`), o `DROP DATABASE` exige `CONFIRMAR=<DB_NAME>` **na linha de comando** (ex.: `make db-reset CONFIRMAR=rotaperfumes`; no `.env` ou no ambiente não vale). Loopback ou outro nome de banco (ex.: `rotaperfumes_teste`) seguem sem pergunta.
- **SEC-50:** os testes Go de integração (helper `backend/shared/testkit/integracao`) recusam a produção **antes de conectar**, com a mesma regra de host e porta, também quando o `go test` com `INTEGRATION=1` é chamado direto. **SEC-53:** `make test`/`lint`/`build` e os alvos `npm` não recebem mais as variáveis do `.env` (só `make test-integration` passa as `DB_*` ao `go test`).
- Os agentes da equipe também seguem a regra: nenhum `make db-*` nem `INTEGRATION=1` com o `.env` atual.

---

## 4. Cloudflare Turnstile

1. No painel do Cloudflare (Turnstile → seu site), cadastre em **Hostnames** o hostname de acesso: `ivo-inspiron-15-3530`.
2. Anote a **Site Key**, que é pública e vai no parâmetro `TURNSTILE_SITE_KEY` do job, e a **Secret Key**, que vai em `TURNSTILE_SECRET_KEY` no Secret file.
3. **Acesse sempre pelo hostname**, e não pelo IP. O widget só valida nos hostnames cadastrados.
4. Se o PC cliente não resolver `ivo-inspiron-15-3530`, adicione esta linha em `C:\Windows\System32\drivers\etc\hosts` (como administrador):
   ```
   192.168.168.106   ivo-inspiron-15-3530
   ```
5. **Use a secret real.** Desde o SEC-37 (Lote 29), a API **não sobe** em produção (`COOKIE_SECURE=true`) com uma secret de **teste** da Cloudflare (`1x…`, `2x…`, `3x…` seguidas de zeros). O log mostra `config: TURNSTILE_SECRET_KEY é uma secret de teste da Cloudflare e não é aceita com COOKIE_SECURE=true ...`, porque com ela o captcha aceita qualquer token e não confere hostname nem action. Se o Cloudflare não aceitar o hostname da LAN e você **precisar** das chaves de teste (site `1x00000000000000000000AA`, secret `1x0000000000000000000000000000000AA`), existe uma válvula: `TURNSTILE_PERMITIR_SECRET_TESTE=true` no Secret file. **Aviso:** com ela o login fica **sem captcha** (só o rate limit protege contra força bruta), e a API loga a cada subida `[config] SEGURANCA: secret de teste do Turnstile em producao (TURNSTILE_PERMITIR_SECRET_TESTE=true): captcha desligado`. Use só nesta rede interna, sabendo do risco, e remova a válvula assim que tiver a secret real.

---

## 5. Credenciais no Jenkins

Caminho: **Gerenciar Jenkins → Credentials → System → Global credentials (unrestricted) → Add Credentials**.

### 5.1 Secret file `rotaperfumes-api-env`

1. Numa máquina de confiança, copie `deploy/api.env.example` para um arquivo temporário (ex.: `api.env`) **fora do repositório**.
2. Preencha:
   - `DB_USUARIO=rotaperfumes_app` e `DB_SENHA` = conteúdo de `/root/rotaperfumes-mariadb/rotaperfumes_app.senha` (leia no servidor com `sudo cat`). `DB_HOST`/`DB_PORT`/`DB_TLS_CA`/`DB_TLS_SERVER_NAME` são fixados pelo compose. **Não** inclua `DB_TLS_REQUIRED=false` (SEC-42).
   - `JWT_SECRET` = valor **novo**, diferente do ambiente local:
     ```bash
     openssl rand -base64 48
     ```
     Com menos de 32 bytes, ou com o placeholder do exemplo, a API não sobe (SEC-18, Lote 24).
   - `COOKIE_SECURE=true` (SEC-21, Lote 24; é o default se a chave faltar). Nunca `false` no servidor: com `false` os cookies de sessão saem sem `Secure`.
   - `TURNSTILE_SECRET_KEY` = secret key **real** do Turnstile (secret de teste impede a subida, SEC-37; ver seção 4, item 5).
   - `PASSWORD_PEPPER` = pepper das senhas (SEC-13). Use **o mesmo do `.env` que gerou os hashes do dump importado**: com outro valor, nenhuma senha Argon2id do dump funciona (hashes bcrypt antigos continuam valendo e migram no próximo login). Num banco novo, gere com `openssl rand -base64 48`. Sem ele, ou com menos de 32 bytes, a API não sobe. Depois de definido, **não troque**.
   - `SMTP_*` e `SECURITY_ALERT_EMAILS`, se for enviar e-mails. Se ficarem vazios, os e-mails vão só para o log.
   - Se mudar `SITE_HOST`/`HTTPS_PORT`, ajuste também o `CORS_ALLOWED_ORIGINS`.
   - Se algum valor tiver `$`, coloque-o entre aspas **simples**.
3. Em **Add Credentials**: Kind **Secret file**, envie o arquivo e informe o ID **`rotaperfumes-api-env`** (exatamente assim).
4. **Apague** o arquivo preenchido da máquina local.

O pipeline grava esse arquivo em `deploy/api.env` (modo 600) só durante o deploy e o apaga no final (`post { always }`).

### 5.2 Username with password `rotaperfumes-db-admin`

1. Kind **Username with password**.
2. Username `rotaperfumes_admin`, Password = conteúdo de `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha`.
3. ID **`rotaperfumes-db-admin`**.

### 5.3 Credencial do GitHub (só se o repositório for privado)

Kind **Username with password**: o usuário do GitHub e um **Personal Access Token** (escopo de leitura do repositório) como senha. O ID é livre (ex.: `github-residentivo`).

---

## 6. Criar o job

1. **Novo item** → nome `rotaperfumes-deploy` → tipo **Pipeline** → OK.
2. Seção **Pipeline**:
   - Definition: **Pipeline script from SCM**
   - SCM: **Git**
   - Repository URL: `https://github.com/residentivo/RotaPerfume.git`
   - Credentials: a do passo 5.3 (se o repositório for privado)
   - Branch Specifier: `*/main`
   - Script Path: `Jenkinsfile`
3. Salve.
4. Antes de rodar, faça **commit e push** dos arquivos de deploy para o `main`. O Jenkins só enxerga o que está no GitHub.

> Na **primeira** execução, o Jenkins ainda não conhece os parâmetros: o botão é "Construir agora" e ele roda com os valores padrão (sem Turnstile e sem importar o dump). Pode deixar rodar ou abortar. Depois dela, aparece **"Construir com parâmetros"**.

---

## 7. Dump de um banco MariaDB de origem e primeira carga

Use esta seção só se for preciso levar dados de **outro MariaDB** para o servidor. A instalação inicial da produção (seção 11) **não** importa dump: o banco nasce com o seed do `deploy/seed-producao.sh`, e os dados do MariaDB do CasaOS são de desenvolvimento. Importar um dump **sobrescreve** as tabelas de produção (dados reais): faça backup antes (seção 9).

1. **No Windows**, na raiz do projeto:
   ```powershell
   powershell -ExecutionPolicy Bypass -File deploy\dump-local.ps1
   ```
   O script lê `DB_*` e `DB_TLS_CA` do `.env` da raiz e roda **só o `mariadb-dump`** (SEC-41; sem fallback; procura `-DumpExe`, o `PATH` e `C:\Program Files\MariaDB *\bin`, e confere que o `--version` diz MariaDB). Com `DB_TLS_CA`, usa `--ssl-ca=<caminho a partir da raiz> --ssl-verify-server-cert`; sem `DB_TLS_CA`, só aceita `DB_HOST` loopback (`localhost`, `127.x`, `::1`), sem TLS e com aviso, e **aborta** para host remoto. A senha vai por `MYSQL_PWD`. Grava `deploy\dumps\rotaperfumes-AAAAMMDD-HHMMSS.sql.gz` (a pasta é ignorada pelo git) e apaga o `.sql` intermediário.
2. **No servidor**, prepare a pasta. O Jenkins precisa conseguir **ler** o arquivo (com o Jenkins em container, o `setup-servidor.sh` cuida disso; ver seção 0, etapa 4):
   ```bash
   sudo mkdir -p /opt/rotaperfumes/dumps
   sudo chown "$USER":jenkins /opt/rotaperfumes/dumps
   sudo chmod 750 /opt/rotaperfumes/dumps
   ```
3. **No Windows**, copie o dump com scp:
   ```powershell
   scp deploy\dumps\rotaperfumes-AAAAMMDD-HHMMSS.sql.gz SEU_USUARIO@192.168.168.106:/opt/rotaperfumes/dumps/rotaperfumes.sql.gz
   ```
   Depois, no servidor:
   ```bash
   chmod 640 /opt/rotaperfumes/dumps/rotaperfumes.sql.gz
   ```
4. No Jenkins, **Construir com parâmetros**:
   - `TURNSTILE_SITE_KEY` = site key do passo 4
   - `IMPORTAR_DUMP` = **marcado**
   - demais parâmetros no padrão
5. O estágio **Importar dump**:
   - valida o gzip;
   - para o container da API;
   - confere que o estágio `CA do banco` validou a CA;
   - importa com o cliente `mariadb` de `DB_CLIENT_IMAGE`, pela rede `rotaperfumes-db` com IP `172.30.42.20`, em `rotaperfumes-mariadb:3306`, com `--local-infile=0 --ssl-ca=... --ssl-verify-server-cert` e o usuário da credencial `rotaperfumes-db-admin`;
   - roda `TRUNCATE TABLE refresh_tokens`, porque as sessões do ambiente de origem não valem no servidor.

   Em seguida vêm `Deploy` e `Smoke test`.
6. Com o build verde, **apague o dump** do servidor e da máquina local:
   ```bash
   rm /opt/rotaperfumes/dumps/rotaperfumes.sql.gz
   ```
   ```powershell
   Remove-Item deploy\dumps\*.sql.gz
   ```
7. Nos próximos deploys, deixe `IMPORTAR_DUMP` **desmarcado**. Se marcar de novo, o dump **sobrescreve** os dados do servidor.

---

## 8. Primeiro acesso e certificado (CA do Caddy)

O Caddy usa `tls internal`, ou seja, uma CA própria guardada no volume `caddy_data`. Até a raiz dessa CA ser instalada no cliente, o navegador mostra um aviso de certificado.

1. **No servidor**, exporte a raiz da CA:
   ```bash
   docker cp rotaperfumes-caddy-1:/data/caddy/pki/authorities/local/root.crt ./rotaperfumes-caddy-root.crt
   ```
   Alternativa com compose, rodada dentro de `deploy/` no workspace do job (ex.: `/var/lib/jenkins/workspace/rotaperfumes-deploy/deploy`):
   ```bash
   API_ENV_FILE=/dev/null docker compose -p rotaperfumes cp caddy:/data/caddy/pki/authorities/local/root.crt .
   ```
2. Copie o `.crt` para o PC Windows. Por exemplo, com o scp rodado **no Windows**:
   ```powershell
   scp SEU_USUARIO@192.168.168.106:~/rotaperfumes-caddy-root.crt .
   ```
3. **No Windows**: clique duas vezes no `.crt` → **Instalar Certificado** → **Máquina Local** (ou Usuário Atual) → **Colocar todos os certificados no repositório a seguir** → **Autoridades de Certificação Raiz Confiáveis** → Concluir.
   - O Firefox usa um repositório próprio. Importe em Configurações → Privacidade e Segurança → Certificados → Autoridades, ou ative `security.enterprise_roots.enabled`.
4. Feche e reabra o navegador e acesse **https://ivo-inspiron-15-3530:8443**. O cadeado deve aparecer sem aviso.
5. Faça login com um usuário do dump.

> A CA vive no volume `caddy_data`. **Não** rode `docker compose down -v` nem `docker volume rm rotaperfumes_caddy_data`: isso gera uma CA nova e obriga a reinstalar o certificado em todos os clientes.

---

## 9. Operações do dia a dia

Os comandos manuais rodam **no servidor**, dentro de `deploy/` no workspace do job, **sempre** com `API_ENV_FILE=/dev/null`. O `api.env` só existe durante o build e, sem essa variável, o compose falha de propósito.

```bash
cd /var/lib/jenkins/workspace/rotaperfumes-deploy/deploy
```

| Ação | Comando |
|---|---|
| Status | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes ps` |
| Logs (todos, seguindo) | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes logs -f --tail 100` |
| Logs da API | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes logs -f --tail 200 api` |
| Logs sem compose | `docker logs -f --tail 200 rotaperfumes-api-1` (ou `-frontend-1`, `-caddy-1`) |
| Logs do banco | `docker logs -f --tail 200 rotaperfumes-mariadb` |
| Parar (mantém containers) | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes stop` |
| Religar containers parados | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes start` |
| Remover (mantém o volume da CA) | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes down` |

- **Nunca** rode `up` manualmente com `API_ENV_FILE=/dev/null`, porque a API seria recriada sem as variáveis. Para subir ou recriar, **rode o job**.
- O banco **não** faz parte do compose: `down` não o afeta. Para recriá-lo, use `sudo bash deploy/mariadb-docker.sh --recriar [--dev-ip ...]`.
- Os containers têm `restart: unless-stopped` e voltam sozinhos após um reboot do servidor. A exceção é se tiverem sido parados com `stop`. A unit `rotaperfumes-db-firewall` reaplica a regra do banco depois do Docker.
- Os logs rodam em `json-file` com até 5 × 10 MB por serviço.

### Backup do banco

No servidor, pela rede interna, com o cliente da imagem fixada (a mesma string de `IMAGEM_BASE`/`DB_CLIENT_IMAGE`) e a conta admin:

```bash
sudo -i
umask 077
IMG='mariadb:11.4.13@sha256:d1891ede5a559f1086f55053bb8cc1569b0298338c4d72d71725a7f74f6ba620'
MYSQL_PWD=$(cat /root/rotaperfumes-mariadb/rotaperfumes_admin.senha) \
docker run --rm -i -e MYSQL_PWD --network rotaperfumes-db --ip 172.30.42.20 \
  --cap-drop ALL --security-opt no-new-privileges:true \
  --mount type=bind,source=/etc/mysql/ssl/ca.pem,target=/ca.pem,readonly \
  --entrypoint mariadb-dump "$IMG" -h rotaperfumes-mariadb -P 3306 -u rotaperfumes_admin \
  --ssl-ca=/ca.pem --ssl-verify-server-cert --single-transaction --routines --triggers \
  --hex-blob --default-character-set=utf8mb4 rotaperfumes \
  | gzip > "/root/backup-rotaperfumes-$(date +%Y%m%d-%H%M%S).sql.gz"
```

O arquivo fica em `/root` com modo 600 (tem os hashes de senha). Copie para mídia offline e apague os antigos.

### Atualizar

1. Faça commit e push no `main`.
2. Em até ~2 minutos o Jenkins detecta o commit novo (`pollSCM('H/2 * * * *')` no `Jenkinsfile`) e roda o job sozinho, com os valores padrão dos parâmetros (`IMPORTAR_DUMP` desmarcado, `TURNSTILE_SITE_KEY` já preenchida). Para rodar na hora, use **Construir com parâmetros**.
3. O pipeline testa, reconstrói as imagens, valida a CA do banco, recria os containers e roda o smoke test.

O gatilho só passa a valer depois que o job roda uma vez com o `Jenkinsfile` que contém o bloco `triggers`. Confira em `rotaperfumes-deploy` → **Configurar** que "Consultar periodicamente o SCM" aparece marcado.

Para mudar só segredos (ex.: senha SMTP): atualize a credencial `rotaperfumes-api-env` no Jenkins e rode o job. Auditoria de dependências e de digests: `make deps-audit` mensalmente e antes de cada deploy (`CLAUDE.md`).

### Rollback

As imagens da aplicação são sempre `:latest`, sem tag por versão. Para voltar:

1. No repositório, `git revert <commit-problemático>` e push no `main`. Evite reescrever o histórico.
2. Rode o job de novo.
3. Se o problema for de dados, reimporte um dump bom (seção 7) com `IMPORTAR_DUMP=true`. O banco não é versionado pelo pipeline: faça backup (acima) antes de importar.
4. Banco (imagem, certificado ou CA): `docs/Instal_SSL.md`, passo 10.

---

## 10. Troubleshooting

| Sintoma | Causa provável | Como resolver |
|---|---|---|
| Login aceita a senha, mas volta para a tela de login ou perde a sessão ao recarregar | Acesso por HTTP, ou certificado não confiável (cookie `Secure` descartado) | Acesse **https://ivo-inspiron-15-3530:8443** e instale a CA do Caddy (seção 8) |
| Aviso "Sua conexão não é particular" | CA do Caddy não instalada no cliente, ou CA regenerada (volume apagado) | Reexporte e reinstale o `root.crt` (seção 8) |
| Widget do Turnstile não aparece ou dá erro, ou o login recusa o captcha | Hostname não cadastrado no Cloudflare; acesso pelo IP; site key vazia/errada no build; secret errada no `api.env` | Cadastre o hostname, acesse pelo hostname e rode o job com `TURNSTILE_SITE_KEY` correta (ela é embutida no **build**). Confira `TURNSTILE_SECRET_KEY` na credencial |
| Frontend chama URL errada da API | `PUBLIC_URL` definido no ambiente do build (o padrão `/` usa a mesma origem) | Remova `PUBLIC_URL` e rode o job de novo |
| Site não abre pelo Cloudflare Tunnel | `cloudflared` com origem errada ou validando o certificado | Origem na porta 8443 do servidor com `noTLSVerify: true`; teste no servidor com `curl -k https://192.168.168.106:8443/api/health` |
| Smoke test: `tunnel=000` com `/api/health=200` usando `localhost` | O Jenkins roda em container: o `localhost` dele é o próprio container, não o servidor | O smoke test testa o site curinga pelo `SITE_IP` com um `Host` qualquer, como o `cloudflared` faz |
| `docker compose up` falha com `network rotaperfumes-db declared as external, but could not be found` | O banco ainda não foi instalado (ou a rede foi apagada) | `sudo bash deploy/mariadb-docker.sh ...` (seção 3) antes do deploy |
| `compose up` falha com `Address already in use` para `172.30.42.10` | Outro container ocupou o IP fixo da API (rede criada sem `--ip-range`) | `docker network inspect rotaperfumes-db`; tire o intruso. O script avisa quando a rede não tem `--ip-range 172.30.42.128/25` |
| API não conecta no banco (`connection refused`, `no such host` ou timeout) | Container `rotaperfumes-mariadb` parado ou fora da rede `rotaperfumes-db` | `docker ps --filter name=rotaperfumes-mariadb`; `docker logs rotaperfumes-mariadb`; rode o script (com `--recriar` se ele pedir) |
| API: `Access denied for user 'rotaperfumes_app'` | `DB_SENHA` do Secret file diferente de `rotaperfumes_app.senha`, ou API fora do IP `172.30.42.10` | Confira a credencial `rotaperfumes-api-env` e o `ipv4_address` do compose |
| API não sobe: `config: DB_TLS_CA é obrigatório quando DB_HOST não é loopback ...` | `DB_TLS_CA` vazio com host remoto (SEC-42) | Defina `DB_TLS_CA` (o compose de produção já define). Não use `DB_TLS_REQUIRED=false` |
| `openssl s_client` com `no peer certificate available`, `Cipher is (NONE)` e `Verify return code: 0 (ok)` (rc 1) | Não houve handshake TLS: o `mariadbd` recusou o host de origem (erro 1130 antes do greeting). Do próprio servidor isso sempre acontece na porta publicada (DNAT, origem sem conta); pela rede interna, quando o cliente não usa `--ip 172.30.42.20` | Teste a porta publicada **da máquina do `--dev-ip`** (§11 passo 11) exigindo `Cipher is` ≠ `(NONE)`; pela rede interna, `docker run ... --ip 172.30.42.20`. O `mariadb-docker.sh` não usa mais o `s_client` na porta publicada (DEPLOY-08) |
| Estágio **CA do banco**: "bind source path does not exist" ou "nao legivel pelo uid 65532" | `/etc/mysql/ssl/ca.pem` ausente, ou arquivo/diretório sem permissão para "outros" | `docs/Instal_SSL.md` passo 5 (`chmod 644`/`755` **no host**) |
| Estágio **CA do banco**: "deploy/db-ca.fingerprint sem fingerprint: rode mariadb-docker.sh --nova-ca e grave o CA_FP (DEPLOY-07)" | Primeira instalação: o arquivo só tem o cabeçalho | Seção 11, passos 2 e 4 (rodar o script, commitar o `CA_FP` como única linha útil) |
| Estágio **CA do banco**: "fora de deploy/db-ca.fingerprint" | CA trocada no servidor sem commit do fingerprint (ou arquivo adulterado) | Confira a CA do host; se a troca foi legítima, commit do fingerprint novo (seção 3.7) |
| `mariadb-docker.sh` avisa "contem o fingerprint da CA de DEV" | A linha `ef31b656...` (CA de dev, `deploy/ca-cert.pem`) está em `deploy/db-ca.fingerprint` | Remova essa linha e commite: a CA de dev nunca vale em produção (seção 3.7) |
| `make db-*`: "DEPLOY-07: DB_HOST=... DB_PORT=... e o banco de PRODUCAO: alvo recusado" | Guarda `db-guard-producao`: host do servidor com `DB_PORT` vazio/3306, ou host da rede Docker (o dev do CasaOS é a porta 3307) | Confira se era mesmo para a produção. Se sim (e o alvo não apaga dados), repita com `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` na linha de comando (seção 3.8) |
| `make db-down/db-reset/db-rebuild/db-seed`: "SEC-46: ... e PRODUCAO: ... sao recusados mesmo com CONFIRMO_PRODUCAO" | Guarda `db-guard-sem-producao`: esses alvos nunca rodam na produção | Na produção, use backup (seção 9), `sudo bash deploy/mariadb-docker.sh --recriar` e `sudo bash deploy/seed-producao.sh` no servidor (seção 11) |
| `make db-down/db-reset/db-rebuild`: "SEC-46: DROP DATABASE rotaperfumes em ... (fora do loopback): repita com CONFIRMAR=rotaperfumes" | Guarda `db-guard-drop`: `DROP DATABASE rotaperfumes` fora do loopback (ex.: CasaOS :3307) | Confira o `DB_HOST`/`DB_PORT`. Se for mesmo para apagar esse banco, repita com `CONFIRMAR=rotaperfumes` na linha de comando |
| `seed-producao.sh`: "sem terminal para pedir o PASSWORD_PEPPER: use --pepper-file ARQ" ou "--forcar exige um terminal para a confirmacao" | Script rodado sem terminal de controle (`ssh` sem `-t`, cron, pipe); falha na etapa 1 (BUG-26) | Nada foi alterado. Rode de um terminal interativo ou use `--pepper-file` (o `--forcar` sempre exige terminal) |
| `seed-producao.sh`: "a tabela usuarios ja tem N linha(s)" | O banco já foi semeado | Nada foi alterado. Não rode de novo; `--forcar` só numa instalação ainda sem uso (inativa todos os não-admin e troca a senha do admin) |
| Estágio **CA do banco**: "nao e certificado", "invalido", "VENCIDO" ou "NAO valida o certificado servido" | Arquivo com chave, CA vencida, CA que não assinou o certificado em uso ou banco fora do ar | `docs/Instal_SSL.md` passos 2, 4 e 9 |
| API: `x509: certificate signed by unknown authority` | A API ainda monta a CA antiga depois de uma troca de CA (`--nova-ca`) | Rode o job (a label do sha256 recria a API) depois do commit do fingerprint (`docs/Instal_SSL.md` passo 9.2) |
| API: `x509: certificate is valid for ..., not rotaperfumes-mariadb` | Certificado do servidor emitido fora do script ou com SAN antigo | `sudo bash deploy/mariadb-docker.sh --novo-cert --ca-key ...` |
| Estágio **Importar dump** falha com "Dump nao encontrado/legivel" | Arquivo fora do `DUMP_PATH` ou sem permissão para o usuário do Jenkins | Seção 7.2/7.3 (`chown`/`chmod`), ou o `DUMP_PATH` informado pelo `setup-servidor.sh` (Jenkins em container) |
| Import falha com `Access denied` | Credencial `rotaperfumes-db-admin` com senha diferente de `rotaperfumes_admin.senha` | Atualize a credencial (seção 5.2) |
| `make db-* ENV_ARQUIVO=.env.producao` da máquina do admin: `Access denied for user 'casaos'@'<IP>'` ou timeout | IP da máquina fora da lista `--dev-ip` (ou mudou por DHCP) | Rode o script com o `--dev-ip` certo (e `--recriar` se a lista mudou); fixe o IP |
| Cliente: `Authentication plugin 'client_ed25519' cannot be loaded` | Cliente sem ed25519 (ex.: `mysql` do Oracle, Workbench) | Use o cliente `mariadb` 11.x; em último caso, `--casaos-native-motivo` (`docs/Instal_SSL.md` passo 8) |
| `mariadb-docker.sh`: "rode de novo com --recriar" | Container existente difere do desejado (rede, porta, `--dev-ip`, read-only, usuário) | Confira a lista `--dev-ip`; rode com `--recriar` (o banco fica fora do ar alguns segundos) |
| `mariadb-docker.sh`: "o container parou" com escrita negada fora de `/var/lib/mysql` | `--read-only` incompatível com a imagem | `--recriar --sem-read-only` |
| `permission denied ... docker.sock` no build | Usuário do Jenkins sem acesso ao socket, ou Jenkins não reiniciado | Seção 2.2 |
| Erro de schema do compose | Compose < 2.17 | Atualize o plugin `docker-compose-plugin` |
| `Bind for 0.0.0.0:8443 failed: port is already allocated` | Outra aplicação usa a 8443 | `sudo ss -ltnp \| grep 8443`. Libere a porta ou rode o job com outro `HTTPS_PORT` e ajuste `CORS_ALLOWED_ORIGINS` no Secret file |
| Smoke test falha após 30 tentativas | API caiu na subida (env inválido, banco inacessível) | O próprio job imprime as últimas linhas de `api`, `frontend` e `caddy`. Veja também a seção 9 |
| `compose` reclama de `api.env` não encontrado em comando manual | Falta `API_ENV_FILE=/dev/null` | Prefixe o comando com a variável (seção 9) |

---

## 11. Instalação inicial da produção (DEPLOY-07)

> **Execução do usuário**, uma vez, no servidor `192.168.168.106` (`ivo-inspiron-15-3530`) e na máquina do admin. A equipe não tem acesso ao servidor nem ao Docker dele: nada desta seção foi executado contra o servidor real.
>
> **Cenário (decisões do usuário, 2026-11-04):** não há banco anterior a aproveitar. O MariaDB 11.4 de produção (`rotaperfumes-mariadb`) nasce **vazio**, com uma CA **nova** (`--nova-ca`), a porta 3306 publicada **só** para a máquina do admin (`--dev-ip`) e o seed inicial: schema de `sql/`, o admin (senha gerada, troca obrigatória no 1º login) e os 42 vendedores **reais**, todos **inativos**. **Não há importação de dados:** o MariaDB 11.8 do CasaOS (porta 3307) continua sendo o banco de **desenvolvimento**, com a CA de dev em `deploy/ca-cert.pem`, e os dados dele não vão para a produção.
>
> **Por que esta ordem:** a `main` espera a rede `rotaperfumes-db`, o container `rotaperfumes-mariadb`, a CA de produção fixada em `deploy/db-ca.fingerprint`, o banco semeado e as credenciais do Jenkins. O push do passo 9 dispara o deploy (`pollSCM`, até ~2 min). Enquanto faltar algo, o estágio `CA do banco` recusa o build **antes** do `Deploy`, e nada muda no servidor.

| Passo | Onde | O quê |
|---|---|---|
| 1 | servidor | checkout do repositório com o DEPLOY-07 |
| 2 | servidor | `mariadb-docker.sh --nova-ca --dev-ip <IP do admin>`; anotar o `CA_FP` |
| 3 | servidor | chave da CA para mídia offline e `shred` |
| 4 | máquina de dev | `CA_FP` em `deploy/db-ca.fingerprint`, `make test`, commit (sem push) |
| 5 | servidor + Jenkins | `PASSWORD_PEPPER` novo, Secret file `rotaperfumes-api-env` e credencial `rotaperfumes-db-admin` |
| 6 | servidor | `seed-producao.sh --pepper-file ...` |
| 7 | servidor | conferências SQL |
| 8 | todos | checklist pré-push |
| 9 | máquina de dev | push na `main` (deploy) |
| 10 | navegador | 1º login do admin e ativação dos vendedores |
| 11 | máquina do admin | `.env.producao` + `deploy/ca-producao.pem` |
| 12 | máquina com Docker (não o servidor) | teste real do `mariadb-docker.sh` |

### Passo 1. Código no servidor

O checkout no servidor precisa ter o DEPLOY-07 (`deploy/seed-producao.sh`, `sql/ordem-ddl.txt` e o `deploy/mariadb-docker.sh` atual). Não é preciso branch especial:

```bash
sudo git clone --depth 1 https://github.com/residentivo/RotaPerfume.git /root/rotaperfumes
# já clonado antes:
#   sudo git -C /root/rotaperfumes pull --ff-only
sudo test -f /root/rotaperfumes/deploy/seed-producao.sh && echo "DEPLOY-07 presente"
```

- Se o commit do DEPLOY-07 já foi para a `main` do GitHub, o build que esse push disparou para no estágio `CA do banco` (`deploy/db-ca.fingerprint sem fingerprint: ...` ou `bind source path does not exist: /etc/mysql/ssl/ca.pem`). É o esperado nesta fase: o estágio roda antes do `Deploy` e nada muda no servidor.
- Se ainda não está no GitHub, copie da máquina de dev `deploy/`, `sql/` e `backend/shared/` para `/root/rotaperfumes/` (`scp -r`/`rsync`; o `seed-producao.sh` compila o `resetpassword` a partir de `backend/shared`). Não copie `.env`, `.env.producao` nem `deploy/ca-cert.pem`.

Nos passos 2, 3, 5, 6 e 7, os comandos rodam **no servidor**, como root (`sudo -i`), com `umask 077`, a partir de `/root/rotaperfumes`.

### Passo 2. MariaDB de produção com CA nova

```bash
sudo -i
umask 077
cd /root/rotaperfumes
bash deploy/mariadb-docker.sh --nova-ca --dev-ip <IP da máquina do admin>
```

- `--nova-ca` gera a CA de produção (`CN=RotaPerfumes_MariaDB_CA`, chave em `/root/rotaperfumes-ca/ca-key.pem`, que sai do servidor no passo 3) e o certificado do servidor (SAN `IP:192.168.168.106, DNS:rotaperfumes-mariadb, DNS:localhost, IP:127.0.0.1, DNS:<hostname>`). O script cria a rede `rotaperfumes-db`, o container, o banco `rotaperfumes` **vazio**, as contas e as senhas em `/root/rotaperfumes-mariadb/` (seções 3.1 a 3.4).
- `--dev-ip <IP>` publica `192.168.168.106:3306` **só** para esse IP (chain `ROTAPERFUMES-DB` em `DOCKER-USER`, seção 3.3) e cria a conta `casaos@<IP>` (`ALL ON rotaperfumes.*`), usada pelo `.env.producao` do passo 11. Use IP fixo (ou reserva no DHCP) na máquina do admin. **Repita sempre a mesma lista** de `--dev-ip` em toda execução do script: lista diferente faz o script pedir `--recriar`, e um IP que sai da lista perde a conta. **Sem `--dev-ip`, nada é publicado** e não há conta `casaos` nem `casaos.senha` (CHORE-17: um `casaos.senha` já existente é mantido, com aviso; descarte com `shred -u` se não for mais usar); a API e o Jenkins não precisam da porta (usam a rede interna).
- Confira:
  ```bash
  docker ps --filter name=rotaperfumes-mariadb --format '{{.Names}} {{.Image}} {{.Status}}'
  docker network inspect rotaperfumes-db -f '{{range .IPAM.Config}}{{.Subnet}} {{.IPRange}}{{end}}'   # 172.30.42.0/24 172.30.42.128/25
  ss -ltnp | grep ':3306 '              # 192.168.168.106:3306 (docker-proxy); nada além disso
  iptables -S ROTAPERFUMES-DB           # -s <IP do admin>/32 -j RETURN e, no fim, -j DROP
  iptables -S DOCKER-USER | grep ROTAPERFUMES-DB   # uma linha, com --ctdir ORIGINAL (DEPLOY-08)
  ```
- **Anote o `CA_FP`**: é o valor impresso no fim, em "CA do banco de PRODUCAO ... fingerprint SHA-256 (formato do deploy/db-ca.fingerprint)". Ele vai para o passo 4. Nesta fase, o resumo também avisa que o `deploy/db-ca.fingerprint` "ainda nao tem fingerprint" e que a chave da CA está no servidor (passo 3). Os dois avisos são esperados.
- **Não** copie esta CA para `deploy/ca-cert.pem`: esse arquivo é a CA de **dev** (CasaOS). Na máquina do admin, a CA de produção vai para `deploy/ca-producao.pem` (passo 11).

### Passo 3. Chave da CA para mídia offline

A chave da CA permite emitir certificados que a API aceita: ela **não fica no servidor** (`docs/Instal_SSL.md`, passo 3).

```bash
cp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem
cp /etc/mysql/ssl/ca.pem /root/rotaperfumes-ca/server-ext.cnf /media/<pendrive-cifrado>/
cmp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem && \
  shred -u /root/rotaperfumes-ca/ca-key.pem
```

Guarde uma segunda cópia (outro pendrive cifrado ou cofre de senhas). Para renovar o certificado depois, use `--ca-key /media/<pendrive>/ca-key.pem` (`docs/Instal_SSL.md`, passo 9.1).

### Passo 4. Fingerprint da CA no repositório (máquina de dev; sem push)

Na raiz do repositório, na máquina de dev, com o `CA_FP` do passo 2:

```bash
printf '%s\n' '<CA_FP do passo 2>' >> deploy/db-ca.fingerprint
grep -Ev '^[[:space:]]*(#|$)' deploy/db-ca.fingerprint        # uma linha só: o CA_FP (64 hex minúsculos)
env -u INTEGRATION make test
git add deploy/db-ca.fingerprint
git commit -m "DEPLOY-07: fingerprint da CA de producao"
```

- Mantenha as 3 linhas de cabeçalho (`#`); a **única** linha útil é o `CA_FP`. **Nunca** acrescente o fingerprint da CA de dev (`ef31b656...`, `deploy/ca-cert.pem`): o teste `TestDEPLOY07_CADevNuncaEmProducao` falha e o `mariadb-docker.sh` avisa.
- Para conferir no servidor: `openssl x509 -in /etc/mysql/ssl/ca.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f`.
- **Não faça push ainda**: o push é o passo 9, depois das credenciais e do seed.

### Passo 5. `PASSWORD_PEPPER` novo e credenciais do Jenkins

O banco é novo, então o pepper também é **novo** (nada a reaproveitar do dev). Ele vai para o Secret file **e** para o seed (passo 6): os dois precisam do **mesmo** valor, senão o admin não consegue logar. Depois de definido, **nunca troque** (todas as senhas Argon2id deixam de valer).

1. No servidor, gere o pepper e monte o `api.env` a partir de `deploy/api.env.example` (os segredos vão por `printf`, um builtin do bash, e nunca aparecem no argv):
   ```bash
   cd /root/rotaperfumes
   openssl rand -base64 48 | tr -d '\n' > /root/pepper-producao.txt
   chmod 600 /root/pepper-producao.txt
   A=/root/rotaperfumes-api.env
   {
     grep -Ev '^(DB_SENHA|JWT_SECRET|PASSWORD_PEPPER)=' deploy/api.env.example
     printf 'DB_SENHA=%s\n' "$(cat /root/rotaperfumes-mariadb/rotaperfumes_app.senha)"
     printf 'JWT_SECRET=%s\n' "$(openssl rand -base64 48 | tr -d '\n')"
     printf 'PASSWORD_PEPPER=%s\n' "$(cat /root/pepper-producao.txt)"
     printf 'DB_TLS_SERVER_NAME=rotaperfumes-mariadb\n'
   } > "$A"
   chmod 600 "$A"
   nano "$A"
   ```
   O arquivo já sai com `DB_HOST=rotaperfumes-mariadb`, `DB_PORT=3306`, `DB_NAME=rotaperfumes`, `DB_USUARIO=rotaperfumes_app`, `COOKIE_SECURE=true` e `TRUST_PROXY_HEADERS=true` do modelo (`DB_HOST`/`DB_PORT`/`DB_TLS_CA`/`DB_TLS_SERVER_NAME` são fixados pelo `docker-compose.yml` de qualquer forma). No editor, preencha o resto conforme a seção 5.1: `TURNSTILE_SECRET_KEY` **real** (seção 4), `SMTP_*` e `SECURITY_ALERT_EMAILS` (o SMTP é **necessário** para ativar os vendedores no passo 10) e confira `CORS_ALLOWED_ORIGINS`. **Não** inclua `DB_TLS_REQUIRED=false`.
2. Leve o arquivo para a máquina de onde você acessa o Jenkins, sem passar por chat ou e-mail. Por exemplo, no servidor: `install -m 600 -o <seu_usuario> "$A" /home/<seu_usuario>/rotaperfumes-api.env`; na sua máquina: `scp <seu_usuario>@192.168.168.106:rotaperfumes-api.env .`
3. Jenkins → Gerenciar Jenkins → Credentials → System → Global credentials:
   - **Secret file** com ID `rotaperfumes-api-env` (se já existir: abra → **Update** → envie o arquivo novo).
   - **Username with password** com ID `rotaperfumes-db-admin`: usuário `rotaperfumes_admin`, senha = `sudo cat /root/rotaperfumes-mariadb/rotaperfumes_admin.senha` (lida no terminal do servidor e colada direto no Jenkins).
4. Apague as cópias: na sua máquina, o `rotaperfumes-api.env`; no servidor, `shred -u /home/<seu_usuario>/rotaperfumes-api.env /root/rotaperfumes-api.env`. **Mantenha** o `/root/pepper-producao.txt` até o passo 6.

> Não use o `setup-servidor.sh` com o `.env` de dev para gerar o Secret file: ele copia o `PASSWORD_PEPPER` do `--env` (o de dev), que não é o do seed de produção.

### Passo 6. Seed inicial (`deploy/seed-producao.sh`)

```bash
cd /root/rotaperfumes
bash deploy/seed-producao.sh --pepper-file /root/pepper-producao.txt
```

- O `--pepper-file` precisa ser arquivo regular do root com modo 600 ou 400 (o do passo 5 já é); sem a opção, o script pede o pepper no terminal, sem eco e com confirmação. O valor tem de ser o **mesmo** `PASSWORD_PEPPER` do Secret file, com 32 caracteres ou mais.
- **Sem terminal (BUG-26):** rodado sem terminal de controle (ex.: `ssh` sem `-t`, cron, pipe), o script **falha logo na etapa 1**, antes do Docker e do banco, com mensagem clara: sem `--pepper-file`, `sem terminal para pedir o PASSWORD_PEPPER: use --pepper-file ARQ`; com `--forcar`, `--forcar exige um terminal para a confirmacao` (mesmo com o banco vazio). Nada é alterado. Rode de um terminal interativo (`sudo -i` numa sessão `ssh -t`) ou passe `--pepper-file`.
- As 7 etapas (`[n/7]`; detalhes em `bash deploy/seed-producao.sh --help`): (1) confere root, Docker, container, rede, CA, senha do `rotaperfumes_admin`, `sql/ordem-ddl.txt`, seeds, pepper e o destino da senha do admin, sem mudar nada; (2) **recusa** se a tabela `usuarios` já tem linhas (banco já semeado; `--forcar` exige digitar o nome do banco, reaplica tudo, troca a senha do admin e inativa **todos** os não-admin: só numa instalação ainda sem uso); (3) DDL na ordem de `sql/ordem-ddl.txt` (a mesma do `make db-up`); (4) `sql/02_seed_admin.sql` e `sql/03_seed_vendedores.sql` e `UPDATE usuarios SET ativo = 0 WHERE role <> 'admin'`; (5) compila o `resetpassword` num container `GO_IMAGE` (sem rede do banco nem segredos; baixa os módulos do proxy do Go, então o servidor precisa de internet); (6) `resetpassword -create-admin` num container na rede `rotaperfumes-db` (IP `172.30.42.20`, conta `rotaperfumes_admin`, TLS verificado), com os segredos num `--env-file` temporário 600 apagado com `shred`; (7) conferência.
- **Senha do admin:** gerada e **nunca impressa**, gravada em `/root/rotaperfumes-mariadb/admin-app.senha` (600, só root; o script recusa se o arquivo já existir). Outro destino: `--admin-senha-arquivo ARQ`. Para digitar a senha em vez de gerar: `--admin-senha-prompt` (exige terminal). Em qualquer caso, o admin fica com `deve_trocar_senha = 1`.
- O script **não** roda `-all-users`: os 42 vendedores ficam com o hash placeholder (ninguém loga com ele) e inativos até o passo 10.
- **Nunca** use `make db-seed`, `make db-reset` ou `make fix-hash` contra este banco (a guarda `db-guard-producao` recusa; seção 3.8).
- Depois do seed: guarde uma cópia do pepper num cofre de senhas ou na mídia offline do passo 3 (sem ele, nenhuma senha de produção funciona se o Secret file se perder) e apague o arquivo do servidor:
  ```bash
  shred -u /root/pepper-producao.txt
  ```
  O pepper continua no Secret file `rotaperfumes-api-env`.

### Passo 7. Conferências SQL

Pela rede interna, com o mesmo cliente do `seed-producao.sh` e do estágio `Importar dump` (imagem fixada = `IMAGEM_BASE`/`DB_CLIENT_IMAGE`, conta `rotaperfumes_admin` em `172.30.42.20`, senha por `MYSQL_PWD`, nunca no argv):

```bash
IMG='mariadb:11.4.13@sha256:d1891ede5a559f1086f55053bb8cc1569b0298338c4d72d71725a7f74f6ba620'   # = IMAGEM_BASE
admin_sql() {
  MYSQL_PWD=$(cat /root/rotaperfumes-mariadb/rotaperfumes_admin.senha) \
  docker run --rm -i -e MYSQL_PWD --network rotaperfumes-db --ip 172.30.42.20 \
    --cap-drop ALL --security-opt no-new-privileges:true \
    --mount type=bind,source=/etc/mysql/ssl/ca.pem,target=/ca.pem,readonly \
    --entrypoint mariadb "$IMG" --local-infile=0 -h rotaperfumes-mariadb -P 3306 -u rotaperfumes_admin \
      --ssl-ca=/ca.pem --ssl-verify-server-cert --default-character-set=utf8mb4 "$@"
}
admin_sql rotaperfumes <<'SQL'
SELECT COUNT(*) AS tabelas FROM information_schema.TABLES WHERE TABLE_SCHEMA = 'rotaperfumes';
SELECT id, email, role, ativo, deve_trocar_senha, LEFT(password_hash, 10) AS hash FROM usuarios WHERE role = 'admin';
SELECT COUNT(*) AS vendedores FROM vendedores;
SELECT COUNT(*) AS nao_admin, SUM(ativo <> 0) AS nao_admin_ativos FROM usuarios WHERE role <> 'admin';
SQL
```

Esperado: `tabelas` = 13; uma linha de admin (`admin@rotaperfumes.com.br`) com `ativo` = 1, `deve_trocar_senha` = 1 e `hash` começando com `$argon2id$`; `vendedores` = 42; `nao_admin` = 42 e `nao_admin_ativos` = 0. Qualquer diferença: **não** siga para o push; veja a saída do `seed-producao.sh`.

### Passo 8. Checklist pré-push

- [ ] `docker ps` mostra `rotaperfumes-mariadb` `Up` e `docker network inspect rotaperfumes-db` funciona;
- [ ] `/etc/mysql/ssl/ca.pem` legível por "outros" (`sudo -u nobody cat /etc/mysql/ssl/ca.pem >/dev/null && echo OK`; `docs/Instal_SSL.md`, passo 5);
- [ ] o fingerprint de `/etc/mysql/ssl/ca.pem` é a **única** linha útil do `deploy/db-ca.fingerprint` **commitado** (passo 4), e não é o da CA de dev (`ef31b656...`);
- [ ] a chave da CA saiu do servidor (`test ! -e /root/rotaperfumes-ca/ca-key.pem && echo OK`);
- [ ] Secret file `rotaperfumes-api-env` cadastrado ou atualizado (passo 5), com o **mesmo** `PASSWORD_PEPPER` do seed, `JWT_SECRET` novo, `TURNSTILE_SECRET_KEY` real, `SMTP_*`, `COOKIE_SECURE=true` e sem `DB_TLS_REQUIRED=false`; credencial `rotaperfumes-db-admin` cadastrada;
- [ ] seed conferido (passo 7);
- [ ] job `rotaperfumes-deploy` criado (seção 6) e o parâmetro **`IMPORTAR_DUMP` desmarcado** (padrão `false`, que é o valor usado pelo `pollSCM`). Não há dump a importar: marcado, ele **sobrescreveria** o banco de produção;
- [ ] `make deps-audit` rodado (`CLAUDE.md`: antes de cada deploy).

### Passo 9. Push na `main` (dispara o deploy)

Na máquina de dev:

```bash
git push origin main
```

Em até ~2 minutos o `pollSCM` dispara o job `rotaperfumes-deploy` com os valores padrão. Estágios esperados: `Checkout` → `Testes Go` → `Testes Frontend` → `Build` → `CA do banco` (lista o fingerprint aceito, cada certificado com `na lista (deploy/db-ca.fingerprint)`, `s_client: Verify return code: 0 (ok)` e `CA do banco validada: /etc/mysql/ssl/ca.pem (sha256 ...)`) → `Importar dump` (pulado: `IMPORTAR_DUMP=false`) → `Deploy` (a API entra na rede `rotaperfumes-db` com `172.30.42.10`) → `Smoke test` verde. Depois, no servidor:

```bash
docker logs rotaperfumes-api-1 2>&1 | grep -E 'TLS do banco|conexão MariaDB estabelecida'
# [server] TLS do banco: ligado (ca=/etc/rotaperfumes/db-ca.pem, server_name=rotaperfumes-mariadb, min=TLS1.3)
# [db] conexão MariaDB estabelecida (host=..., tls=ligado (custom))
```

Falhou? A seção 10 (Troubleshooting) cobre os erros de cada estágio. O `CA do banco` roda antes do `Deploy`: se ele falhar, nada foi alterado.

### Passo 10. Primeiro login do admin e ativação dos vendedores

1. No servidor, leia a senha inicial **no terminal** (não cole em chat, log ou ticket): `sudo cat /root/rotaperfumes-mariadb/admin-app.senha`.
2. Acesse `https://ivo-inspiron-15-3530:8443` (CA do Caddy instalada, seção 8) e faça login com `admin@rotaperfumes.com.br`. Com `deve_trocar_senha = 1`, o frontend leva para `/trocar-senha`, e a API responde `403` `"troca de senha obrigatória"` em todas as outras rotas protegidas até a troca (SEC-23). Troque a senha e faça login de novo.
3. Apague o arquivo da senha inicial: `sudo shred -u /root/rotaperfumes-mariadb/admin-app.senha`.
4. **Ativar os vendedores:** os 42 usuários de vendedor estão **inativos** e com hash placeholder (ninguém loga). Para cada vendedor que vai usar o sistema, como admin, na tela **Usuários**: confira o e-mail cadastrado (o seed usa `nome.sobrenome@rotaperfumes.com.br`), clique em **Resetar** (a API gera uma senha aleatória, grava com troca obrigatória no 1º login e a envia **por e-mail**; `POST /api/admin/reset-password`) e **ative** o usuário pelo botão de status (`PATCH /api/usuarios/{id}/inativar`). O reset sozinho não ativa o usuário.
   - O reset depende do **SMTP** do Secret file: sem SMTP, a resposta vem com `email_enviado = false` e a senha gerada não chega a ninguém (não aparece na tela nem no log). Configure o `SMTP_*` antes.
   - Usuários de vendedores desligados (vendedores 1, 3, 9, 30, 37 e 38) continuam inativos.
   - Não use `make reset-senhas-ativos` nem o `resetpassword -all-users` para isso.

### Passo 11. Máquina do admin: `.env.producao`

Só a máquina do `--dev-ip` (passo 2) alcança `192.168.168.106:3306`. Ela usa **arquivos separados** para a produção; o `.env` padrão continua apontando para o banco de **dev** do CasaOS (`DB_HOST=192.168.168.106`, `DB_PORT=3307`, `DB_TLS_CA=deploy/ca-cert.pem`; a guarda `db-guard-producao` deixa passar a porta 3307).

1. Na raiz do repositório, copie a CA de produção (pública) e confira o fingerprint:
   ```bash
   scp <seu_usuario>@192.168.168.106:/etc/mysql/ssl/ca.pem deploy/ca-producao.pem
   openssl x509 -in deploy/ca-producao.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f
   # igual à linha útil de deploy/db-ca.fingerprint
   git check-ignore deploy/ca-producao.pem .env.producao       # os dois precisam aparecer (estão no .gitignore)
   ```
2. Crie o `.env.producao` (modo 600) com:
   ```
   # ATENCAO: PRODUCAO (dados reais). Use só com ENV_ARQUIVO=.env.producao.
   DB_HOST=192.168.168.106
   DB_PORT=3306
   DB_NAME=rotaperfumes
   DB_USUARIO=casaos
   DB_SENHA=<conteúdo de /root/rotaperfumes-mariadb/casaos.senha>
   DB_TLS_CA=deploy/ca-producao.pem
   ```
   Leia a senha no terminal do servidor (`ssh -t <seu_usuario>@192.168.168.106 sudo cat /root/rotaperfumes-mariadb/casaos.senha`) e cole direto no editor. `DB_TLS_SERVER_NAME` fica vazio: o IP `192.168.168.106` está no SAN do certificado (o cliente `mariadb` dos alvos `make db-*` confere o próprio `DB_HOST`). O cliente precisa do plugin `client_ed25519` (cliente `mariadb` 11.x).
3. Uso: `ENV_ARQUIVO` faz o `make` ler o `.env.producao` **no lugar** do `.env`, e só vale na linha de comando. Os alvos que escrevem no banco ainda exigem a confirmação explícita da guarda `db-guard-producao`:
   ```bash
   make db-check-env ENV_ARQUIVO=.env.producao
   openssl s_client -starttls mysql -connect 192.168.168.106:3306 -CAfile deploy/ca-producao.pem \
     -verify_return_error -verify_ip 192.168.168.106 </dev/null 2>/dev/null | grep -E 'Verify return code|Cipher is'
   # esperado: "Cipher is TLS_AES_..." (nunca "(NONE)") E "Verify return code: 0 (ok)"
   make <alvo> ENV_ARQUIVO=.env.producao CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes
   ```
   O `Verify return code: 0 (ok)` **sozinho é enganoso**: sem handshake (ex.: o `mariadbd` recusou o host com o erro 1130, ou a máquina não está em `--dev-ip`) o `s_client` também o imprime, junto com `no peer certificate available` e `Cipher is (NONE)`. Só vale com uma cifra de verdade em `Cipher is`. Esse teste só funciona de uma máquina listada em `--dev-ip`; do próprio servidor ele nunca faz handshake (DEPLOY-08).

> **Aviso forte: dados reais.** Com o `.env.producao`, a conta `casaos` tem `ALL` no banco de produção. **Nunca** rode `make db-down`, `db-reset`, `db-rebuild`, `db-seed`, `db-import-*`, `db-seed-mes-atual`, `gen-hash`, `fix-hash` nem `reset-senhas-ativos` contra ele, mesmo com o `CONFIRMO_PRODUCAO`: eles apagam, sobrescrevem ou trocam senhas reais (SEC-46: `db-down`, `db-reset`, `db-rebuild` e `db-seed` são recusados na produção mesmo com o `CONFIRMO_PRODUCAO`; os demais dependem só da sua atenção). Faça backup (seção 9) antes de qualquer migração (`db-fix-*`). Não use o `.env.producao` com `make dev-api` nem com as CLIs Go (a guarda não cobre esses casos). Os testes de integração recusam a produção antes de conectar (SEC-50) e o `make test` não recebe as variáveis do arquivo lido (SEC-53), mas também não aponte o `go test` para ela. Nunca copie a CA de produção para `deploy/ca-cert.pem` nem o conteúdo do `.env.producao` para o `.env`.

### Passo 12. Teste real do script (`TestTST15_MariadbDocker_Real`)

Numa máquina **com Docker que não seja o servidor** (o teste usa `/root/rotaperfumes-ca`, `/root/rotaperfumes-mariadb` e `/opt/rotaperfumes/mariadb-imagem`, e se recusa a rodar se os dois primeiros já existirem), como root, na raiz do repositório:

```bash
cd backend/shared
sudo -E env "PATH=$PATH" TST15_DOCKER=1 go test -count=1 -run TestTST15_MariadbDocker_Real ./tests/deploy/
```

Ele sobe um container `tst15-mariadb` numa rede `tst15-db` (`172.30.243.0/24`) com `--nova-ca`, confere as contas ed25519, `REQUIRE SSL`, a recusa sem TLS, nenhuma porta sem `--dev-ip`, a reexecução sem troca de senha e o `--novo-cert` com `FLUSH SSL`, e limpa tudo no fim. Se puder, rode-o **antes** do passo 2. Registre o resultado no card TST-15.

O `deploy/seed-producao.sh` ainda **não** tem teste real com Docker (só testes estáticos e de ensaio em `backend/shared/tests/deploy/`); a primeira execução real é a do passo 6. Pendência registrada no card DEPLOY-07.
