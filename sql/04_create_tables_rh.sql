USE rotaperfumes;

CREATE TABLE IF NOT EXISTS usuarios (
  id              INT PRIMARY KEY AUTO_INCREMENT,
  login           VARCHAR(50) NOT NULL UNIQUE,
  email           VARCHAR(120) NOT NULL UNIQUE,
  senha_hash      VARCHAR(255) NOT NULL,
  tipo_usuario    ENUM('vendedor','gerente','rh') NOT NULL,
  vendedor_id     INT DEFAULT NULL,
  gerenciado_por  INT DEFAULT NULL,
  ativo           CHAR(1) DEFAULT 'S',
  data_criacao    DATETIME DEFAULT CURRENT_TIMESTAMP,
  data_atualizacao DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (vendedor_id)   REFERENCES vendedores(id),
  FOREIGN KEY (gerenciado_por) REFERENCES usuarios(id)
);
