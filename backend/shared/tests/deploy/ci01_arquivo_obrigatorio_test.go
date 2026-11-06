package deploy_test

// CI-01 (Lote 38, TestBrain): o comportamento de arquivoObrigatorio com
// arquivo AUSENTE (t.Fatalf no CI, t.Skip fora). t.Fatalf não é observável
// de dentro do mesmo teste, então o caso roda num subprocesso do próprio
// binário de teste (os.Args[0] -test.run), que herda o CI do caso (t.Setenv).

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ci01VarFilho marca o subprocesso que chama arquivoObrigatorio.
const ci01VarFilho = "RP_CI01_ARQUIVO_AUSENTE_FILHO"

// TestCI01_FilhoArquivoAusente só faz algo no subprocesso.
func TestCI01_FilhoArquivoAusente(t *testing.T) {
	if os.Getenv(ci01VarFilho) != "1" {
		t.Skip("executado só como subprocesso de TestCI01_ArquivoAusenteFalhaNoCI")
	}
	arquivoObrigatorio(t, "scripts", "ci01-nao-existe.sh")
	t.Log("CI01-NAO-PAROU")
}

func TestCI01_ArquivoAusenteFalhaNoCI(t *testing.T) {
	if os.Getenv(ci01VarFilho) == "1" {
		t.Skip("no subprocesso")
	}
	casos := []struct {
		ci         string
		falha      bool
		marcaSaida string
	}{
		{"true", true, "ausente no CI"},
		{"1", true, "ausente no CI"},
		{"", false, "ausente (checkout parcial)"},
		{"false", false, "ausente (checkout parcial)"},
		{"yes", false, "ausente (checkout parcial)"},
	}
	for _, c := range casos {
		t.Run("CI="+c.ci, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestCI01_FilhoArquivoAusente$", "-test.v", "-test.count=1")
			// O subprocesso herda o ambiente (cmd.Env nil) com o CI do caso;
			// t.Setenv restaura os dois ao fim do subteste.
			t.Setenv("CI", c.ci)
			t.Setenv(ci01VarFilho, "1")
			out, err := cmd.CombinedOutput()
			saida := string(out)
			assert.Contains(t, saida, c.marcaSaida)
			assert.Contains(t, saida, "ci01-nao-existe.sh")
			assert.NotContains(t, saida, "CI01-NAO-PAROU", "o teste não segue depois do arquivo ausente")
			if c.falha {
				require.Error(t, err, "no CI o arquivo ausente falha: %s", saida)
				assert.Contains(t, saida, "--- FAIL: TestCI01_FilhoArquivoAusente")
			} else {
				require.NoError(t, err, "fora do CI o arquivo ausente pula: %s", saida)
				assert.Contains(t, saida, "--- SKIP: TestCI01_FilhoArquivoAusente")
			}
		})
	}
}
