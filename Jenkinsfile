// Pipeline de deploy do rotaperfumes no servidor da LAN (ivo-inspiron-15-3530).
//
// Pre-requisitos no agente (o proprio servidor):
//   - docker + plugin "docker compose" v2;
//     usuario do Jenkins no grupo "docker".
//   - curl, gzip (zcat), install (coreutils), bash, openssl.
//   - DEPLOY-06: banco = container MariaDB 11.4 "rotaperfumes-mariadb" criado
//     por deploy/mariadb-docker.sh, so na rede Docker EXTERNA "rotaperfumes-db"
//     (172.30.42.0/24; MariaDB em 172.30.42.2). A API entra nela com IP fixo
//     172.30.42.10 (docker-compose.yml) e os clientes do pipeline (Importar
//     dump) com 172.30.42.20 (conta rotaperfumes_admin@'172.30.42.20').
//   - SEC-29/SEC-40: o certificado PUBLICO da CA do banco no HOST em
//     /etc/mysql/ssl/ca.pem (constante DB_TLS_CA_HOST_PATH abaixo), modo 644,
//     com o fingerprint SHA-256 listado em deploy/db-ca.fingerprint. O Jenkins
//     pode rodar em container (Docker do host pelo socket): o arquivo e montado
//     pelo daemon direto do host, nunca passa pelo workspace.
// Credenciais Jenkins:
//   - rotaperfumes-api-env   (Secret file)              -> deploy/api.env (ver deploy/api.env.example)
//   - rotaperfumes-db-admin  (Username with password)   -> so para IMPORTAR_DUMP
//
// Regras: nunca "set -x", nunca cat de segredo, nunca "-e SEGREDO=valor".
// Todo "sh" comeca com #!/bin/bash: com shebang o Jenkins NAO aplica o
// "sh -xe" padrao (que ecoaria os comandos).

