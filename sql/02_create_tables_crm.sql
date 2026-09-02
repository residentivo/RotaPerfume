USE rotaperfumes;

CREATE TABLE IF NOT EXISTS vendedores (
  id            INT PRIMARY KEY,
  nome          VARCHAR(100) NOT NULL,
  regiao        VARCHAR(50),
  uf            CHAR(2),
  data_admissao DATE,
  data_desligamento DATE DEFAULT NULL,
  meta_mensal   DECIMAL(12,2)
);

CREATE TABLE IF NOT EXISTS clientes (
  id              INT PRIMARY KEY,
  cnpj            VARCHAR(20),
  razao_social    VARCHAR(150),
  segmento        VARCHAR(80),
  cidade          VARCHAR(80),
  uf              CHAR(2),
  bairro          VARCHAR(80),
  data_cadastro   DATE,
  ativo           CHAR(1) DEFAULT 'S'
);

CREATE TABLE IF NOT EXISTS carteira (
  id            INT PRIMARY KEY,
  cliente_id    INT NOT NULL,
  vendedor_id   INT NOT NULL,
  data_inicio   DATE,
  data_fim      DATE DEFAULT NULL,
  FOREIGN KEY (cliente_id)  REFERENCES clientes(id),
  FOREIGN KEY (vendedor_id) REFERENCES vendedores(id)
);

CREATE TABLE IF NOT EXISTS visitas (
  id             INT PRIMARY KEY,
  cliente_id     INT NOT NULL,
  vendedor_id    INT NOT NULL,
  data_visita    DATE,
  resultado      VARCHAR(50),
  duracao_min    INT,
  FOREIGN KEY (cliente_id)  REFERENCES clientes(id),
  FOREIGN KEY (vendedor_id) REFERENCES vendedores(id)
);

CREATE TABLE IF NOT EXISTS oportunidades (
  id                  INT PRIMARY KEY,
  cliente_id          INT NOT NULL,
  vendedor_id         INT NOT NULL,
  origem              VARCHAR(50),
  data_abertura       DATE,
  etapa               VARCHAR(50),
  probabilidade_pct   INT,
  valor_estimado      DECIMAL(12,2),
  data_fechamento    DATE DEFAULT NULL,
  ciclo_dias          INT DEFAULT NULL,
  motivo_perda        VARCHAR(200) DEFAULT NULL,
  FOREIGN KEY (cliente_id)  REFERENCES clientes(id),
  FOREIGN KEY (vendedor_id) REFERENCES vendedores(id)
);
