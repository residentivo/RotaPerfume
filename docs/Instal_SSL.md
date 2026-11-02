# Instalação do TLS (SSL) no banco de produção: passo a passo

> Card **SEC-29** (Lote 29, 2026-11-01). Este roteiro é para **você** executar no **servidor de produção** (`ivo-inspiron-15-3530`, `192.168.168.106`). A equipe não tem acesso a ele.
>
> **Leia antes:** todos os comandos são rodados **no HOST** (no shell do servidor, via SSH ou terminal local), **nunca dentro de um container**, nem do Jenkins nem da API. Os containers só leem os arquivos que o host deixar legíveis.

## 0. Contexto e ordem

| Ambiente | Banco | Porta | Onde roda | Certificado |
|---|---|---|---|---|
| **Produção** | **MySQL 8.0.46** (`8.0.46-0ubuntu0.24.04.4`, pacote apt) | 3306 | nativo no host | hoje é o autogerado (`CN=MySQL_Server_8.0.46_Auto_Generated_Server_Certificate`), **sem SAN** |
| Dev | MariaDB 11.8.8 | 3307 | container do CasaOS | já tem TLS (`/DATA/AppData/mariadb/ssl`), ver passo 11 |

> **Divergência registrada:** `docs/deploy-servidor.md` (seção 3), `deploy/setup-servidor.sh` e `deploy/mariadb-setup.sql` falam em **MariaDB 11.4**, mas a produção é **MySQL 8.0**. Os comandos deste roteiro são de MySQL 8.0. Alinhar o restante fica para um card próprio.

**Por que um certificado novo?** A API (Go) só aceita certificado que tenha o nome do servidor no **SAN** (Subject Alternative Name). O autogerado do MySQL não tem SAN, e o Go o recusa (`x509: certificate relies on legacy Common Name field` ou `self-signed certificate`). Resultado atual, conferido em 2026-11-01 (**antes**):

```
$ openssl s_client -starttls mysql -connect 192.168.168.106:3306 -verify_return_error -verify_ip 192.168.168.106
verify error:num=19:self-signed certificate in certificate chain
Verify return code: 19 (self-signed certificate in certificate chain)
subject=CN = MySQL_Server_8.0.46_Auto_Generated_Server_Certificate   (No extensions in certificate)
```

**Ordem obrigatória.** O `Jenkinsfile` do SEC-29 **valida a CA a cada build** (estágio `CA do banco`) e para o deploy se ela não validar o certificado servido na 3306. Como o job roda sozinho a cada commit no `main` (`pollSCM`), faça os passos 1 a 6 **antes** de levar o SEC-29 para o `main`. Senão os deploys automáticos param no estágio `CA do banco`. Eles param antes de tocar na API, então a versão atual continua no ar.

---

## 1. (a) Gerar a CA e o certificado do servidor

Gere numa pasta temporária do servidor, só de root:

```bash
sudo -i                                   # vira root NO HOST
umask 077
mkdir -p /root/ssl-rotaperfumes && cd /root/ssl-rotaperfumes

# 1.1 CA própria (10 anos). A chave da CA (ca-key.pem) é a peça mais sensível.
openssl genrsa -out ca-key.pem 3072
openssl req -x509 -new -key ca-key.pem -sha256 -days 3650 \
  -subj "/CN=RotaPerfumes_MySQL_CA" \
  -addext "basicConstraints=critical,CA:TRUE" \
  -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -out ca.pem

# 1.2 Chave e pedido do servidor
openssl genrsa -out server-key.pem 3072
openssl req -new -key server-key.pem -subj "/CN=RotaPerfumes MySQL Server" -out server.csr

# 1.3 Certificado do servidor (2 anos) com SAN.
#     host.docker.internal / 172.17.0.1 = como os containers enxergam o host
#     (confira o gateway com: ip -4 addr show docker0).
cat > server-ext.cnf <<'EOF'
basicConstraints = critical, CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = IP:192.168.168.106, IP:127.0.0.1, IP:172.17.0.1, DNS:localhost, DNS:host.docker.internal, DNS:ivo-inspiron-15-3530
EOF
openssl x509 -req -in server.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial \
  -sha256 -days 730 -extfile server-ext.cnf -out server-cert.pem

# 1.4 Conferências
openssl verify -CAfile ca.pem server-cert.pem                 # server-cert.pem: OK
openssl x509 -in server-cert.pem -noout -subject -enddate -ext subjectAltName
```

