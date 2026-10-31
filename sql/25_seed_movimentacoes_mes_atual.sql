-- ============================================================
-- SEED: movimentações de exemplo no MÊS CORRENTE (DB-05)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-10-31
-- Agente: DataBrain (🌸)
-- Uso: make db-seed-mes-atual
--
-- O que gera (mês = o de CURDATE() no momento da execução, nunca fixo;
-- datas de movimento entre o dia 1 e o ÚLTIMO DIA do mês, @ref = LAST_DAY):
--   - 1000 pedidos com itens (1 a 8 por pedido, produtos ativos, preço de
--     tabela ou 5% abaixo, desconto 0/5/10/15%). valor_bruto e valor_total
--     seguem a regra do domínio (vendas.novoItem): bruto =
--     ROUND(qtd * preço * (1 - desconto/100), 2) e total = soma dos brutos.
--     Todos os status e canais. O vendedor do pedido é o da carteira ativa
--     do cliente (só clientes ativos de vendedores não desligados).
--   - 1 pagamento por pedido não cancelado (mesmo padrão do CSV), com as
--     7 formas, prazo/taxa/parcelas do CSV (Boleto 14/28 dias, Cheque 35,
--     Cartão crédito 1 dia/3,20%/1-6x, débito 1 dia/1,80%, Dinheiro/PIX no
--     dia), valor = valor_total e valor_liquido = ROUND(valor*(1-taxa/100),2).
--     Status coerentes com @ref (último dia do mês): vencimento >= @ref →
--     Em aberto ou Pago antecipado; vencimento < @ref → Pago, Pago com atraso
--     (pagamento depois do vencimento, no máximo @ref) ou Inadimplente. data_pagamento NULL em
--     Em aberto/Inadimplente. Vencimentos de boleto/cheque podem cair no mês
--     seguinte (o pedido fica no mês corrente).
--   - 700 oportunidades (6 etapas, 7 origens, 8 motivos de perda; fechadas
--     com data_fechamento <= @ref e ciclo_dias coerente; abertas com
--     data_fechamento/ciclo_dias/motivo_perda NULL).
--   - 700 visitas (os 5 resultados existentes; duração 10 a 90 min).
--   - Snapshots de estoque nos dias 1, 8, 15, 22 e 29 (os que o mês tiver)
--     para TODOS os SKUs, com e sem ruptura (ruptura = saldo 0).
--   - 100 transferências de carteira no mês: 50 simples (vínculo importado
--     encerrado + 1 vínculo novo ativo) e 50 duplas (vínculo importado
--     encerrado + 1 vínculo novo encerrado no mês + 1 vínculo novo ativo).
--     Sempre 1 vínculo ativo por cliente. Esses clientes ficam fora dos
--     pedidos/visitas/oportunidades gerados (evita vendedor incoerente).
--
-- Idempotência (sem alterar schema):
--   MARCADOR = created_at exatamente à meia-noite da data do movimento
--   (created_at = TIMESTAMP(data_pedido | data_abertura | data_visita |
--   data_snapshot | data_inicio)) E data do movimento no mês corrente.
--   Linhas importadas do CSV têm created_at = hora da importação e linhas da
--   API têm created_at = NOW(), então não casam com o marcador. O fuso da
--   sessão é fixado em -03:00 para o marcador ser estável entre execuções.
--   Antes de gerar, o script apaga só as linhas marcadas do mês corrente:
--   pagamentos dos pedidos marcados, os pedidos (itens saem por CASCADE),
--   oportunidades, visitas, estoque e carteiras marcadas; e reabre
--   (data_fim = NULL) o vínculo não-seed que o seed encerrou, se o cliente
--   ficou sem vínculo ativo. Meses anteriores não são tocados.
--   Tudo numa transação: qualquer erro aborta o cliente mariadb e a
--   conexão fecha sem COMMIT (rollback).
--
-- Limitações:
--   - Como a referência é o último dia do mês, o seed gera movimentos com
--     data futura (entre amanhã e o fim do mês) quando roda antes do dia final.
--   - A tela de Estoque mostra só o snapshot mais recente por SKU: no máximo
--     292 linhas (nº de SKUs), menos que 5 páginas de 100.
--   - Reexecutar consome IDs AUTO_INCREMENT (lacunas), sem duplicar linhas.
-- ============================================================

SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;
SET time_zone = '-03:00';

START TRANSACTION;

