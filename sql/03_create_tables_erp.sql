USE rotaperfumes;

CREATE TABLE IF NOT EXISTS produtos (
  sku              VARCHAR(20) PRIMARY KEY,
  descricao        VARCHAR(255),
  categoria        VARCHAR(80),
  marca            VARCHAR(80),
  nota_olfativa    VARCHAR(80),
  preco_tabela     DECIMAL(10,2),
  custo_unitario   DECIMAL(10,2),
  unidade          VARCHAR(20),
  ativo            CHAR(1) DEFAULT 'S',
  data_lancamento  DATE DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS pedidos (
  id             INT PRIMARY KEY,
  cliente_id     INT NOT NULL,
  vendedor_id    INT NOT NULL,
  data_pedido    DATE,
  canal          VARCHAR(30),
  status         VARCHAR(30),
  valor_total    DECIMAL(12,2),
  FOREIGN KEY (cliente_id)  REFERENCES clientes(id),
  FOREIGN KEY (vendedor_id) REFERENCES vendedores(id)
);

CREATE TABLE IF NOT EXISTS itens_pedido (
  id               INT PRIMARY KEY,
  pedido_id        INT NOT NULL,
  sku              VARCHAR(20) NOT NULL,
  quantidade       INT,
  preco_praticado  DECIMAL(10,2),
  desconto_pct     DECIMAL(5,2),
  valor_bruto      DECIMAL(12,2),
  FOREIGN KEY (pedido_id) REFERENCES pedidos(id),
  FOREIGN KEY (sku)      REFERENCES produtos(sku)
);

CREATE TABLE IF NOT EXISTS estoque (
  data_snapshot DATE,
  sku           VARCHAR(20) NOT NULL,
  saldo         INT,
  ruptura       CHAR(1),
  PRIMARY KEY (data_snapshot, sku),
  FOREIGN KEY (sku) REFERENCES produtos(sku)
);

CREATE TABLE IF NOT EXISTS pagamentos (
  id                 INT PRIMARY KEY,
  pedido_id          INT NOT NULL,
  forma_pagamento    VARCHAR(40),
  parcelas           INT,
  valor              DECIMAL(12,2),
  taxa_pct           DECIMAL(5,2),
  valor_liquido      DECIMAL(12,2),
  data_vencimento    DATE,
  data_pagamento     DATE DEFAULT NULL,
  status_pagamento   VARCHAR(40),
  FOREIGN KEY (pedido_id) REFERENCES pedidos(id)
);
