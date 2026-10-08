# Deploy no servidor da LAN (`ivo-inspiron-15-3530`) com Docker + Jenkins

> Card **DEPLOY-01** (Lote 13, 2026-09-30). Arquivos envolvidos: `Jenkinsfile`, `deploy/docker-compose.yml`, `deploy/Caddyfile`, `deploy/Dockerfile.caddy`, `deploy/api.env.example`, `deploy/mysql-setup.sql`, `deploy/dump-local.ps1`, `deploy/setup-servidor.sh`, `apis/Dockerfile`, `frontend/Dockerfile`.

## 0. Instalação automática (recomendado)

O script `deploy/setup-servidor.sh` faz, no servidor, tudo o que as seções 2, 3, 6 e 7 descrevem. As **duas credenciais do Jenkins** continuam sendo cadastradas **à mão**, porque o script não cria, não altera e não apaga credenciais. Ele pausa, mostra as instruções e depois só **confere** (por leitura) se elas existem.

**Antes:** faça commit e push dos arquivos de deploy para o `main`, gere o dump no Windows (seção 7.1) e deixe o Jenkins com os plugins Pipeline, Git e Credentials Binding. O Jenkins precisa ser **nativo** (serviço systemd `jenkins`). Se ele rodar em container, o script aborta e explica o motivo.

1. **No Windows**, na raiz do projeto, copie o dump e o `.env` para o `/tmp` do servidor:
   ```powershell
   scp deploy\dumps\rotaperfumes-AAAAMMDD-HHMMSS.sql.gz .env SEU_USUARIO@ivo-inspiron-15-3530:/tmp/
   ```
2. **No servidor**, baixe e rode o script:
   ```bash
   curl -fsSLO https://raw.githubusercontent.com/residentivo/RotaPerfume/main/deploy/setup-servidor.sh
   sudo bash setup-servidor.sh --env /tmp/.env --dump /tmp/rotaperfumes-AAAAMMDD-HHMMSS.sql.gz \
     --turnstile-site-key SUA_SITE_KEY --remove-inputs
   ```
   Opções: `--site-host` (padrão `ivo-inspiron-15-3530`), `--site-ip` (`192.168.168.106`), `--https-port` (`8443`), `--jenkins-url` (`http://localhost:8888`), `--skip-mysql` (não mexe em banco/usuários e pede a senha atual do `rotaperfumes_app`), `--skip-build` e `--remove-inputs` (apaga o `.env` e o dump do `/tmp` no final). Veja `bash setup-servidor.sh --help`.
3. O script executa 11 etapas e mostra o andamento como `[n/11]`:
   1. **Pré-checagens:** root via `sudo`, `docker compose` ≥ 2.17, integridade do dump (`gzip -t`), Jenkins nativo, MySQL ou MariaDB e acesso root ao banco. Se o acesso root por socket falhar, o script pede a senha do root.
   2. Coloca o usuário `jenkins` no grupo `docker`. O Jenkins só é reiniciado se o grupo mudou.
   3. **bind-address:** se o banco escuta só em loopback, grava `zz-rotaperfumes.cnf` com `bind-address = 0.0.0.0` e reinicia o banco.
   4. **ufw:** se estiver ativo, libera a porta do banco só para `172.16.0.0/12`, nega o resto e libera a 8443. Se estiver inativo, **só avisa** e sugere os comandos. O script nunca ativa o ufw.
   5. **Banco e usuários:** cria o banco `rotaperfumes` e os usuários `rotaperfumes_app` e `rotaperfumes_admin`. A senha do app é gerada automaticamente. A senha do admin é **você** que escolhe, com no mínimo 12 caracteres, e a digita depois no Jenkins. Se os usuários já existirem, o script pergunta se deve redefinir as senhas.
   6. Instala o dump em `/opt/rotaperfumes/dumps/rotaperfumes.sql.gz` (dono `root:jenkins`, modo 640).
   7. Gera `~/rotaperfumes-api.env` (modo 600) a partir do seu `.env`. Substitui `DB_*`, gera um `JWT_SECRET` novo e define `CORS_ALLOWED_ORIGINS` e `TRUST_PROXY_HEADERS=true`. As demais chaves, inclusive `PASSWORD_PEPPER`, vêm do seu `.env`: o pepper precisa ser o mesmo que gerou os hashes do dump. Se algum valor tiver `$` sem aspas simples, o script avisa (mostra só o nome da chave).
   8. Pede seu usuário e um **API token** do Jenkins (Jenkins → seu usuário → Security → API Token), confere os plugins e cria ou atualiza o job `rotaperfumes-deploy`.
   9. **Pausa para as credenciais manuais.** Siga as instruções que aparecem na tela:
      1. No Windows: `scp SEU_USUARIO@ivo-inspiron-15-3530:rotaperfumes-api.env .`
      2. Jenkins → Gerenciar Jenkins → Credentials → System → Global credentials → **Add Credentials**. Kind **Secret file**, ID `rotaperfumes-api-env`, arquivo `rotaperfumes-api.env`.
      3. **Add Credentials** de novo. Kind **Username with password**, ID `rotaperfumes-db-admin`, usuário `rotaperfumes_admin`, senha = a escolhida na etapa 5.
      4. Apague o `rotaperfumes-api.env` do Windows.

      Tecle ENTER. O script confere as duas credenciais e, se estiverem cadastradas, apaga o `~/rotaperfumes-api.env` do servidor.
   10. Dispara o build com `IMPORTAR_DUMP=true` (quando há `--dump`) e mostra o log ao vivo. Se o build não terminar em `SUCCESS`, o script sai com erro.
   11. **Resumo:** mostra a URL de acesso, exporta a CA do Caddy para `~/rotaperfumes-caddy-root.crt` (instale no Windows como na seção 8) e lembra de cadastrar o hostname no Turnstile.