SET @ref  = LAST_DAY(CURDATE());  -- referência: último dia do mês corrente
SET @ini  = CAST(DATE_FORMAT(@ref, '%Y-%m-01') AS DATE);
SET @fim  = @ref;
SET @dias = DAY(@ref);            -- movimentos entre @ini e @ref (mês inteiro)

-- ------------------------------------------------------------
-- 1. Desfaz a execução anterior do mês corrente
-- ------------------------------------------------------------
DROP TEMPORARY TABLE IF EXISTS tmp_seed_ped;
CREATE TEMPORARY TABLE tmp_seed_ped (pedido_id BIGINT PRIMARY KEY) ENGINE=MEMORY
SELECT pedido_id_origem AS pedido_id FROM pedidos
 WHERE data_pedido BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_pedido);

DELETE FROM pagamentos WHERE pedido_id IN (SELECT pedido_id FROM tmp_seed_ped);
DELETE FROM pedidos    WHERE pedido_id_origem IN (SELECT pedido_id FROM tmp_seed_ped);

DELETE FROM oportunidades WHERE data_abertura BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_abertura);
DELETE FROM visitas       WHERE data_visita   BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_visita);
DELETE FROM estoque       WHERE data_snapshot BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_snapshot);

-- Carteiras: guarda a 1a data de transferência de cada cliente, apaga as
-- linhas do seed e reabre o vínculo anterior encerrado nessa data.
DROP TEMPORARY TABLE IF EXISTS tmp_rest;
CREATE TEMPORARY TABLE tmp_rest (cliente_id BIGINT PRIMARY KEY, d DATE) ENGINE=MEMORY
SELECT cliente_id, MIN(data_inicio) AS d FROM carteiras
 WHERE data_inicio BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_inicio)
 GROUP BY cliente_id;

DELETE FROM carteiras WHERE data_inicio BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_inicio);

DROP TEMPORARY TABLE IF EXISTS tmp_rest_id;
CREATE TEMPORARY TABLE tmp_rest_id (carteira_id BIGINT PRIMARY KEY) ENGINE=MEMORY
SELECT MAX(ca.carteira_id_origem) AS carteira_id
  FROM carteiras ca
  JOIN tmp_rest r ON r.cliente_id = ca.cliente_id AND ca.data_fim = r.d
 WHERE NOT EXISTS (SELECT 1 FROM carteiras x WHERE x.cliente_id = ca.cliente_id AND x.data_fim IS NULL)
 GROUP BY ca.cliente_id;

UPDATE carteiras SET data_fim = NULL, updated_at = updated_at
 WHERE carteira_id_origem IN (SELECT carteira_id FROM tmp_rest_id);

-- ------------------------------------------------------------
-- 2. Bases: vendedores ativos, clientes elegíveis, produtos
-- ------------------------------------------------------------
DROP TEMPORARY TABLE IF EXISTS tmp_vend;
CREATE TEMPORARY TABLE tmp_vend (rn INT PRIMARY KEY, id BIGINT) ENGINE=MEMORY
SELECT ROW_NUMBER() OVER (ORDER BY id) AS rn, id FROM vendedores WHERE data_desligamento IS NULL;
SET @nv = (SELECT COUNT(*) FROM tmp_vend);

-- Clientes ativos cuja carteira ativa é de vendedor não desligado.
DROP TEMPORARY TABLE IF EXISTS tmp_base;
CREATE TEMPORARY TABLE tmp_base (rn INT PRIMARY KEY, cliente_id BIGINT, vendedor_id BIGINT, carteira_id BIGINT) ENGINE=MEMORY
SELECT ROW_NUMBER() OVER (ORDER BY c.cliente_id_origem) AS rn,
       c.cliente_id_origem AS cliente_id, ca.vendedor_id, ca.carteira_id_origem AS carteira_id
  FROM clientes c
  JOIN carteiras ca ON ca.cliente_id = c.cliente_id_origem AND ca.data_fim IS NULL
  JOIN vendedores v ON v.id = ca.vendedor_id AND v.data_desligamento IS NULL
 WHERE c.ativo = 1;

-- 100 clientes transferidos (1 a cada 23, determinístico).
DROP TEMPORARY TABLE IF EXISTS tmp_transf;
CREATE TEMPORARY TABLE tmp_transf (idx INT PRIMARY KEY, cliente_id BIGINT, vend_a BIGINT, carteira_id BIGINT,
                                   d1 DATE, d2 DATE, vend_b BIGINT NULL, vend_c BIGINT NULL) ENGINE=MEMORY
