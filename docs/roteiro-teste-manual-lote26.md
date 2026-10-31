# Roteiro de teste manual e uso da tela: Lote 26 (IMG-01, imagem do produto)

**Card:** IMG-01 (imagem do produto). Detalhes em `tarefas/feito.md` (Lote 26), em `postman/README.md` ("Mudanças do Lote 26" e os endpoints de imagem em "Produtos") e em `docs/manual-base-de-dados.md` (seções 13 e 13.1).
**Migração:** 26 (`produtos.imagem`, já aplicada no banco de dev).
**Mudança de contrato HTTP:** 3 rotas novas (`POST`/`DELETE /api/produtos/{id}/imagem` e `GET /api/public/produtos/imagens/{arquivo}`) e o campo `imagem_url` no JSON do produto. Nenhum contrato existente mudou.
**Tempo estimado:** 20 a 30 minutos.
**Autor:** SubBrain (2026-10-31)

Marque cada checkbox depois de conferir. Se algo divergir, anote o caso e encaminhe ao SubBrain.

---

## 0. Preparação

1. Banco de dev com a migração 26. Num banco antigo, rode `make db-fix-produtos-imagem` (idempotente).
2. Se os produtos ainda não tiverem imagem, rode `make db-gerar-imagens-produtos`. Cada produto ganha um PNG 600x600 com o nome na diagonal. Para trocar todas: `make db-gerar-imagens-produtos GERAR_IMAGENS_FLAGS=-sobrescrever`. Para só simular: `GERAR_IMAGENS_FLAGS=-dry-run`.
3. Suba a API (`make dev-api`) e o frontend (`make dev-frontend`). Abra o frontend em `http://localhost:3000` (mesmo host `localhost` da API, por causa do `Cross-Origin-Resource-Policy: same-site`; com `127.0.0.1` num lado e `localhost` no outro, as imagens podem não carregar).
4. Separe arquivos de teste:
   - uma foto JPEG pequena (menos de 1 MB);
   - um PNG renomeado para `foto.jpg`;
   - um arquivo maior que 1 MB (JPEG ou PNG);
   - um SVG renomeado para `logo.png`;
   - um `.txt` renomeado para `teste.png`.

## 1. Como usar a tela (passo a passo)

### 1.1 Ver as imagens na lista de produtos

1. Entre como **admin** e abra **Produtos** (`/admin/produtos`).
2. A primeira coluna da tabela, **Imagem**, mostra uma miniatura de 40 px de cada produto.
3. Produto sem imagem mostra um quadro cinza com a dica "Sem imagem".

### 1.2 Enviar ou trocar a imagem (só admin)

1. Na linha do produto, clique em **Editar**. A seção **Imagem** fica no fim do formulário. Ela só aparece na edição: para pôr imagem num produto novo, crie o produto e depois edite.
2. A seção mostra a imagem atual, ou "Sem imagem".
3. Em **Arquivo (JPEG, PNG ou WebP, até 1 MB)**, escolha o arquivo. A prévia aparece no lugar da imagem atual.
4. Para desistir antes de enviar, clique em **Descartar seleção**.
5. Clique em **Enviar imagem** (ou **Trocar imagem**, se o produto já tem uma). Ao terminar, aparece "Imagem enviada com sucesso." e a miniatura da lista é atualizada.
6. O envio da imagem é imediato e independe do botão **Salvar alterações**, que grava só os campos do formulário.

### 1.3 Remover a imagem (só admin)

1. No modal de edição de um produto com imagem, clique em **Remover imagem**.
2. Confirme a pergunta "Remover a imagem do produto ...?". Aparece "Imagem removida com sucesso." e a lista volta a mostrar "Sem imagem".

### 1.4 Mensagens de erro

| Situação | Mensagem |
|---|---|
| Tipo diferente de JPEG, PNG ou WebP (checado no navegador e depois pela API, pelo conteúdo) | `arquivo de imagem inválido (use JPEG, PNG ou WebP)` |
| Arquivo maior que 1 MB | `imagem excede 1 MB` |
| Produto apagado ou inexistente | `produto não encontrado` |

### 1.5 Usuário normal (vendedor)

O item **Produtos** do menu (grupo Administração) só aparece para admin. Se um usuário `normal` abrir a URL `/admin/produtos` à mão, a seção Imagem do modal mostra só a imagem, sem seletor de arquivo e sem os botões Enviar, Trocar e Remover. Mesmo que ele chame a API direto, as rotas de envio e remoção respondem `403`. A leitura da imagem é pública e não exige login.

