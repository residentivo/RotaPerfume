USE rotaperfumes;

-- Configuração para carregar CSVs locais
SET GLOBAL local_infile = 1;

-- =============================================
-- CRM
-- =============================================

LOAD DATA LOCAL INFILE 'dados/crm/vendedores.csv'
  INTO TABLE vendedores
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;

LOAD DATA LOCAL INFILE 'dados/crm/clientes.csv'
  INTO TABLE clientes
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;

LOAD DATA LOCAL INFILE 'dados/crm/carteira.csv'
  INTO TABLE carteira
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;

LOAD DATA LOCAL INFILE 'dados/crm/visitas.csv'
  INTO TABLE visitas
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;

LOAD DATA LOCAL INFILE 'dados/crm/oportunidades.csv'
  INTO TABLE oportunidades
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;

-- =============================================
-- ERP
-- =============================================

LOAD DATA LOCAL INFILE 'dados/erp/produtos.csv'
  INTO TABLE produtos
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;

LOAD DATA LOCAL INFILE 'dados/erp/pedidos.csv'
  INTO TABLE pedidos
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;

LOAD DATA LOCAL INFILE 'dados/erp/itens_pedido.csv'
  INTO TABLE itens_pedido
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;

LOAD DATA LOCAL INFILE 'dados/erp/estoque.csv'
  INTO TABLE estoque
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;

LOAD DATA LOCAL INFILE 'dados/erp/pagamentos.csv'
  INTO TABLE pagamentos
  FIELDS TERMINATED BY ',' ENCLOSED BY '"'
  LINES TERMINATED BY '\n'
  IGNORE 1 ROWS;