SELECT ROW_NUMBER() OVER (ORDER BY rn) AS idx, cliente_id, vendedor_id AS vend_a, carteira_id,
       CAST(NULL AS DATE) AS d1, CAST(NULL AS DATE) AS d2, CAST(NULL AS SIGNED) AS vend_b, CAST(NULL AS SIGNED) AS vend_c
  FROM tmp_base WHERE MOD(rn, 23) = 0
 ORDER BY rn LIMIT 100;

-- idx ímpar = transferência simples (d1 = d2); par = dupla (B de d1 a d2, C a partir de d2).
UPDATE tmp_transf
   SET d1 = DATE_ADD(@ini, INTERVAL MOD(idx * 7, GREATEST(@dias - 1, 1)) DAY);
UPDATE tmp_transf
   SET d2 = IF(MOD(idx, 2) = 1, d1, LEAST(DATE_ADD(d1, INTERVAL 1 + MOD(idx, 7) DAY), @ref));

DROP TEMPORARY TABLE IF EXISTS tmp_pick;
CREATE TEMPORARY TABLE tmp_pick (idx INT PRIMARY KEY, vend BIGINT) ENGINE=MEMORY
SELECT idx, id AS vend FROM (
  SELECT t.idx, v.id, ROW_NUMBER() OVER (PARTITION BY t.idx ORDER BY MOD(v.rn + t.idx * 5, @nv), v.rn) AS k
    FROM tmp_transf t JOIN tmp_vend v ON v.id <> t.vend_a
) x WHERE k = 1;
UPDATE tmp_transf t JOIN tmp_pick p ON p.idx = t.idx SET t.vend_b = p.vend;

DELETE FROM tmp_pick;
INSERT INTO tmp_pick
SELECT idx, id FROM (
  SELECT t.idx, v.id, ROW_NUMBER() OVER (PARTITION BY t.idx ORDER BY MOD(v.rn + t.idx * 11 + 3, @nv), v.rn) AS k
    FROM tmp_transf t JOIN tmp_vend v ON v.id <> t.vend_a AND v.id <> t.vend_b
   WHERE MOD(t.idx, 2) = 0
) x WHERE k = 1;
UPDATE tmp_transf t JOIN tmp_pick p ON p.idx = t.idx SET t.vend_c = p.vend;

-- Clientes que recebem pedidos/visitas/oportunidades (sem os transferidos).
DROP TEMPORARY TABLE IF EXISTS tmp_cli;
CREATE TEMPORARY TABLE tmp_cli (rn INT PRIMARY KEY, cliente_id BIGINT, vendedor_id BIGINT) ENGINE=MEMORY
SELECT ROW_NUMBER() OVER (ORDER BY b.rn) AS rn, b.cliente_id, b.vendedor_id
  FROM tmp_base b WHERE b.cliente_id NOT IN (SELECT cliente_id FROM tmp_transf);
SET @nc = (SELECT COUNT(*) FROM tmp_cli);

DROP TEMPORARY TABLE IF EXISTS tmp_prod;
CREATE TEMPORARY TABLE tmp_prod (rn INT PRIMARY KEY, id BIGINT, preco DECIMAL(10,2)) ENGINE=MEMORY
SELECT ROW_NUMBER() OVER (ORDER BY id) AS rn, id, preco_tabela AS preco FROM produtos WHERE ativo = 1;
SET @np = (SELECT COUNT(*) FROM tmp_prod);

-- ------------------------------------------------------------
-- 3. Carteiras (transferências)
-- ------------------------------------------------------------
UPDATE carteiras ca JOIN tmp_transf t ON t.carteira_id = ca.carteira_id_origem
   SET ca.data_fim = t.d1, ca.updated_at = ca.updated_at;

-- Vínculo novo com B: encerrado em d2 nas duplas, ativo nas simples.
INSERT INTO carteiras (cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at)
SELECT cliente_id, vend_b, d1, IF(MOD(idx, 2) = 0, d2, NULL), TIMESTAMP(d1), TIMESTAMP(d1)
  FROM tmp_transf;

-- Vínculo novo ativo com C (só nas duplas).
INSERT INTO carteiras (cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at)
SELECT cliente_id, vend_c, d2, NULL, TIMESTAMP(d2), TIMESTAMP(d2)
  FROM tmp_transf WHERE MOD(idx, 2) = 0;