4. **Reexecução:** o script pode ser rodado de novo sem problemas. Se ele **redefinir as senhas** ou gerar um `api.env` novo, que sempre traz um `JWT_SECRET` novo, **atualize** as credenciais no Jenkins: abra a credencial → **Update** → envie o arquivo novo ou digite a senha nova. O script só confere se as credenciais existem, não o conteúdo delas.

As seções abaixo descrevem o mesmo processo **manualmente**, como alternativa ou para diagnóstico.

## 1. Visão geral

```
Navegador (LAN)
   │  https://ivo-inspiron-15-3530:8443     (única porta publicada no host)
   ▼
┌──────────── docker compose -p rotaperfumes ────────────┐
│  caddy (tls internal, 443 no container → 8443 no host) │
│     ├── /api/health → api:8080/health                  │
│     ├── /api/*      → api:8080       (API Go)          │
│     └── resto       → frontend:3000  (Next standalone) │
└──────────────────────────┬─────────────────────────────┘
                           │ host.docker.internal:3306
                           ▼
                 MySQL do HOST (fora do Docker), banco rotaperfumes
```

- O **Jenkins** (`http://ivo-inspiron-15-3530:8888/`) clona `residentivo/RotaPerfume` (branch `main`) e roda o `Jenkinsfile`, com estes estágios: `Checkout` → `Testes Go` (vet + test dentro de `golang:1.26-alpine`) → `Build` (imagens do compose) → `Importar dump` (só com `IMPORTAR_DUMP=true`) → `Deploy` (`up -d`) → `Smoke test` (`/api/health` e `/login` pelo Caddy, acessando direto `https://SITE_IP:HTTPS_PORT`).
- A API e o frontend **não** publicam porta no host. Só o Caddy publica.
- **HTTPS é obrigatório:** os cookies de autenticação são `Secure`. Por HTTP o login "funciona", mas a sessão não se mantém.
- O frontend e a API ficam na **mesma origem** (`https://ivo-inspiron-15-3530:8443`). O frontend chama `${PUBLIC_URL}/api/...`, e o `PUBLIC_URL` é embutido no bundle **no build**.

### Parâmetros do job (definidos no `Jenkinsfile`)

| Parâmetro | Padrão | Uso |
|---|---|---|
| `SITE_HOST` | `ivo-inspiron-15-3530` | Hostname de acesso (Caddy e `PUBLIC_URL`) |
| `SITE_IP` | `192.168.168.106` | IP da LAN (o Caddy também aceita esse IP; usado no smoke test) |
| `HTTPS_PORT` | `8443` | Porta publicada pelo Caddy |
| `TURNSTILE_SITE_KEY` | *(vazio)* | Site key **pública** do Turnstile, embutida no bundle |
| `IMPORTAR_DUMP` | `false` | Importa o dump no MySQL. **Sobrescreve as tabelas** |
| `DUMP_PATH` | `/opt/rotaperfumes/dumps/rotaperfumes.sql.gz` | Caminho do dump no servidor |

### Credenciais Jenkins exigidas

