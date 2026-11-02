// Pipeline de deploy do rotaperfumes no servidor da LAN (ivo-inspiron-15-3530).
//
// Pre-requisitos no agente (o proprio servidor):
//   - docker + plugin "docker compose" v2;
//     usuario do Jenkins no grupo "docker".
//   - curl, gzip (zcat), install (coreutils), bash, openssl.
//   - SEC-29: o certificado PUBLICO da CA do banco no HOST em
//     DB_TLS_CA_HOST_PATH, modo 644 (docs/Instal_SSL.md). O Jenkins pode rodar
//     em container (Docker do host pelo socket): o arquivo e montado pelo
//     daemon direto do host, nunca passa pelo workspace.
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
        booleanParam(name: 'IMPORTAR_DUMP', defaultValue: false, description: 'Importa o dump (.sql.gz) no MariaDB do servidor - SOBRESCREVE as tabelas')
        string(name: 'DUMP_PATH', defaultValue: '/opt/rotaperfumes/dumps/rotaperfumes.sql.gz', description: 'Caminho do dump no servidor (visivel para o agente Jenkins)')
        // SEC-29: TLS API <-> banco. A CA e lida do HOST a cada deploy (pode
        // ser rotacionada); caminho ABSOLUTO no host, resolvido pelo daemon.
        string(name: 'DB_TLS_CA_HOST_PATH', defaultValue: '/etc/mysql/ssl/ca.pem', description: 'Caminho ABSOLUTO, no HOST, do certificado PUBLICO da CA do banco (ssl_ca), modo 644')
        string(name: 'DB_TLS_SERVER_NAME', defaultValue: '', description: 'Nome conferido no certificado do banco (precisa estar no SAN). Vazio = SITE_IP')
    }

    environment {
        COMPOSE_PROJECT_NAME = 'rotaperfumes'
        GO_IMAGE             = 'golang:1.26-alpine'
        // Cliente do MariaDB 11.4 LTS (mesma versao do servidor); a imagem 11.x
        // so tem o comando "mariadb" (sem o symlink "mysql").
        DB_CLIENT_IMAGE      = 'mariadb:11.4'
        DB_NAME              = 'rotaperfumes'
        // SEC-29: porta do banco no host (producao: MySQL 8.0 na 3306). Usada
        // pelo import e pela conferencia TLS do estagio "CA do banco"; a API
        // usa o DB_PORT do api.env.
        DB_HOST_PORT         = '3306'
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
                // O layout do repositorio e preservado (backend/..., sql/, Makefile):
                // cmdutil.FindProjectRoot procura backend/shared/go.mod e os testes
                // de config leem o Makefile e os .sql da raiz.
                sh '''#!/bin/bash
                    set -euo pipefail
                    tar -cf - backend/shared backend/rotaperfumes-api sql Makefile \
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
                // SEC-29: valida a CA do HOST (DB_TLS_CA_HOST_PATH) dentro de um
                // container descartavel (DB_CLIENT_IMAGE tem bash + openssl),
                // como uid 65532 (o da API), com deploy/validar-ca-banco.sh via
                // stdin: so blocos CERTIFICATE, cada um valido e nao vencido, e
                // s_client -starttls mysql confirmando que a CA valida o
                // certificado servido em SITE_IP:DB_HOST_PORT para o nome
                // conferido. Roda ANTES de parar a API ou importar. O sha256
                // sai em deploy/db-ca.sha256 (label do compose, rotacao).
                sh '''#!/bin/bash
                    set -euo pipefail
                    # Caminho ABSOLUTO no host, sem virgula/espaco (vai dentro do --mount).
                    if ! [[ "$DB_TLS_CA_HOST_PATH" =~ ^/[A-Za-z0-9._/-]+$ ]]; then
                      echo "DB_TLS_CA_HOST_PATH precisa ser caminho ABSOLUTO no host (letras, numeros, . _ - /): $DB_TLS_CA_HOST_PATH" >&2
                      exit 1
                    fi
                    rm -f deploy/db-ca.sha256
                    export TLS_NOME="${DB_TLS_SERVER_NAME:-$SITE_IP}"
                    # --mount (e nao -v): caminho inexistente no host = erro, e
                    # nao um diretorio criado no lugar do arquivo.
                    SHA=$(docker run --rm -i --user 65532:65532 --network bridge \\
                            --mount "type=bind,source=$DB_TLS_CA_HOST_PATH,target=/etc/rotaperfumes/db-ca.pem,readonly" \\
                            -e SITE_IP -e DB_HOST_PORT -e TLS_NOME \\
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

                            # SEC-29: TLS com verificacao do certificado. O cliente
                            # mariadb nao tem opcao de "server name": conecta pelo
                            # proprio nome do SAN. IP (padrao: SITE_IP) e acessado
                            # direto; nome DNS e apontado para o host (host-gateway).
                            TLS_NOME="${DB_TLS_SERVER_NAME:-$SITE_IP}"
                            if ! [[ "$TLS_NOME" =~ ^[A-Za-z0-9.-]+$ ]] || [ "$TLS_NOME" = "0.0.0.0" ]; then
                              echo "DB_TLS_SERVER_NAME invalido: '$TLS_NOME' (hostname ou IPv4 do SAN)" >&2
                              exit 1
                            fi
                            case "$TLS_NOME" in
                              localhost|127.*|::1)
                                echo "DB_TLS_SERVER_NAME=$TLS_NOME nao serve para o import (no container o loopback e o proprio container): use o IP da LAN ou um DNS do SAN" >&2
                                exit 1 ;;
                            esac
                            HOST_ARGS=()
                            if ! [[ "$TLS_NOME" =~ ^[0-9.]+$ ]]; then
                              HOST_ARGS=(--add-host "$TLS_NOME:host-gateway")
                            fi

                            [ -r db-ca.sha256 ] || { echo "Estagio CA do banco nao validou a CA" >&2; exit 1; }

                            mysql_cli() {
                              # -e MYSQL_PWD (sem valor) repassa a variavel do ambiente,
                              # sem expor a senha na linha de comando (o cliente
                              # mariadb le MYSQL_PWD). --local-infile=0: o dump
                              # nao usa LOAD DATA LOCAL. A CA (publica) e a do HOST
                              # (DB_TLS_CA_HOST_PATH), validada no estagio "CA do
                              # banco" e montada read-only pelo daemon.
                              docker run --rm -i \
                                ${HOST_ARGS[@]+"${HOST_ARGS[@]}"} \
                                --mount "type=bind,source=$DB_TLS_CA_HOST_PATH,target=/etc/rotaperfumes/db-ca.pem,readonly" \
                                -e MYSQL_PWD \
                                "$DB_CLIENT_IMAGE" \
                                mariadb --local-infile=0 -h "$TLS_NOME" -P "$DB_HOST_PORT" -u "$DB_ADMIN_USER" \
                                      --ssl-ca=/etc/rotaperfumes/db-ca.pem --ssl-verify-server-cert \
                                      --default-character-set=utf8mb4 "$@"
                            }

                            echo "Importando dump em $DB_NAME..."
                            zcat "$DUMP_PATH" | mysql_cli "$DB_NAME"

                            # Sessoes do ambiente de origem nao valem aqui.
                            mysql_cli "$DB_NAME" -e 'TRUNCATE TABLE refresh_tokens'
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
                    // monta DB_TLS_CA_HOST_PATH (parametro do job, ja no ambiente).
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