-- ------------------------------------------------------------
-- 4. Pedidos + itens + pagamentos
-- ------------------------------------------------------------
SET @base_ped = (SELECT COALESCE(MAX(pedido_id_origem), 0) FROM pedidos);

DROP TEMPORARY TABLE IF EXISTS tmp_ped;
CREATE TEMPORARY TABLE tmp_ped (
  i INT PRIMARY KEY, pedido_id BIGINT, cliente_id BIGINT, vendedor_id BIGINT, data_pedido DATE,
  canal VARCHAR(20), status VARCHAR(20), total DECIMAL(15,2) DEFAULT 0) ENGINE=MEMORY
SELECT s.seq AS i, @base_ped + s.seq AS pedido_id, c.cliente_id, c.vendedor_id,
       DATE_ADD(@ini, INTERVAL MOD(s.seq * 7, @dias) DAY) AS data_pedido,
       ELT(1 + MOD(s.seq, 4), 'App', 'Telefone', 'Visita', 'WhatsApp') AS canal,
       CASE
         WHEN MOD(s.seq, 20) = 0  THEN 'Cancelado'
         WHEN MOD(s.seq, 20) <= 2 THEN 'Em separação'
         WHEN MOD(s.seq, 20) <= 9 THEN 'Faturado'
         ELSE 'Entregue'
       END AS status,
       0 AS total
  FROM seq_1_to_1000 s
  JOIN tmp_cli c ON c.rn = 1 + MOD(s.seq * 37, @nc);

DROP TEMPORARY TABLE IF EXISTS tmp_item;
CREATE TEMPORARY TABLE tmp_item (
  pedido_id BIGINT, j INT, produto_id BIGINT, quantidade INT, preco DECIMAL(10,2), desconto DECIMAL(5,2),
  bruto DECIMAL(15,2), PRIMARY KEY (pedido_id, j)) ENGINE=MEMORY
SELECT x.pedido_id, x.j, x.produto_id, x.quantidade, x.preco, x.desconto,
       ROUND(x.quantidade * x.preco * (1 - x.desconto / 100), 2) AS bruto
  FROM (
    SELECT p.pedido_id, j.seq AS j, pr.id AS produto_id,
           1 + MOD(p.i + j.seq * 3, 12) AS quantidade,
           IF(MOD(p.i + j.seq, 3) = 0, ROUND(pr.preco * 0.95, 2), pr.preco) AS preco,
           CAST(ELT(1 + MOD(p.i * 3 + j.seq, 6), '0', '0', '5', '10', '0', '15') AS DECIMAL(5,2)) AS desconto
      FROM tmp_ped p
      JOIN seq_1_to_8 j ON j.seq <= 1 + MOD(p.i, 8)
      JOIN tmp_prod pr ON pr.rn = 1 + MOD(p.i * 13 + j.seq * 29, @np)
  ) x;

UPDATE tmp_ped p JOIN (SELECT pedido_id, SUM(bruto) AS t FROM tmp_item GROUP BY pedido_id) s
    ON s.pedido_id = p.pedido_id
   SET p.total = s.t;

INSERT INTO pedidos (pedido_id_origem, cliente_id, vendedor_id, data_pedido, canal, status, valor_total, created_at, updated_at)
SELECT pedido_id, cliente_id, vendedor_id, data_pedido, canal, status, total, TIMESTAMP(data_pedido), TIMESTAMP(data_pedido)
  FROM tmp_ped ORDER BY i;

INSERT INTO itens_pedido (pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto, created_at, updated_at)
SELECT it.pedido_id, it.produto_id, it.quantidade, it.preco, it.desconto, it.bruto, TIMESTAMP(p.data_pedido), TIMESTAMP(p.data_pedido)
  FROM tmp_item it JOIN tmp_ped p ON p.pedido_id = it.pedido_id
 ORDER BY it.pedido_id, it.j;

-- Pagamentos: forma -> prazo/taxa/parcelas; status coerente com @ref.
INSERT INTO pagamentos (pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido,
                        data_vencimento, data_pagamento, status_pagamento, created_at, updated_at)