| ID | Tipo | Conteúdo |
|---|---|---|
| `rotaperfumes-api-env` | Secret file | `.env` de produção da API (modelo: `deploy/api.env.example`) |
| `rotaperfumes-db-admin` | Username with password | Usuário MySQL com DDL no banco `rotaperfumes` (usado só quando `IMPORTAR_DUMP=true`) |

---

## 2. Pré-requisitos no servidor

Rode no servidor (via SSH ou no terminal local dele):

1. Instale o Docker Engine e o plugin compose. O compose precisa ser **≥ 2.17** (checado pelo `setup-servidor.sh`). O build funciona com o builder clássico ou com BuildKit; a imagem do Caddy usa `deploy/Dockerfile.caddy`:
   ```bash
   docker --version
   docker compose version     # precisa ser v2.17 ou superior
   ```
2. Coloque o usuário do Jenkins no grupo `docker` e **reinicie o Jenkins** (sem o restart, o processo continua sem o grupo novo):
   ```bash
   sudo usermod -aG docker jenkins
   sudo systemctl restart jenkins
   sudo -u jenkins docker ps          # deve listar sem "permission denied"
   ```
3. Confirme as ferramentas usadas pelo pipeline: `bash`, `curl`, `gzip`/`zcat` e `install` (coreutils).
   ```bash
   which bash curl gzip zcat install
   ```
4. No Jenkins, confirme que os plugins **Pipeline**, **Git** e **Credentials Binding** estão instalados (vêm nos "suggested plugins").
5. Confirme que a porta **8443** está livre:
   ```bash
   sudo ss -ltnp | grep ':8443' || echo "8443 livre"
   ```

---

## 3. Preparar o MySQL do servidor

A API roda em container e acessa o MySQL do host por `host.docker.internal` (IP da bridge `docker0`, normalmente `172.17.0.1`). As redes do compose ficam em `172.16.0.0/12`.

1. **bind-address.** Por padrão, o MySQL do Ubuntu escuta só em `127.0.0.1`, que os containers não alcançam. Edite `/etc/mysql/mysql.conf.d/mysqld.cnf`, seção `[mysqld]`, e escolha **uma** opção:
   - `bind-address = 0.0.0.0` (todas as interfaces). Nesse caso, bloqueie a 3306 para a LAN no firewall (passo 4).
   - `bind-address = 127.0.0.1,172.17.0.1` (MySQL ≥ 8.0.13). O MySQL precisa subir **depois** do Docker, senão falha no bind.

   Depois reinicie:
   ```bash
   sudo systemctl restart mysql
   ```
2. **Usuários e banco.** Gere senhas fortes, sem `$`, `#`, aspas nem espaços:
   ```bash
   openssl rand -base64 32     # senha do rotaperfumes_app
   openssl rand -base64 32     # senha do rotaperfumes_admin
   ```
   Edite uma **cópia temporária** do script. Não altere nem versione o arquivo do repositório com a senha.
   ```bash
   cp deploy/mysql-setup.sql /tmp/mysql-setup.sql
   nano /tmp/mysql-setup.sql
   ```
   - Troque `<TROCAR>` pela senha do `rotaperfumes_app`. Esse usuário tem só SELECT/INSERT/UPDATE/DELETE e é o que a API usa.
   - **Descomente** o bloco do `rotaperfumes_admin` e troque `<TROCAR_ADMIN>`. Esse usuário tem ALL PRIVILEGES no banco, porque o dump faz DROP/CREATE TABLE e o pipeline roda `TRUNCATE refresh_tokens`. Ele será a credencial `rotaperfumes-db-admin`. O `root` via socket **não** serve, porque o import sai por um container na rede docker.
   ```bash
   sudo mysql < /tmp/mysql-setup.sql
   shred -u /tmp/mysql-setup.sql
   ```
   O script cria o banco `rotaperfumes` (utf8mb4) e os usuários `@'172.16.0.0/255.240.0.0'`.
3. Guarde as duas senhas para os passos 5.1 (`DB_SENHA`) e 5.2 (admin).
4. **Firewall (se usar ufw):**
   ```bash
   sudo ufw allow from 172.16.0.0/12 to any port 3306 proto tcp   # containers
   sudo ufw deny 3306                                              # resto da LAN (necessário com bind 0.0.0.0)
   sudo ufw allow 8443/tcp                                         # acesso ao sistema
   ```
   A regra `allow` da faixa docker precisa vir **antes** do `deny` (o ufw avalia as regras em ordem).

