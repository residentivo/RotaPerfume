-- ============================================================
-- SEED: Pedidos de exemplo (30+ registros)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-07
-- Agente: DataBrain (🌸)
--
-- Dados dos últimos 30 dias (2026-08-08 a 2026-09-07)
-- Valores entre R$ 200 e R$ 5.000
-- Distribuídos entre os vendedores existentes
-- ============================================================

SET NAMES utf8mb4;

-- ------------------------------------------------------------
-- Seed: Pedidos (INSERT IGNORE para idempotência)
-- ------------------------------------------------------------
INSERT IGNORE INTO `pedidos` (`id`, `id_vendedor`, `data_pedido`, `valor_total`, `quantidade_itens`, `status`, `cliente_nome`) VALUES
-- Agostinho (08/08)
(1, 1, '2026-08-08 09:15:00', 1250.00, 5, 'confirmado', 'Perfumaria Bela Vista'),
(2, 5, '2026-08-08 14:30:00', 890.50, 3, 'confirmado', 'Cosméticos Express'),
(3, 10, '2026-08-08 16:45:00', 2340.00, 8, 'confirmado', 'Bazar Aromas'),
-- Agostinho (10/08)
(4, 3, '2026-08-10 10:00:00', 450.00, 2, 'cancelado', 'Casa de Fragrâncias'),
(5, 7, '2026-08-10 11:30:00', 1890.00, 6, 'confirmado', 'Loja Essências'),
(6, 12, '2026-08-10 15:20:00', 3200.00, 10, 'confirmado', 'Supermercado Aroma'),
-- Agostinho (13/08)
(7, 2, '2026-08-13 08:45:00', 680.00, 4, 'confirmado', 'Perfumaria Central'),
(8, 8, '2026-08-13 13:00:00', 1560.00, 5, 'confirmado', 'Distribuidora Beauty'),
(9, 15, '2026-08-13 17:30:00', 975.00, 3, 'pendente', 'Farmácia São José'),
-- Agostinho (15/08)
(10, 4, '2026-08-15 09:00:00', 2100.00, 7, 'confirmado', 'Hotel Sabor & Aroma'),
(11, 11, '2026-08-15 14:15:00', 560.00, 2, 'confirmado', 'Restaurante Vintage'),
(12, 18, '2026-08-15 16:00:00', 4800.00, 15, 'confirmado', 'Rede Hotéis Paraiso'),
-- Agostinho (18/08)
(13, 6, '2026-08-18 10:30:00', 330.00, 1, 'devolvido', 'Espaço Beleza Plus'),
(14, 14, '2026-08-18 12:45:00', 1720.00, 6, 'confirmado', 'Clínica Estética Oásis'),
(15, 20, '2026-08-18 15:00:00', 890.00, 4, 'confirmado', 'Salão Elegância'),
-- Agostinho (20/08)
(16, 9, '2026-08-20 08:30:00', 2450.00, 9, 'confirmado', 'Academia Fit Life'),
(17, 16, '2026-08-20 11:00:00', 720.00, 3, 'confirmado', 'SPA Serenidade'),
(18, 22, '2026-08-20 14:30:00', 1890.00, 7, 'confirmado', 'Instituto Beleza Natural'),
-- Agostinho (23/08)
(19, 13, '2026-08-23 09:45:00', 1100.00, 4, 'confirmado', 'Clube de Campo Horizonte'),
(20, 17, '2026-08-23 13:15:00', 670.00, 2, 'cancelado', 'Bar Restaurante Colonial'),
(21, 23, '2026-08-23 16:30:00', 3400.00, 12, 'confirmado', 'Confeitaria Doces Sonhos'),
-- Agostinho (26/08)
(22, 19, '2026-08-26 10:00:00', 540.00, 2, 'confirmado', 'Papelaria Colorida'),
(23, 21, '2026-08-26 12:30:00', 2890.00, 8, 'confirmado', 'Loja Variedades Premium'),
(24, 25, '2026-08-26 15:45:00', 1280.00, 5, 'confirmado', 'Padaria Pão Dourado'),
-- Agostinho (28/08)
(25, 24, '2026-08-28 08:15:00', 920.00, 3, 'confirmado', 'Sorveteria Sabor Natural'),
(26, 27, '2026-08-28 11:30:00', 4100.00, 14, 'confirmado', 'Hotel Transylvania'),
(27, 29, '2026-08-28 14:00:00', 780.00, 3, 'pendente', 'Quitutes da Vovó'),
-- Agostinho (30/08)
(28, 26, '2026-08-30 09:00:00', 1560.00, 6, 'confirmado', 'Empório Colonial'),
(29, 28, '2026-08-30 13:30:00', 2100.00, 7, 'confirmado', 'Casa de Chá Serenidade'),
(30, 31, '2026-08-30 16:00:00', 640.00, 2, 'confirmado', 'Doceria doce Lar'),
-- Agostinho (02/09)
(31, 32, '2026-09-02 10:15:00', 1850.00, 6, 'confirmado', 'Lanchonete Sabor Rural'),
(32, 33, '2026-09-02 14:00:00', 980.00, 4, 'confirmado', 'Barbearia Elegância'),
(33, 35, '2026-09-02 17:30:00', 3200.00, 11, 'confirmado', 'Restaurante Tudo Gostoso'),
-- Agostinho (04/09)
(34, 34, '2026-09-04 08:45:00', 450.00, 2, 'confirmado', 'Boutique Fashion'),
(35, 36, '2026-09-04 12:00:00', 1680.00, 5, 'confirmado', 'Casa de Eventos Alegria'),
(36, 38, '2026-09-04 15:15:00', 2250.00, 8, 'confirmado', 'Centro Estético Beleza Plena'),
-- Agostinho (05/09)
(37, 37, '2026-09-05 09:30:00', 890.00, 3, 'devolvido', 'Floricultura Rosas'),
(38, 39, '2026-09-05 13:00:00', 1320.00, 5, 'confirmado', 'Lavanderia Aroma Fresco'),
(39, 40, '2026-09-05 16:30:00', 4750.00, 16, 'confirmado', 'Spa Resort Serenity'),
-- Agostinho (06/09)
(40, 41, '2026-09-06 10:00:00', 620.00, 2, 'confirmado', 'Cantina Italiana Trattoria'),
(41, 42, '2026-09-06 14:30:00', 1980.00, 7, 'confirmado', 'Pet Shop Amigo Fiel'),
(42, 1, '2026-09-06 17:00:00', 850.00, 3, 'confirmado', 'Livraria Palavra Viva'),
-- Agostinho (07/09)
(43, 5, '2026-09-07 08:30:00', 1100.00, 4, 'confirmado', 'Armarinho Confiança'),
(44, 10, '2026-09-07 11:15:00', 2800.00, 9, 'confirmado', 'Hotel Majestic'),
(45, 15, '2026-09-07 14:45:00', 740.00, 3, 'confirmado', 'Bar do Zé');