## 2. Testes da tela

1. **Envio válido:** edite um produto, envie a foto JPEG.
   - [ ] 2.1 Prévia antes do envio; depois, "Imagem enviada com sucesso.".
   - [ ] 2.2 A miniatura da lista mostra a nova imagem sem recarregar a página.
   - [ ] 2.3 No DevTools (aba Rede), a requisição `POST /api/produtos/{id}/imagem` leva `X-Requested-With: XMLHttpRequest` e `Content-Type: multipart/form-data; boundary=...` (não `application/json`).
   - [ ] 2.4 A resposta traz `imagem_url` no formato `/api/public/produtos/imagens/<uuid>.jpg`.
2. **Extensão enganosa aceita:** envie o PNG renomeado para `foto.jpg`.
   - [ ] 2.5 Aceito; o `imagem_url` termina em `.png` (vale o conteúdo, não a extensão).
3. **Troca:** com imagem já gravada, envie outra.
   - [ ] 2.6 O botão diz "Trocar imagem"; o `imagem_url` muda (novo UUID).
   - [ ] 2.7 Abrir a URL antiga no navegador (`http://localhost:8080/api/public/produtos/imagens/<uuid-antigo>`) dá `404` `"imagem não encontrada"`.
   - [ ] 2.8 Em `apis/rotaperfumes-api/data/produtos-imagens/` o arquivo antigo não existe mais.
4. **Rejeições:**
   - [ ] 2.9 Arquivo acima de 1 MB: `imagem excede 1 MB`, sem chamar a API (validação no navegador).
   - [ ] 2.10 SVG renomeado para `.png` e `.txt` renomeado para `.png`: `arquivo de imagem inválido (use JPEG, PNG ou WebP)`. Se o navegador informar `image/png`, a requisição sai e a API responde `400` com a mesma mensagem.
   - [ ] 2.11 Depois de uma rejeição, a imagem anterior do produto continua igual.
5. **Remoção:**
   - [ ] 2.12 Cancelar a confirmação não remove nada.
   - [ ] 2.13 Confirmar remove; a lista mostra "Sem imagem"; a URL antiga dá `404`.
6. **Criação:**
   - [ ] 2.14 O modal "Novo Produto" não mostra a seção Imagem.

## 3. Testes da API (Postman ou curl)

Use a pasta **Produtos** da collection: "Enviar/Trocar Imagem do Produto", "Imagem do Produto — leitura pública" e "Remover Imagem do Produto", nessa ordem.

- [ ] 3.1 Upload sem o header `X-Requested-With` → `403` `"requisição de upload recusada"`.
- [ ] 3.2 Upload com token de vendedor → `403`; sem token → `401`.
- [ ] 3.3 Upload em `/api/produtos/0/imagem` → `400` `"id inválido"`; em `/api/produtos/999999/imagem` → `404` e nenhum arquivo novo no diretório.
- [ ] 3.4 Leitura pública sem token → `200`, `Content-Type` de imagem, `Cache-Control: public, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff`, `Cross-Origin-Resource-Policy: same-site`, sem `Set-Cookie`.
- [ ] 3.5 `curl -I` na URL pública → `200` sem corpo; `curl -H "Range: bytes=0-99"` → `206`.
- [ ] 3.6 Nome em maiúsculas, `.jpeg`, `.svg` ou `.tmp-<uuid>` → `404`.
- [ ] 3.7 Remover duas vezes seguidas → `200` nas duas, com `imagem_url: null`.

## 4. Comando de geração

- [ ] 4.1 `cd apis/rotaperfumes-api && go run ./cmd/gerarimagens` (sem `-confirmar`) não grava nada.
- [ ] 4.2 `go run ./cmd/gerarimagens -dry-run` lista os produtos sem gravar.
- [ ] 4.3 `make db-gerar-imagens-produtos` pula os produtos que já têm imagem e loga os totais.

## 5. Limitações conhecidas (cards de backlog)

- WebP é gravado como veio (EXIF e bytes anexados ficam no arquivo); JPEG e PNG são reencodados. Card SEC no bloco "Achados do IMG-01" de `tarefas/afazer.md`.
- Arquivos órfãos (sem referência no banco) e `.tmp-*` antigos não são limpos automaticamente. Card CHORE no mesmo bloco.