Se o IP do `docker0` não for `172.17.0.1`, troque no `server-ext.cnf` antes do passo 1.3.

## 2. (b) Instalar em `/etc/mysql/ssl`

```bash
install -d -o root -g root -m 755 /etc/mysql/ssl
install -o root  -g root  -m 644 ca.pem          /etc/mysql/ssl/ca.pem
install -o root  -g root  -m 644 server-cert.pem /etc/mysql/ssl/server-cert.pem
install -o mysql -g mysql -m 600 server-key.pem  /etc/mysql/ssl/server-key.pem
ls -l /etc/mysql/ssl
```

Modos esperados: diretório `755`; `ca.pem` e `server-cert.pem` com `644` (públicos); `server-key.pem` com `600` e dono `mysql`.

**AppArmor (Ubuntu).** O perfil `/etc/apparmor.d/usr.sbin.mysqld` já permite ao `mysqld` ler `/etc/mysql/**`. Confira:

```bash
grep -n '/etc/mysql' /etc/apparmor.d/usr.sbin.mysqld
```

Se usar outro diretório, acrescente-o em `/etc/apparmor.d/local/usr.sbin.mysqld` (ex.: `/srv/mysql-ssl/** r,`) e rode `apparmor_parser -r /etc/apparmor.d/usr.sbin.mysqld`. Sem isso, o MySQL não sobe e o `error.log` mostra `Permission denied` ao abrir o certificado.

**A chave da CA (`ca-key.pem`) não fica no servidor.** Copie-a para um lugar seguro fora dele (pendrive cifrado ou gerenciador de senhas, junto com `ca.pem` e `server-ext.cnf`) e apague a pasta temporária:

```bash
# na SUA máquina:  scp root@192.168.168.106:/root/ssl-rotaperfumes/{ca-key.pem,ca.pem,server-ext.cnf} <destino seguro>
shred -u /root/ssl-rotaperfumes/ca-key.pem /root/ssl-rotaperfumes/server-key.pem
rm -rf /root/ssl-rotaperfumes
```

Ela só é necessária para emitir um certificado de servidor novo (passo 9).

## 3. (c) Configurar o MySQL

Crie `/etc/mysql/mysql.conf.d/zz-rotaperfumes-ssl.cnf`:

```bash
cat > /etc/mysql/mysql.conf.d/zz-rotaperfumes-ssl.cnf <<'EOF'
# SEC-29: TLS obrigatório com certificado próprio (SAN). Ver docs/Instal_SSL.md.
[mysqld]
ssl_ca   = /etc/mysql/ssl/ca.pem
ssl_cert = /etc/mysql/ssl/server-cert.pem
ssl_key  = /etc/mysql/ssl/server-key.pem
require_secure_transport = ON
tls_version = TLSv1.2,TLSv1.3
EOF
chmod 644 /etc/mysql/mysql.conf.d/zz-rotaperfumes-ssl.cnf
systemctl restart mysql
systemctl status mysql --no-pager
```

`require_secure_transport = ON` faz o servidor recusar **qualquer** conexão TCP sem TLS. O socket local (`sudo mysql`) continua funcionando.

**Se não subir:**

```bash
journalctl -u mysql -n 50 --no-pager
tail -n 50 /var/log/mysql/error.log
```

| Mensagem no log | Causa e correção |
|---|---|
| `Permission denied` em `server-key.pem` | dono/modo errado (passo 2) ou AppArmor |
| `Failed to set up SSL because of the following SSL library error: ... key values mismatch` | `server-key.pem` não é a chave de `server-cert.pem`: refaça o passo 1.3 |
| `unknown variable` | erro de digitação no drop-in |

Para voltar rápido, veja o passo 10 (rollback).

