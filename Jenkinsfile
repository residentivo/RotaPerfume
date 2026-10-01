// Pipeline de deploy do rotaperfumes no servidor da LAN (ivo-inspiron-15-3530).
//
// Pre-requisitos no agente (o proprio servidor):
//   - docker + plugin "docker compose" v2;
//     usuario do Jenkins no grupo "docker".
//   - curl, gzip (zcat), install (coreutils), bash.
// Credenciais Jenkins:
//   - rotaperfumes-api-env   (Secret file)              -> deploy/api.env (ver deploy/api.env.example)
//   - rotaperfumes-admin  (Username with password)   -> so para IMPORTAR_DUMP
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

    parameters {
        string(name: 'SITE_HOST', defaultValue: 'ivo-inspiron-15-3530', description: 'Hostname de acesso na LAN')
        string(name: 'SITE_IP', defaultValue: '192.168.168.106', description: 'IP do servidor na LAN (tambem aceito pelo Caddy)')
        string(name: 'HTTPS_PORT', defaultValue: '8443', description: 'Porta HTTPS publicada pelo Caddy')
        string(name: 'TURNSTILE_SITE_KEY', defaultValue: '', description: 'Site key PUBLICA do Cloudflare Turnstile (embutida no bundle do frontend)')
        booleanParam(name: 'IMPORTAR_DUMP', defaultValue: false, description: 'Importa o dump (.sql.gz) no MySQL do servidor - SOBRESCREVE as tabelas')
        string(name: 'DUMP_PATH', defaultValue: '/opt/rotaperfumes/dumps/rotaperfumes.sql.gz', description: 'Caminho do dump no servidor (visivel para o agente Jenkins)')
    }

    environment {
        COMPOSE_PROJECT_NAME = 'rotaperfumes'
        GO_IMAGE             = 'golang:1.26-alpine'
        MYSQL_CLIENT_IMAGE   = 'mysql:8.4'
        DB_NAME              = 'rotaperfumes'
        // Origem publica SEM /api: o frontend monta "${API_BASE}/api/...".
        PUBLIC_URL           = "https://${params.SITE_HOST}:${params.HTTPS_PORT}"
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
                // com INTEGRATION=1, entao aqui sao pulados (sem MySQL).
                // O layout do repositorio e preservado (apis/..., sql/, Makefile):
                // cmdutil.FindProjectRoot procura apis/shared/go.mod e os testes
                // de config leem o Makefile e os .sql da raiz.
                sh '''#!/bin/bash
                    set -euo pipefail
                    tar -cf - apis/shared apis/rotaperfumes-api sql Makefile \
                      | docker run --rm -i -e CGO_ENABLED=0 -e GOFLAGS=-mod=readonly "$GO_IMAGE" sh -ec '
                          mkdir -p /src && tar -xf - -C /src
                          cd /src/apis/shared && go vet ./... && go test ./...
                          cd /src/apis/rotaperfumes-api && go vet ./... && go test ./...
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

        stage('Importar dump') {
            when { expression { params.IMPORTAR_DUMP } }
            steps {
                withCredentials([usernamePassword(credentialsId: 'rotaperfumes-admin',
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

                            mysql_cli() {
                              # -e MYSQL_PWD (sem valor) repassa a variavel do ambiente,
                              # sem expor a senha na linha de comando.
                              docker run --rm -i \
                                --add-host host.docker.internal:host-gateway \
                                -e MYSQL_PWD \
                                "$MYSQL_CLIENT_IMAGE" \
                                mysql -h host.docker.internal -u "$DB_ADMIN_USER" \
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
                    sh '''#!/bin/bash
                        set -euo pipefail
                        docker compose -p "$COMPOSE_PROJECT_NAME" up -d --remove-orphans
                    '''
                }
            }
        }

        stage('Smoke test') {
            steps {
                // --resolve: acessa pelo hostname real (SNI/Host corretos) no IP da LAN.
                sh '''#!/bin/bash
                    set -uo pipefail
                    BASE="https://${SITE_HOST}:${HTTPS_PORT}"
                    RESOLVE="${SITE_HOST}:${HTTPS_PORT}:${SITE_IP}"

                    check() {
                      curl -fsSk --max-time 5 --resolve "$RESOLVE" -o /dev/null -w "%{http_code}" "$BASE$1"
                    }

                    for i in $(seq 1 30); do
                      api=$(check /api/health || true)
                      front=$(check /login || true)
                      if [ "$api" = "200" ] && [ "$front" = "200" ]; then
                        echo "Smoke test OK: /api/health=$api /login=$front"
                        exit 0
                      fi
                      echo "Aguardando servicos (tentativa $i/30): /api/health=${api:-erro} /login=${front:-erro}"
                      sleep 3
                    done

                    echo "Smoke test FALHOU. Ultimos logs:" >&2
                    cd deploy && API_ENV_FILE=/dev/null docker compose -p "$COMPOSE_PROJECT_NAME" logs --tail 80 api frontend caddy >&2 || true
                    exit 1
                '''
            }
        }
    }

    post {
        always {
            sh '''#!/bin/bash
                rm -f deploy/api.env
            '''
        }
    }
}
