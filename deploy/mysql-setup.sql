-- ===========================================================================
-- Setup do MySQL do SERVIDOR (host, fora do Docker) - rodar UMA vez, como root:
--
--   sudo mysql < deploy/mysql-setup.sql
--   (antes, troque <TROCAR> por uma senha forte:  openssl rand -base64 32 ;
--    evite "$", "#", aspas e espacos - ver api.env.example)
--
-- Rede: os containers acessam o MySQL do host via host.docker.internal
-- (host-gateway = IP da bridge docker0, normalmente 172.17.0.1). O compose
-- cria redes proprias em 172.16.0.0/12 (172.18.x, 172.19.x...), por isso o
-- usuario e liberado para a faixa 172.16.0.0/255.240.0.0.
--
-- bind-address: por padrao o MySQL do Ubuntu escuta so em 127.0.0.1, e os
-- containers NAO alcancam 127.0.0.1 do host. Em
-- /etc/mysql/mysql.conf.d/mysqld.cnf, secao [mysqld], use UMA das opcoes:
--   bind-address = 0.0.0.0          (todas as interfaces; bloqueie 3306 no
--                                    firewall para a LAN: sudo ufw deny 3306)
--   bind-address = 127.0.0.1,172.17.0.1   (MySQL >= 8.0.13: so loopback e
--                                    docker0; o MySQL precisa subir depois
--                                    do Docker, senao falha ao fazer bind)
-- e reinicie:  sudo systemctl restart mysql
-- Se usar ufw, libere a origem docker:  sudo ufw allow from 172.16.0.0/12 to any port 3306 proto tcp
-- ===========================================================================

CREATE DATABASE IF NOT EXISTS rotaperfumes
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

-- Usuario da API: somente DML. A API nao cria/altera tabelas, nao usa
-- tabelas temporarias, LOCK TABLES, procedures, views nem triggers; os
-- "SELECT ... FOR UPDATE" precisam apenas de SELECT + UPDATE.
CREATE USER IF NOT EXISTS 'rotaperfumes_app'@'172.16.0.0/255.240.0.0'
  IDENTIFIED BY '<TROCAR>';

GRANT SELECT, INSERT, UPDATE, DELETE
  ON rotaperfumes.*
  TO 'rotaperfumes_app'@'172.16.0.0/255.240.0.0';

-- Usuario ADMIN para importar o dump / migracoes (credencial Jenkins
-- "rotaperfumes-db-admin"). O import roda num container mysql:8 que tambem
-- sai pela rede docker, por isso a mesma faixa. Precisa de DDL porque o dump
-- faz DROP/CREATE TABLE. Descomente e troque a senha se ainda nao tiver um.
--
-- CREATE USER IF NOT EXISTS 'rotaperfumes_admin'@'172.16.0.0/255.240.0.0'
--   IDENTIFIED BY '<TROCAR_ADMIN>';
-- GRANT ALL PRIVILEGES ON rotaperfumes.* TO 'rotaperfumes_admin'@'172.16.0.0/255.240.0.0';

FLUSH PRIVILEGES;