---

## 4. Cloudflare Turnstile

1. No painel do Cloudflare (Turnstile → seu site), cadastre em **Hostnames** o hostname de acesso: `ivo-inspiron-15-3530`.
2. Anote a **Site Key**, que é pública e vai no parâmetro `TURNSTILE_SITE_KEY` do job, e a **Secret Key**, que vai em `TURNSTILE_SECRET_KEY` no Secret file.
3. **Acesse sempre pelo hostname**, e não pelo IP. O widget só valida nos hostnames cadastrados.
4. Se o PC cliente não resolver `ivo-inspiron-15-3530`, adicione esta linha em `C:\Windows\System32\drivers\etc\hosts` (como administrador):
   ```
   192.168.168.106   ivo-inspiron-15-3530
   ```
5. Se o Cloudflare não aceitar o hostname da LAN, dá para usar as **chaves de teste** da Cloudflare, que sempre passam: site `1x00000000000000000000AA`, secret `1x0000000000000000000000000000000AA`. Com elas, o Turnstile **deixa de proteger** o login. Use só nesta rede interna e sabendo desse risco.

---

## 5. Credenciais no Jenkins

Caminho: **Gerenciar Jenkins → Credentials → System → Global credentials (unrestricted) → Add Credentials**.

### 5.1 Secret file `rotaperfumes-api-env`

1. Numa máquina de confiança, copie `deploy/api.env.example` para um arquivo temporário (ex.: `api.env`) **fora do repositório**.
2. Preencha:
   - `DB_SENHA` = senha do `rotaperfumes_app` (passo 3.2).
   - `JWT_SECRET` = valor **novo**, diferente do ambiente local:
     ```bash
     openssl rand -base64 48
     ```
   - `TURNSTILE_SECRET_KEY` = secret key do Turnstile.
   - `PASSWORD_PEPPER` = pepper das senhas (SEC-13). Use **o mesmo do `.env` que gerou os hashes do dump importado**: com outro valor, nenhuma senha Argon2id do dump funciona (hashes bcrypt antigos continuam valendo e migram no próximo login). Num banco novo, gere com `openssl rand -base64 48`. Sem ele, ou com menos de 32 bytes, a API não sobe. Depois de definido, **não troque**.
   - `SMTP_*` e `SECURITY_ALERT_EMAILS`, se for enviar e-mails. Se ficarem vazios, os e-mails vão só para o log.
   - Se mudar `SITE_HOST`/`HTTPS_PORT`, ajuste também o `CORS_ALLOWED_ORIGINS`.
   - Se algum valor tiver `$`, coloque-o entre aspas **simples**.
3. Em **Add Credentials**: Kind **Secret file**, envie o arquivo e informe o ID **`rotaperfumes-api-env`** (exatamente assim).
4. **Apague** o arquivo preenchido da máquina local.

O pipeline grava esse arquivo em `deploy/api.env` (modo 600) só durante o deploy e o apaga no final (`post { always }`).

### 5.2 Username with password `rotaperfumes-db-admin`

1. Kind **Username with password**.
2. Username `rotaperfumes_admin`, Password = senha do admin (passo 3.2).
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

## 7. Dump do banco local e primeira carga

1. **No Windows (máquina de desenvolvimento)**, na raiz do projeto:
   ```powershell
   powershell -ExecutionPolicy Bypass -File deploy\dump-local.ps1
   ```
   O script lê `DB_*` do `.env` da raiz, roda `mysqldump` e grava `deploy\dumps\rotaperfumes-AAAAMMDD-HHMMSS.sql.gz`. A pasta é ignorada pelo git.
2. **No servidor**, prepare a pasta. O Jenkins precisa conseguir **ler** o arquivo:
   ```bash
   sudo mkdir -p /opt/rotaperfumes/dumps
   sudo chown "$USER":jenkins /opt/rotaperfumes/dumps
   sudo chmod 750 /opt/rotaperfumes/dumps
   ```
