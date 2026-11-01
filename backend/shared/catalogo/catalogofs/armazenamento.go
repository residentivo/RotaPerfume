// Package catalogofs é o adapter de filesystem das imagens de produto
// (IMG-01, porta catalogoapp.ArmazenamentoImagens). Todo acesso passa por um
// *os.Root aberto no diretório configurado (PRODUTO_IMAGENS_DIR): nenhum nome
// escapa do diretório, nem por "..", nem por symlink. Ligado só no bootstrap.
//
// Os nomes em disco são gerados aqui (UUID v4 de crypto/rand + extensão do
// tipo detectado) e conferidos pelo regex de catalogo.NomeImagemValido em
// Remover/Abrir, mesmo quando vêm do banco. Nada do nome do cliente chega ao
// disco nem ao log.
package catalogofs

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/vlog"
)

const (
	arqArmazenamento = "armazenamento.go"
	permDiretorio    = 0o750
	permArquivo      = 0o640
	prefixoTemp      = ".tmp-"
	tentativasNome   = 3
)

// ErrNomeInvalido: nome fora do padrão <uuid-v4><ext> (nada é tocado no
// disco).
var ErrNomeInvalido = errors.New("catalogofs: nome de imagem inválido")

// Armazenamento grava, remove e abre as imagens dentro do Root.
type Armazenamento struct{ root *os.Root }

// Novo cria o diretório (0750) se faltar, abre o Root e faz um teste de
// escrita. Qualquer falha é erro de subida (o bootstrap faz fail-fast).
func Novo(dir string) (*Armazenamento, error) {
	if err := os.MkdirAll(dir, permDiretorio); err != nil {
		return nil, fmt.Errorf("catalogofs: criar diretório: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("catalogofs: abrir diretório: %w", err)
	}
	a := &Armazenamento{root: root}
	if err := a.testarEscrita(); err != nil {
		_ = root.Close()
		return nil, err
	}
	return a, nil
}

// Close fecha o Root.
func (a *Armazenamento) Close() error { return a.root.Close() }

// testarEscrita grava e apaga um arquivo temporário.
func (a *Armazenamento) testarEscrita() error {
	id, err := novoUUID()
	if err != nil {
		return err
	}
	tmp := prefixoTemp + "probe-" + id
	if err := a.escreverArquivo(tmp, []byte("ok")); err != nil {
		return fmt.Errorf("catalogofs: diretório sem permissão de escrita: %w", err)
	}
	if err := a.root.Remove(tmp); err != nil {
		return fmt.Errorf("catalogofs: remover teste de escrita: %w", err)
	}
	return nil
}

// Gravar escreve dados de forma atômica (temporário O_EXCL → Sync → Close
// → Rename) com o nome <uuid-v4><extensao> e devolve o nome. extensao tem
// de ser .jpg, .png ou .webp.
func (a *Armazenamento) Gravar(ctx context.Context, extensao string, dados []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	for range tentativasNome {
		id, err := novoUUID()
		if err != nil {
			return "", err
		}
		nome := id + extensao
		if !catalogo.NomeImagemValido(nome) {
			return "", ErrNomeInvalido
		}
		if _, err := a.root.Lstat(nome); err == nil {
			continue // colisão de UUID: sorteia outro
		}
		if err := a.gravarAtomico(id, nome, dados); err != nil {
			return "", err
		}
		vlog.Printf(arqArmazenamento, "Armazenamento.Gravar", "arquivo gravado nome=%s bytes=%d", nome, len(dados))
		return nome, nil
	}
	return "", errors.New("catalogofs: não foi possível gerar um nome livre")
}

func (a *Armazenamento) gravarAtomico(id, nome string, dados []byte) error {
	tmp := prefixoTemp + id
	if err := a.escreverArquivo(tmp, dados); err != nil {
		return fmt.Errorf("catalogofs: gravar temporário: %w", err)
	}
	if err := a.root.Rename(tmp, nome); err != nil {
		_ = a.root.Remove(tmp)
		return fmt.Errorf("catalogofs: renomear temporário: %w", err)
	}
	return nil
}

// escreverArquivo cria nome com O_EXCL (0640), escreve, faz Sync e fecha.
// Se o OpenFile falha (ex.: nome já existe), nada é removido: o arquivo não
// é desta chamada. Se a falha vem depois de criado, o arquivo é apagado.
func (a *Armazenamento) escreverArquivo(nome string, dados []byte) error {
	f, err := a.root.OpenFile(nome, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permArquivo)
	if err != nil {
		return err
	}
	if err := escreverESincronizar(f, dados); err != nil {
		_ = a.root.Remove(nome)
		return err
	}
	return nil
}

// escreverESincronizar escreve dados, faz Sync e fecha f (sempre fecha).
func escreverESincronizar(f *os.File, dados []byte) error {
	if _, err := f.Write(dados); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Remover apaga o arquivo. Nome fora do padrão → ErrNomeInvalido e ctx já
// cancelado → ctx.Err(), ambos sem tocar o disco; arquivo inexistente não é
// erro (idempotente).
func (a *Armazenamento) Remover(ctx context.Context, nome string) error {
	if !catalogo.NomeImagemValido(nome) {
		return ErrNomeInvalido
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err := a.root.Remove(nome)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("catalogofs: remover: %w", err)
	}
	vlog.Printf(arqArmazenamento, "Armazenamento.Remover", "arquivo removido nome=%s existia=%t", nome, err == nil)
	return nil
}

// Abrir abre o arquivo para leitura e devolve a data de modificação. Nome
// fora do padrão, inexistente, symlink ou não regular →
// catalogo.ErrImagemNaoEncontrada.
func (a *Armazenamento) Abrir(nome string) (io.ReadSeekCloser, time.Time, error) {
	if !catalogo.NomeImagemValido(nome) {
		return nil, time.Time{}, catalogo.ErrImagemNaoEncontrada
	}
	info, err := a.root.Lstat(nome)
	if err != nil || !info.Mode().IsRegular() {
		return nil, time.Time{}, catalogo.ErrImagemNaoEncontrada
	}
	f, err := a.root.Open(nome)
	if err != nil {
		return nil, time.Time{}, catalogo.ErrImagemNaoEncontrada
	}
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, time.Time{}, catalogo.ErrImagemNaoEncontrada
	}
	return f, info.ModTime(), nil
}

// novoUUID gera um UUID v4 minúsculo (RFC 9562) com crypto/rand.
func novoUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("catalogofs: gerar uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