pipeline {
    agent any

    options {
        disableConcurrentBuilds()
        buildDiscarder(logRotator(numToKeepStr: '20'))
        timeout(time: 60, unit: 'MINUTES')
    }

    // O Jenkins da LAN nao e alcancavel pelo GitHub (sem webhook): consulta a
    // main a cada ~2 min e so roda quando ha commit novo. Builds disparados
    // assim usam os defaultValue dos parametros abaixo.
    triggers {
        pollSCM('H/2 * * * *')
    }

    parameters {
        string(name: 'SITE_HOST', defaultValue: 'ivo-inspiron-15-3530', description: 'Hostname de acesso na LAN')
        string(name: 'SITE_IP', defaultValue: '192.168.168.106', description: 'IP do servidor na LAN (tambem aceito pelo Caddy)')
        string(name: 'HTTPS_PORT', defaultValue: '8443', description: 'Porta HTTPS publicada pelo Caddy')
        string(name: 'TURNSTILE_SITE_KEY', defaultValue: '0x4AAAAAAFACjc-LtM9NWlvP', description: 'Site key PUBLICA do Cloudflare Turnstile (embutida no bundle do frontend)')
        booleanParam(name: 'IMPORTAR_DUMP', defaultValue: false, description: 'Importa o dump (.sql.gz) no MariaDB 11.4 do container rotaperfumes-mariadb (rede rotaperfumes-db) - SOBRESCREVE as tabelas')
        string(name: 'DUMP_PATH', defaultValue: '/opt/rotaperfumes/dumps/rotaperfumes.sql.gz', description: 'Caminho do dump no servidor (visivel para o agente Jenkins)')
    }

    environment {
        COMPOSE_PROJECT_NAME = 'rotaperfumes'
        // SEC-31: imagens fixadas por digest (indice multi-arch) - a tag sozinha
        // pode ser republicada. Como atualizar:
        //   docker buildx imagetools inspect mariadb:11.4 --format '{{.Manifest.Digest}}'
        //   (golang: idem com golang:1.26-alpine; versao do Go = go.mod)
        //   -> trocar "<tag com patch>@sha256:<digest>" aqui; o do mariadb TAMBEM
        //   no IMAGEM_BASE do deploy/mariadb-docker.sh (mesma string) -> make test
        //   -> backup do banco -> sudo bash deploy/mariadb-docker.sh --recriar.
        // "make deps-audit-imagens" mostra se ha digest novo (so informa).
        GO_IMAGE             = 'golang:1.26.10-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308'
        // Cliente do MariaDB 11.4 LTS (mesma imagem do servidor); a imagem 11.x
        // so tem o comando "mariadb" (sem o symlink "mysql").
        DB_CLIENT_IMAGE      = 'mariadb:11.4.13@sha256:d1891ede5a559f1086f55053bb8cc1569b0298338c4d72d71725a7f74f6ba620'
        DB_NAME              = 'rotaperfumes'
        // DEPLOY-06: o banco so e alcancado pela rede Docker rotaperfumes-db,
        // pelo nome do container (que esta no SAN do certificado). Porta = a
        // interna do container (3306), nao uma porta publicada no host.
        DB_REDE              = 'rotaperfumes-db'
        DB_HOST_INTERNO      = 'rotaperfumes-mariadb'
        DB_HOST_PORT         = '3306'
        // IP do cliente do pipeline na rede (conta rotaperfumes_admin@'172.30.42.20').
        DB_CLIENTE_IP        = '172.30.42.20'
        // SEC-40: constante (deixou de ser parametro do job). O compose monta
        // o mesmo caminho na API; o fingerprint da CA e conferido contra
        // deploy/db-ca.fingerprint no estagio "CA do banco".
        DB_TLS_CA_HOST_PATH  = '/etc/mysql/ssl/ca.pem'
        // Sem PUBLIC_URL: o compose usa "/" e o frontend chama "/api/..." na
        // mesma origem, o que vale para a LAN e para o dominio do Cloudflare Tunnel.
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Testes Go') {
            steps {
                // Codigo enviado por stdin (tar) em vez de bind mount: funciona
                // tambem com Jenkins em container. Testes de integracao so rodam
                // com INTEGRATION=1, entao aqui sao pulados (sem MariaDB).
                // O layout do repositorio e preservado (backend/..., sql/, Makefile,
                // deploy/, Jenkinsfile): cmdutil.FindProjectRoot procura
                // backend/shared/go.mod; os testes de config leem o Makefile e os
                // .sql da raiz, e os de tests/deploy e o sec37 leem deploy/* e o
                // Jenkinsfile (falham de proposito se ausentes). Ficam de fora os
                // dumps e o api.env (segredo) do deploy/.
                sh '''#!/bin/bash
                    set -euo pipefail
                    tar -cf - --exclude=deploy/dumps --exclude=deploy/api.env \
                        backend/shared backend/rotaperfumes-api sql Makefile deploy Jenkinsfile \
                      | docker run --rm -i -e CGO_ENABLED=0 -e GOFLAGS=-mod=readonly "$GO_IMAGE" sh -ec '
                          mkdir -p /src && tar -xf - -C /src
                          cd /src/backend/shared && go vet ./... && go test ./...
                          cd /src/backend/rotaperfumes-api && go vet ./... && go test ./...
                        '
                '''
            }
        }

        stage('Build') {
            steps {
                dir('deploy') {
                    sh '''#!/bin/bash
                        set -euo pipefail
                        API_ENV_FILE=/dev/null docker compose -p "$COMPOSE_PROJECT_NAME" build --pull
                    '''
                }
            }
        }

        stage('CA do banco') {
            steps {
                // SEC-29/SEC-40: valida a CA do HOST (DB_TLS_CA_HOST_PATH) dentro de
                // um container descartavel (DB_CLIENT_IMAGE tem bash + openssl),
                // como uid 65532 (o da API), com deploy/validar-ca-banco.sh via
                // stdin: so blocos CERTIFICATE, cada um valido, nao vencido e com
                // fingerprint listado em deploy/db-ca.fingerprint, e s_client
                // -starttls mysql pela rede rotaperfumes-db em
                // rotaperfumes-mariadb:3306 com -verify_hostname rotaperfumes-mariadb.
                // Roda ANTES de parar a API ou importar. O sha256 sai em
                // deploy/db-ca.sha256 (label do compose, rotacao).
                sh '''#!/bin/bash
                    set -euo pipefail
                    rm -f deploy/db-ca.sha256
                    # SEC-40: fingerprints aceitos (versionados); o script falha se
                    # algum certificado do arquivo do host nao estiver na lista.
                    [ -r deploy/db-ca.fingerprint ] || { echo "deploy/db-ca.fingerprint ausente (SEC-40)" >&2; exit 1; }
                    # DEPLOY-07: so comentarios (instalacao nova sem o fingerprint
                    # commitado) = falha com mensagem clara, antes do container.
                    # "|| true": grep sem linha sai 1 e o pipefail derrubaria mudo.
                    FPS=$(grep -Ev '^[[:space:]]*(#|$)' deploy/db-ca.fingerprint || true)
                    if [ -z "$FPS" ]; then
                      echo "deploy/db-ca.fingerprint sem fingerprint: rode mariadb-docker.sh --nova-ca e grave o CA_FP (DEPLOY-07)" >&2
                      exit 1
                    fi
                    echo "Fingerprints SHA-256 aceitos para a CA do banco:"
                    printf '%s\\n' "$FPS" | sed 's/^/  /'
                    # DEPLOY-06: s_client em rotaperfumes-mariadb:3306 pela rede
                    # interna, conferindo o mesmo nome (-verify_hostname).
                    export CA_FINGERPRINTS="$(cat deploy/db-ca.fingerprint)" \\
                           DB_ALVO="$DB_HOST_INTERNO" DB_HOST_PORT=3306 TLS_NOME="$DB_HOST_INTERNO"
                    # --mount (e nao -v): caminho inexistente no host = erro, e
                    # nao um diretorio criado no lugar do arquivo.
                    # DEPLOY-08: --ip $DB_CLIENTE_IP (conta rotaperfumes_admin):
                    # de um IP sem conta o mariadbd manda o erro 1130 antes do
                    # greeting e o s_client nao faz handshake.
                    SHA=$(docker run --rm -i --user 65532:65532 --network "$DB_REDE" --ip "$DB_CLIENTE_IP" \\
                            --mount "type=bind,source=$DB_TLS_CA_HOST_PATH,target=/etc/rotaperfumes/db-ca.pem,readonly" \\
                            -e DB_ALVO -e DB_HOST_PORT -e TLS_NOME -e CA_FINGERPRINTS \\
                            --entrypoint bash "$DB_CLIENT_IMAGE" -s < deploy/validar-ca-banco.sh)
                    [[ "$SHA" =~ ^[0-9a-f]{64}$ ]] || { echo "CA do banco: sha256 inesperado da validacao" >&2; exit 1; }
                    echo "$SHA" > deploy/db-ca.sha256
                    echo "CA do banco validada: $DB_TLS_CA_HOST_PATH (sha256 $SHA)"
                '''
            }
        }

        stage('Importar dump') {
            when { expression { params.IMPORTAR_DUMP } }
            steps {
                withCredentials([usernamePassword(credentialsId: 'rotaperfumes-db-admin',
                                                  usernameVariable: 'DB_ADMIN_USER',
                                                  passwordVariable: 'MYSQL_PWD')]) {
                    dir('deploy') {
                        sh '''#!/bin/bash
                            set -euo pipefail
                            if [ ! -r "$DUMP_PATH" ]; then
                              echo "Dump nao encontrado/legivel: $DUMP_PATH" >&2
                              exit 1
                            fi
                            gzip -t "$DUMP_PATH"

                            # Para a API durante o import (o dump faz DROP/CREATE TABLE).
                            API_ENV_FILE=/dev/null docker compose -p "$COMPOSE_PROJECT_NAME" stop api || true

                            [ -r db-ca.sha256 ] || { echo "Estagio CA do banco nao validou a CA" >&2; exit 1; }

                            mariadb_cli() {
                              # DEPLOY-06: pela rede rotaperfumes-db com o IP fixo
                              # do cliente (172.30.42.20, o da conta admin), no
                              # nome do container (no SAN): TLS verificado.
                              # -e MYSQL_PWD (sem valor) repassa a variavel do
                              # ambiente, sem expor a senha na linha de comando
                              # (o cliente mariadb le MYSQL_PWD). --local-infile=0:
                              # o dump nao usa LOAD DATA LOCAL. A CA (publica) e a
                              # do HOST, validada no estagio "CA do banco" e
                              # montada read-only pelo daemon.
                              docker run --rm -i \
                                --network "$DB_REDE" --ip "$DB_CLIENTE_IP" \
                                --mount "type=bind,source=$DB_TLS_CA_HOST_PATH,target=/etc/rotaperfumes/db-ca.pem,readonly" \
                                -e MYSQL_PWD \
                                "$DB_CLIENT_IMAGE" \
                                mariadb --local-infile=0 -h "$DB_HOST_INTERNO" -P "$DB_HOST_PORT" -u "$DB_ADMIN_USER" \
                                      --ssl-ca=/etc/rotaperfumes/db-ca.pem --ssl-verify-server-cert \
                                      --default-character-set=utf8mb4 "$@"
                            }

                            echo "Importando dump em $DB_NAME..."
                            zcat "$DUMP_PATH" | mariadb_cli "$DB_NAME"

                            # Sessoes do ambiente de origem nao valem aqui.
                            mariadb_cli "$DB_NAME" -e 'TRUNCATE TABLE refresh_tokens'
                            echo "Import concluido."
                        '''
                    }
                }
            }
        }

        stage('Deploy') {
            steps {
                withCredentials([file(credentialsId: 'rotaperfumes-api-env', variable: 'API_ENV')]) {
                    sh '''#!/bin/bash
                        set -euo pipefail
                        install -m 600 "$API_ENV" deploy/api.env
                    '''
                }
                dir('deploy') {
                    // SEC-29 (rotacao da CA): o hash (do estagio "CA do banco")
                    // vira label do servico api; CA nova = label nova = container
                    // recriado (o bind de um arquivo fica preso ao inode antigo e
                    // o "up" nao recria so por mudanca de conteudo). O compose
                    // monta DB_TLS_CA_HOST_PATH (constante do environment, SEC-40)
                    // e liga a API na rede externa rotaperfumes-db (DEPLOY-06).
                    sh '''#!/bin/bash
                        set -euo pipefail
                        [ -r db-ca.sha256 ] || { echo "deploy/db-ca.sha256 ausente (estagio CA do banco)" >&2; exit 1; }
                        DB_CA_SHA256=$(cat db-ca.sha256)
                        export DB_CA_SHA256
                        echo "CA do banco no deploy: sha256 $DB_CA_SHA256"
                        docker compose -p "$COMPOSE_PROJECT_NAME" up -d --remove-orphans
                    '''
                }
            }
        }

        stage('Smoke test') {
            steps {
                // Acessa direto pelo IP da LAN (o Caddy aceita {$SITE_IP} e usa
                // default_sni para o IP): o hostname fica bloqueado no smoke test.
                sh '''#!/bin/bash
                    set -uo pipefail
                    BASE="https://${SITE_IP}:${HTTPS_PORT}"

                    check() {
                      curl -fsSk --max-time 5 -o /dev/null -w "%{http_code}" "$BASE$1"
                    }

                    # Como o cloudflared acessa (visto nos logs do Caddy): IP da
                    # LAN, sem SNI, com o Host do dominio publico -> site curinga.
                    # Nao use localhost: o Jenkins roda em container e o
                    # loopback dele nao e o do servidor.
                    tunnel() {
                      curl -fsSk --max-time 5 -o /dev/null -w "%{http_code}" \
                        -H "Host: smoke-tunnel.invalid" "$BASE$1"
                    }

                    for i in $(seq 1 30); do
                      api=$(check /api/health || true)
                      front=$(check /login || true)
                      tun=$(tunnel /api/health || true)
                      if [ "$api" = "200" ] && [ "$front" = "200" ] && [ "$tun" = "200" ]; then
                        echo "Smoke test OK: /api/health=$api /login=$front tunnel=$tun"
                        exit 0
                      fi
                      echo "Aguardando servicos (tentativa $i/30): /api/health=${api:-erro} /login=${front:-erro} tunnel=${tun:-erro}"
                      sleep 3
                    done

                    echo "Smoke test FALHOU. Ultimos logs:" >&2
                    cd deploy && API_ENV_FILE=/dev/null docker compose -p "$COMPOSE_PROJECT_NAME" logs --tail 40 api frontend caddy >&2 || true
                    exit 1
                '''
            }
        }
    }

    post {
        always {
            sh '''#!/bin/bash
                rm -f deploy/api.env deploy/db-ca.sha256
            '''
        }
    }
}
