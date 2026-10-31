// Command gerarimagens gera uma imagem PNG por produto com o nome escrito na
// diagonal (IMG-01, D4) e a grava pelo mesmo caso de uso do upload
// (DefinirImagemProduto). A lógica fica em internal/gerarimagens; este main
// é só a composition root.
//
// Uso (a partir de apis/rotaperfumes-api):
//
//	go run ./cmd/gerarimagens -dry-run                  # só lista, não grava
//	go run ./cmd/gerarimagens -confirmar                # grava (pula quem já tem imagem)
//	go run ./cmd/gerarimagens -confirmar -sobrescrever  # troca também as existentes
//
// Usa o mesmo PRODUTO_IMAGENS_DIR da API (padrão ./data/produtos-imagens,
// relativo ao diretório de execução).
package main

import (
	"os"

	"github.com/rotaperfumes/rotaperfumes-api/internal/gerarimagens"
	"github.com/rotaperfumes/shared/cmdutil"
)

func main() {
	os.Exit(cmdutil.Executar(gerarimagens.Tag, gerarimagens.CLI(gerarimagens.DepsPadrao()), os.Args[1:], cmdutil.AmbientePadrao()))
}