3. **No Windows**, copie o dump com scp. O comando pede a senha do usuário SSH, porque não há chave autorizada:
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
   - importa via container `mysql:8.4` com o usuário `rotaperfumes-db-admin`;
   - roda `TRUNCATE TABLE refresh_tokens`, porque as sessões do ambiente local não valem no servidor.

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
| Parar (mantém containers) | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes stop` |
| Religar containers parados | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes start` |
| Remover (mantém o volume da CA) | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes down` |

- **Nunca** rode `up` manualmente com `API_ENV_FILE=/dev/null`, porque a API seria recriada sem as variáveis. Para subir ou recriar, **rode o job**.
- Os containers têm `restart: unless-stopped` e voltam sozinhos após um reboot do servidor. A exceção é se tiverem sido parados com `stop`.
- Os logs rodam em `json-file` com até 5 × 10 MB por serviço.

### Atualizar

1. Faça commit e push no `main`.
2. Jenkins → `rotaperfumes-deploy` → **Construir com parâmetros**, com `IMPORTAR_DUMP` desmarcado e a mesma `TURNSTILE_SITE_KEY`.
3. O pipeline testa, reconstrói as imagens, recria os containers e roda o smoke test.

Para mudar só segredos (ex.: senha SMTP): atualize a credencial `rotaperfumes-api-env` no Jenkins e rode o job.

### Rollback

As imagens são sempre `:latest`, sem tag por versão. Para voltar:

1. No repositório, `git revert <commit-problemático>` e push no `main`. Evite reescrever o histórico.
2. Rode o job de novo.
3. Se o problema for de dados, reimporte um dump bom (seção 7) com `IMPORTAR_DUMP=true`. O MySQL do host não é versionado pelo pipeline, então faça backup antes de importar.

---

## 10. Troubleshooting

| Sintoma | Causa provável | Como resolver |
|---|---|---|
| Login aceita a senha, mas volta para a tela de login ou perde a sessão ao recarregar | Acesso por HTTP, ou certificado não confiável (cookie `Secure` descartado) | Acesse **https://ivo-inspiron-15-3530:8443** e instale a CA do Caddy (seção 8) |
| Aviso "Sua conexão não é particular" | CA do Caddy não instalada no cliente, ou CA regenerada (volume apagado) | Reexporte e reinstale o `root.crt` (seção 8) |
| Widget do Turnstile não aparece ou dá erro, ou o login recusa o captcha | Hostname não cadastrado no Cloudflare; acesso pelo IP; site key vazia/errada no build; secret errada no `api.env` | Cadastre o hostname, acesse pelo hostname e rode o job com `TURNSTILE_SITE_KEY` correta (ela é embutida no **build**). Confira `TURNSTILE_SECRET_KEY` na credencial |
| Frontend chama URL errada da API | `SITE_HOST`/`HTTPS_PORT` diferentes do acesso real (o `PUBLIC_URL` é fixado no build) | Rode o job com os parâmetros certos |
| API não conecta no MySQL (logs com `connection refused`/`timeout`) | `bind-address` em 127.0.0.1; ufw bloqueando a faixa docker | Seção 3.1 e 3.4, depois `sudo systemctl restart mysql` |
| API: `Access denied for user 'rotaperfumes_app'` | Senha diferente entre o MySQL e o `DB_SENHA`; usuário criado com outro host | Confira o `mysql-setup.sql` aplicado (host `172.16.0.0/255.240.0.0`) e a credencial `rotaperfumes-api-env` |
| Estágio **Importar dump** falha com "Dump nao encontrado/legivel" | Arquivo fora do `DUMP_PATH` ou sem permissão para o usuário `jenkins` | Seção 7.2/7.3 (`chown`/`chmod`) |
| Import falha com `Access denied` ou erro de DDL | Credencial `rotaperfumes-db-admin` sem ALL no banco, ou usuário com host `localhost` | Crie o `rotaperfumes_admin@'172.16.0.0/255.240.0.0'` (seção 3.2) |
| `permission denied ... docker.sock` no build | Usuário `jenkins` fora do grupo `docker`, ou Jenkins não reiniciado | Seção 2.2 |
| Erro de schema do compose | Compose < 2.17 | Atualize o plugin `docker-compose-plugin` |
| `Bind for 0.0.0.0:8443 failed: port is already allocated` | Outra aplicação usa a 8443 | `sudo ss -ltnp \| grep 8443`. Libere a porta ou rode o job com outro `HTTPS_PORT` e ajuste `CORS_ALLOWED_ORIGINS` no Secret file |
| Smoke test falha após 30 tentativas | API caiu na subida (env inválido, MySQL inacessível) | O próprio job imprime as últimas 80 linhas de `api`, `frontend` e `caddy`. Veja também a seção 9 |
| `compose` reclama de `api.env` não encontrado em comando manual | Falta `API_ENV_FILE=/dev/null` | Prefixe o comando com a variável (seção 9) |