SELECT y.pedido_id, y.forma, y.parcelas, y.valor, y.taxa, ROUND(y.valor * (1 - y.taxa / 100), 2),
       y.venc,
       CASE y.st
         WHEN 'Pago' THEN IF(y.venc >= @ref,
                             LEAST(DATE_ADD(y.data_pedido, INTERVAL MOD(y.i, 3) DAY), @ref),
                             LEAST(DATE_ADD(y.venc, INTERVAL MOD(y.i, 6) DAY), @ref))
         WHEN 'Pago com atraso' THEN LEAST(DATE_ADD(y.venc, INTERVAL 16 + MOD(y.i, 20) DAY), @ref)
         ELSE NULL
       END,
       y.st, TIMESTAMP(y.data_pedido), TIMESTAMP(y.data_pedido)
  FROM (
    SELECT z.*,
           CASE
             WHEN z.venc >= @ref THEN IF(MOD(z.i, 3) = 0, 'Pago', 'Em aberto')
             WHEN MOD(z.i, 10) <= 1 THEN 'Inadimplente'
             WHEN MOD(z.i, 10) <= 3 THEN 'Pago com atraso'
             ELSE 'Pago'
           END AS st
      FROM (
        SELECT f.*,
               DATE_ADD(f.data_pedido, INTERVAL CASE f.forma
                   WHEN 'Boleto 14 dias' THEN 14 WHEN 'Boleto 28 dias' THEN 28
                   WHEN 'Cheque a prazo' THEN 35
                   WHEN 'Cartão de crédito' THEN 1 WHEN 'Cartão de débito' THEN 1
                   ELSE 0 END DAY) AS venc,
               CASE f.forma WHEN 'Cartão de crédito' THEN 3.20 WHEN 'Cartão de débito' THEN 1.80 ELSE 0.00 END AS taxa,
               IF(f.forma = 'Cartão de crédito', 1 + MOD(f.i, 6), 1) AS parcelas
          FROM (
            SELECT p.i, p.pedido_id, p.data_pedido, p.total AS valor,
                   ELT(1 + MOD(p.i DIV 3, 7), 'PIX', 'Boleto 14 dias', 'Cartão de crédito', 'Dinheiro',
                       'Boleto 28 dias', 'Cartão de débito', 'Cheque a prazo') AS forma
              FROM tmp_ped p WHERE p.status <> 'Cancelado'
          ) f
      ) z
  ) y
 ORDER BY y.i;

-- ------------------------------------------------------------
-- 5. Oportunidades
-- ------------------------------------------------------------
INSERT INTO oportunidades (cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado,
                           data_fechamento, ciclo_dias, motivo_perda, created_at, updated_at)
SELECT o.cliente_id, o.vendedor_id, o.origem, o.abertura, o.etapa,
       CASE o.etapa WHEN 'Prospecção' THEN 10 WHEN 'Qualificação' THEN 25 WHEN 'Proposta enviada' THEN 50
                    WHEN 'Negociação' THEN 75 WHEN 'Fechado ganho' THEN 100 ELSE 0 END,
       o.valor,
       IF(o.fechada, DATE_ADD(o.abertura, INTERVAL o.ciclo DAY), NULL),
       IF(o.fechada, o.ciclo, NULL),
       IF(o.etapa = 'Fechado perdido',
          ELT(1 + MOD(o.i DIV 6, 8), 'Fechou com concorrente', 'Já compra de outro importador',
              'Mix de fragrâncias não atende', 'Pediu exclusividade de marca', 'Prazo de entrega',
              'Preço acima do concorrente', 'Sem contato', 'Sem verba no momento'),
          NULL),
       TIMESTAMP(o.abertura), TIMESTAMP(o.abertura)
  FROM (
    SELECT b.*, b.etapa IN ('Fechado ganho', 'Fechado perdido') AS fechada,
           LEAST(DATEDIFF(@ref, b.abertura), 3 + MOD(b.i * 7, 28)) AS ciclo
      FROM (
        SELECT s.seq AS i, c.cliente_id, c.vendedor_id,
               ELT(1 + MOD(s.seq, 7), 'Feira de beleza', 'Inbound site', 'Indicação', 'Instagram',
                   'Prospecção ativa', 'Reativação', 'WhatsApp') AS origem,
               ELT(1 + MOD(s.seq, 6), 'Prospecção', 'Qualificação', 'Proposta enviada', 'Negociação',
                   'Fechado ganho', 'Fechado perdido') AS etapa,
               DATE_ADD(@ini, INTERVAL MOD(s.seq * 11, @dias) DAY) AS abertura,
               ROUND(2500 + MOD(s.seq * 7919, 168000) + MOD(s.seq * 37, 100) / 100, 2) AS valor
          FROM seq_1_to_700 s
          JOIN tmp_cli c ON c.rn = 1 + MOD(s.seq * 53, @nc)
      ) b
  ) o
 ORDER BY o.i;