## 4. (d) Verificar

No servidor (o cliente `mysql` do Ubuntu):

```bash
mysql --ssl-mode=VERIFY_IDENTITY --ssl-ca=/etc/mysql/ssl/ca.pem -h 192.168.168.106 -P 3306 -u <seu_usuario> -p \
  -e "SHOW VARIABLES LIKE '%ssl%'; SHOW VARIABLES LIKE 'tls_version'; SHOW VARIABLES LIKE 'require_secure_transport'; SHOW STATUS LIKE 'Ssl_version'; SHOW STATUS LIKE 'Ssl_cipher';"
```

Esperado: `ssl_ca`, `ssl_cert` e `ssl_key` apontando para `/etc/mysql/ssl/...`, `require_secure_transport = ON` e `Ssl_version` igual a `TLSv1.3` (ou `TLSv1.2`). `VERIFY_IDENTITY` confere a CA **e** o nome `192.168.168.106` no SAN.

Sem TLS deve falhar: `mysql --ssl-mode=DISABLED -h 192.168.168.106 -u <usuario> -p` dá `Connections using insecure transport are prohibited`.

De **outra máquina** da LAN, com uma cópia do `ca.pem`:

```bash
openssl s_client -starttls mysql -connect 192.168.168.106:3306 -CAfile ca.pem -verify_return_error -verify_ip 192.168.168.106 </dev/null | grep 'Verify return code'
# Verify return code: 0 (ok)
```

> Em 2026-11-01 a porta 3306 de produção respondeu a partir de `192.168.168.105` (outra máquina da LAN). A seção 3.4 do `docs/deploy-servidor.md` prevê bloquear a 3306 para a LAN (`ufw deny 3306`, liberando só `172.16.0.0/12`). Depois do teste acima, confira com `sudo ufw status numbered`.

## 5. (e) Permissão para a API e o Jenkins lerem a CA (no HOST)

A API roda em imagem distroless com **uid 65532**. O compose monta o arquivo do host direto no container (`/etc/rotaperfumes/db-ca.pem`, read-only), e o estágio `CA do banco` do Jenkins o valida num container descartável que também roda como uid 65532. O uid 65532 não existe no host. Por isso o arquivo precisa ser legível por **"outros"** (`644`) e os diretórios, atravessáveis (`755`).

**Isto é feito no HOST, não dentro de nenhum container** (nem do Jenkins nem da API):

```bash
sudo chmod 755 /etc/mysql /etc/mysql/ssl
sudo chmod 644 /etc/mysql/ssl/ca.pem
sudo -u nobody cat /etc/mysql/ssl/ca.pem >/dev/null && echo "legivel por outros: OK"
```

Só o `ca.pem` (público) é montado. As chaves continuam `600`.

## 6. (f) Jenkins: parâmetros do job e Secret file

O Jenkins roda em container e usa o Docker do host pelo socket. O caminho abaixo é do **host**, e quem o resolve é o daemon do host, não o container do Jenkins (que não precisa enxergar `/etc/mysql`).

| Parâmetro | Valor | Observação |
|---|---|---|
| `DB_TLS_CA_HOST_PATH` | `/etc/mysql/ssl/ca.pem` | padrão; caminho **absoluto no host** |
| `DB_TLS_SERVER_NAME` | vazio | = `SITE_IP` (`192.168.168.106`, que está no SAN). Com o certificado novo, `host.docker.internal` também serviria |

No **Secret file** `rotaperfumes-api-env` (modelo: `deploy/api.env.example`), confira:

```
DB_HOST=host.docker.internal
DB_PORT=3306
```

`DB_TLS_CA` e `DB_TLS_SERVER_NAME` **não** precisam estar no Secret file: o `docker-compose.yml` os define (`DB_TLS_CA=/etc/rotaperfumes/db-ca.pem`) e o valor do compose ganha.

## 7. (g) Primeiro deploy e conferência

