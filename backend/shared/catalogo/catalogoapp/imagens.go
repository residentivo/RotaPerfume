package catalogoapp

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/rotaperfumes/shared/appkit"
	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/vlog"
)

const arqImagens = "imagens.go"

// DefinirImagemProduto valida o conteúdo, grava o arquivo novo e troca a
// imagem do produto (IMG-01). Ordem: normaliza → grava o arquivo → tx
// (SELECT … FOR UPDATE, UPDATE, commit) → só então apaga a antiga
// (best-effort, com log). Produto inexistente ou falha na tx apagam o
// arquivo novo. Devolve o produto relido.
func (c *Casos) DefinirImagemProduto(ctx context.Context, id int64, dados []byte) (p *catalogo.Produto, err error) {
	defer vlog.Trace(arqImagens, "Catalogo.DefinirImagemProduto", "id=%d bytes=%d", id, len(dados))(&err)
	img, err := c.imagensLigadas()
	if err != nil {
		return nil, err
	}
	norm, err := img.Normalizador.Normalizar(dados)
	if err != nil {
		return nil, err
	}
	nome, err := img.Armazenamento.Gravar(ctx, norm.Extensao, norm.Dados)
	if err != nil {
		return nil, err
	}
	antiga, err := c.trocarImagem(ctx, img, id, nome)
	if err != nil {
		c.removerArquivo(ctx, img, id, nome)
		return nil, err
	}
	log.Printf("[produtos] imagem gravada: id=%d arquivo=%s tipo=%s bytes=%d", id, nome, norm.Tipo, len(norm.Dados))
	if antiga != "" {
		c.removerArquivo(ctx, img, id, antiga)
	}
	return appkit.RelerAtualizado(ctx, "catalogoapp", id, c.produtos.PorID)
}

// RemoverImagemProduto zera a imagem do produto (idempotente: sem imagem
// não é erro) e só depois do commit apaga o arquivo. Produto inexistente →
// catalogo.ErrProdutoNaoEncontrado. Devolve o produto relido.
func (c *Casos) RemoverImagemProduto(ctx context.Context, id int64) (p *catalogo.Produto, err error) {
	defer vlog.Trace(arqImagens, "Catalogo.RemoverImagemProduto", "id=%d", id)(&err)
	img, err := c.imagensLigadas()
	if err != nil {
		return nil, err
	}
	antiga, err := c.trocarImagem(ctx, img, id, "")
	if err != nil {
		return nil, err
	}
	if antiga != "" {
		log.Printf("[produtos] imagem removida: id=%d arquivo=%s", id, antiga)
		c.removerArquivo(ctx, img, id, antiga)
	}
	return appkit.RelerAtualizado(ctx, "catalogoapp", id, c.produtos.PorID)
}

// AbrirImagemProduto abre o arquivo pelo nome público (rota sem auth).
// Nome inválido, inexistente ou imagens desligadas →
// catalogo.ErrImagemNaoEncontrada.
func (c *Casos) AbrirImagemProduto(nome string) (f io.ReadSeekCloser, modificado time.Time, err error) {
	defer vlog.Trace(arqImagens, "Catalogo.AbrirImagemProduto", "nome_valido=%t", catalogo.NomeImagemValido(nome))(&err)
	img, err := c.imagensLigadas()
	if err != nil || !catalogo.NomeImagemValido(nome) {
		return nil, time.Time{}, catalogo.ErrImagemNaoEncontrada
	}
	return img.Armazenamento.Abrir(nome)
}

// trocarImagem grava nome ("" = NULL) na tx e devolve a imagem anterior.
func (c *Casos) trocarImagem(ctx context.Context, img *Imagens, id int64, nome string) (string, error) {
	var antiga string
	err := img.Tx.WithTx(ctx, func(tx TxImagemProduto) error {
		atual, err := tx.Produtos.ImagemParaAtualizar(ctx, id)
		if err != nil {
			return err
		}
		antiga = atual
		if atual == "" && nome == "" {
			return nil // remoção idempotente: nada a gravar
		}
		return tx.Produtos.DefinirImagem(ctx, id, nome)
	})
	if err != nil {
		return "", err
	}
	vlog.Printf(arqImagens, "Catalogo.trocarImagem", "decisão id=%d tinha_imagem=%t nova_imagem=%t", id, antiga != "", nome != "")
	return antiga, nil
}

// removerArquivo apaga um arquivo sem falhar o caso de uso: o vínculo no
// banco já é o correto; um arquivo que ficar para trás é órfão (card de
// limpeza próprio). Nome fora do padrão (ex.: adulterado no banco) não toca
// o disco.
func (c *Casos) removerArquivo(ctx context.Context, img *Imagens, id int64, nome string) {
	if !catalogo.NomeImagemValido(nome) {
		log.Printf("[produtos] nome de imagem inválido id=%d", id)
		return
	}
	if err := img.Armazenamento.Remover(context.WithoutCancel(ctx), nome); err != nil {
		log.Printf("[produtos] falha ao remover arquivo de imagem id=%d arquivo=%s: %v", id, nome, err)
	}
}