-- ------------------------------------------------------------
-- 6. Visitas
-- ------------------------------------------------------------
INSERT INTO visitas (cliente_id, vendedor_id, data_visita, resultado, duracao_min, created_at, updated_at)
SELECT v.cliente_id, v.vendedor_id, v.data_visita,
       CASE
         WHEN MOD(v.i, 20) <= 8  THEN 'Pedido realizado'
         WHEN MOD(v.i, 20) <= 13 THEN 'Sem pedido'
         WHEN MOD(v.i, 20) <= 16 THEN 'Cliente ausente'
         WHEN MOD(v.i, 20) <= 18 THEN 'Reagendada'
         ELSE 'Apenas relacionamento'
       END,
       10 + MOD(v.i * 13, 81),
       TIMESTAMP(v.data_visita), TIMESTAMP(v.data_visita)
  FROM (
    SELECT s.seq AS i, c.cliente_id, c.vendedor_id,
           DATE_ADD(@ini, INTERVAL MOD(s.seq * 3, @dias) DAY) AS data_visita
      FROM seq_1_to_700 s
      JOIN tmp_cli c ON c.rn = 1 + MOD(s.seq * 41, @nc)
  ) v
 ORDER BY v.i;

-- ------------------------------------------------------------
-- 7. Estoque (snapshots semanais de todos os SKUs)
-- ------------------------------------------------------------
INSERT INTO estoque (data_snapshot, sku, saldo, ruptura, created_at, updated_at)
SELECT e.d, e.sku, IF(e.rup, 0, e.saldo), e.rup, TIMESTAMP(e.d), TIMESTAMP(e.d)
  FROM (
    SELECT DATE_ADD(@ini, INTERVAL w.seq * 7 DAY) AS d, p.sku,
           MOD(p.rn * 7 + w.seq * 3, 9) = 0 AS rup,
           1 + MOD(p.rn * 31 + w.seq * 17, 800) AS saldo
      FROM (SELECT ROW_NUMBER() OVER (ORDER BY id) AS rn, sku FROM produtos) p
      CROSS JOIN seq_0_to_4 w
     WHERE DATE_ADD(@ini, INTERVAL w.seq * 7 DAY) <= @ref
  ) e
 WHERE NOT EXISTS (SELECT 1 FROM estoque x WHERE x.data_snapshot = e.d AND x.sku = e.sku)
 ORDER BY e.d, e.sku;

COMMIT;

DROP TEMPORARY TABLE IF EXISTS tmp_seed_ped, tmp_rest, tmp_rest_id, tmp_vend, tmp_base, tmp_transf,
                               tmp_pick, tmp_cli, tmp_prod, tmp_ped, tmp_item;

-- ------------------------------------------------------------
-- 8. Resumo (linhas do seed no mês corrente)
-- ------------------------------------------------------------
SELECT DATE_FORMAT(@ini, '%Y-%m') AS mes,
  (SELECT COUNT(*) FROM pedidos WHERE data_pedido BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_pedido)) AS pedidos,
  (SELECT COUNT(*) FROM itens_pedido i JOIN pedidos p ON p.pedido_id_origem = i.pedido_id
    WHERE p.data_pedido BETWEEN @ini AND @fim AND p.created_at = TIMESTAMP(p.data_pedido)) AS itens,
  (SELECT COUNT(*) FROM pagamentos g JOIN pedidos p ON p.pedido_id_origem = g.pedido_id
    WHERE p.data_pedido BETWEEN @ini AND @fim AND p.created_at = TIMESTAMP(p.data_pedido)) AS pagamentos,
  (SELECT COUNT(*) FROM oportunidades WHERE data_abertura BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_abertura)) AS oportunidades,
  (SELECT COUNT(*) FROM visitas WHERE data_visita BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_visita)) AS visitas,
  (SELECT COUNT(*) FROM estoque WHERE data_snapshot BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_snapshot)) AS estoque,
  (SELECT COUNT(*) FROM carteiras WHERE data_inicio BETWEEN @ini AND @fim AND created_at = TIMESTAMP(data_inicio)) AS carteiras;