1. Leve o SEC-29 para o `main` (só depois dos passos 1 a 6).
2. Rode o job (**Construir com parâmetros**, padrões). O estágio `CA do banco` deve imprimir os certificados, `s_client: Verify return code: 0 (ok)` e `CA do banco validada: /etc/mysql/ssl/ca.pem (sha256 ...)`.
3. Confira o log da API **no host**, na pasta `deploy/` do workspace (ou em qualquer pasta com o `docker-compose.yml`):
   ```bash
   docker logs rotaperfumes-api-1 2>&1 | grep -E 'TLS do banco|conexão MariaDB estabelecida'
   # [server] TLS do banco: ligado (ca=/etc/rotaperfumes/db-ca.pem, server_name=192.168.168.106, min=TLS1.2)
   # [db] conexão MariaDB estabelecida (host=..., tls=ligado (custom))
   ```
   O texto diz "MariaDB" por herança; a conexão é com o MySQL 8.0.

Se o estágio `CA do banco` falhar, a mensagem diz o motivo (arquivo ilegível pelo uid 65532, bloco que não é certificado, CA vencida, CA que não valida o certificado servido ou nome fora do SAN), e nada foi alterado.

## 8. (h) `REQUIRE SSL` nos usuários (último passo obrigatório)

Só **depois** de ver `tls=ligado (custom)` no log (passo 7). No servidor:

```bash
sudo mysql
```

```sql
-- 8.1 Hosts reais dos usuários (use exatamente o que aparecer aqui)
SELECT user, host, plugin, ssl_type FROM mysql.user WHERE user LIKE 'rotaperfumes%';

-- 8.2 Exigir TLS (troque <host> pelo valor da coluna host do SELECT acima,
--     ex.: 172.16.0.0/255.240.0.0)
ALTER USER 'rotaperfumes_app'@'<host>' REQUIRE SSL;
ALTER USER 'rotaperfumes_admin'@'<host>' REQUIRE SSL;

-- 8.3 Conferência final: ssl_type = ANY nos dois
SELECT user, host, ssl_type FROM mysql.user WHERE user LIKE 'rotaperfumes%';
SHOW CREATE USER 'rotaperfumes_app'@'<host>';    -- termina com REQUIRE SSL
SHOW CREATE USER 'rotaperfumes_admin'@'<host>';
```

Se algum usuário tiver mais de uma linha (hosts diferentes), aplique o `ALTER USER` em cada uma. Rode o job de novo e confira o smoke test verde. Ao recriar usuários (`deploy/mariadb-setup.sql`, que ainda não traz `REQUIRE SSL`; fica para um card futuro), repita este passo.

**Cliente do import.** O estágio `Importar dump` usa o cliente `mariadb` (imagem `mariadb:11.4`) com `--ssl-ca --ssl-verify-server-cert` contra este MySQL 8.0. O MariaDB Connector/C suporta o plugin `caching_sha2_password` do MySQL 8 e, com TLS, a senha vai pelo canal cifrado. **Isso não foi testado pela equipe** (sem acesso ao MySQL de produção). Teste uma vez antes de precisar do import, **no host**:

```bash
docker run --rm -it --mount type=bind,source=/etc/mysql/ssl/ca.pem,target=/ca.pem,readonly mariadb:11.4 \
  mariadb -h 192.168.168.106 -P 3306 -u rotaperfumes_admin -p --ssl-ca=/ca.pem --ssl-verify-server-cert \
  -e "SHOW STATUS LIKE 'Ssl_version'"
```

Se aparecer `Authentication plugin 'caching_sha2_password' cannot be loaded`, troque `DB_CLIENT_IMAGE` no `Jenkinsfile` por `mysql:8.0`. O cliente `mysql` usa `--ssl-mode=VERIFY_IDENTITY --ssl-ca=...` no lugar de `--ssl-verify-server-cert`, então o `mysql_cli` precisa desse ajuste, e o estágio `CA do banco` precisa de uma imagem com `openssl`.

## 9. (i) Rotação

**Só o certificado do servidor (mesma CA; ex.: a cada 2 anos).** Precisa da `ca-key.pem` guardada fora do servidor:

1. Numa pasta temporária de root, gere `server-key.pem`, `server.csr` e `server-cert.pem` como nos passos 1.2 e 1.3 (o `server-ext.cnf` guardado garante o mesmo SAN).
2. Instale como no passo 2 (mesmos modos) e recarregue sem reiniciar: `sudo mysql -e "ALTER INSTANCE RELOAD TLS"` (ou `systemctl restart mysql`).
3. Confira com o passo 4. **Nada muda no Jenkins:** a CA é a mesma, então o sha256 também, e a API não é recriada (conexões novas já pegam o certificado novo).
4. Apague de novo a `ca-key.pem` do servidor.

**Troca da CA (ex.: a cada 10 anos ou se a `ca-key.pem` vazar), sem queda:**

1. Gere a CA nova e um certificado de servidor novo assinado por ela (passo 1), sem instalar ainda.
2. No host, deixe as **duas** CAs no arquivo que a API lê: `cat ca.pem-antiga ca.pem-nova > /etc/mysql/ssl/ca.pem` (modo 644). Rode o job: o estágio valida os dois certificados, a conferência passa (a CA antiga ainda assina o certificado em uso), o sha256 muda e a API é **recriada** confiando nas duas.
3. Instale `server-cert.pem` e `server-key.pem` novos e rode `ALTER INSTANCE RELOAD TLS`.
4. Deixe só a CA nova em `/etc/mysql/ssl/ca.pem` e rode o job de novo (a API é recriada só com ela).

Se o `ca.pem` for trocado **antes** do certificado do servidor, o estágio `CA do banco` barra o job (a CA nova não valida o certificado antigo), e a API continua no ar com a CA anterior. Faltando 30 dias para o vencimento de qualquer certificado do arquivo, o estágio imprime um aviso.

## 10. (j) Rollback

No servidor:

```bash
sudo mysql -e "ALTER USER 'rotaperfumes_app'@'<host>' REQUIRE NONE; ALTER USER 'rotaperfumes_admin'@'<host>' REQUIRE NONE;"
sudo rm /etc/mysql/mysql.conf.d/zz-rotaperfumes-ssl.cnf
sudo systemctl restart mysql          # volta ao certificado autogerado e aceita conexão sem TLS
```

A API do SEC-29 **sempre** exige TLS verificado (o compose fixa `DB_TLS_CA`) e não aceita o certificado autogerado. Para ela voltar a funcionar sem o certificado próprio, reverta o SEC-29 no `main` (`git revert <commit>`) e rode o job: a versão anterior conecta sem TLS. Os arquivos em `/etc/mysql/ssl` podem ficar (não atrapalham).

## 11. (k) DEV: MariaDB na 3307 (CasaOS)

- O MariaDB 11.8.8 de desenvolvimento roda em container na porta **3307** do mesmo servidor, com TLS obrigatório (`require_secure_transport=ON`). Os certificados ficam no **host** em `/DATA/AppData/mariadb/ssl` e já funcionam (SAN: `IP:192.168.168.106`, `IP:127.0.0.1`, `DNS:localhost`; CA `CN=MariaDB_CA`).
- O `deploy/ca-cert.pem` do repositório (não versionado, está no `.gitignore`) é a CA desse MariaDB. Para atualizar a cópia na sua máquina:
  ```bash
  scp <usuario>@192.168.168.106:/DATA/AppData/mariadb/ssl/ca-cert.pem deploy/ca-cert.pem
  openssl x509 -in deploy/ca-cert.pem -noout -subject -enddate   # subject=CN = MariaDB_CA
  ```
  (Se o arquivo da CA tiver outro nome nessa pasta, use o que contém `CN=MariaDB_CA`.)
- No `.env` local:
  ```
  DB_HOST=192.168.168.106
  DB_PORT=3307
  DB_TLS_CA=deploy/ca-cert.pem
  ```
  O caminho relativo vale a partir da raiz do projeto, para `make dev-api`, `go test` e `make db-*`. Conferência: o log de `make dev-api` mostra `tls=ligado (custom)`, e `make db-*` passa `--ssl-ca --ssl-verify-server-cert` ao cliente `mariadb`.
