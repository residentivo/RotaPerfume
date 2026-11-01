package identidade_test

// CHORE-06 (Lote 25): e-mail duplicado continua 409 (sem mudança de
// contrato), mas o caso de uso loga a recusa SEM o e-mail.

import (
	"bytes"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

func TestCHORE06_CriarUsuario_EmailDuplicado_LogSemEmail(t *testing.T) {
	casos := []struct {
		nome      string
		errInsert error
		loga      bool
	}{
		{"e-mail duplicado", identidade.ErrEmailDuplicado, true},
		{"outro erro", errInfra, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out := &bytes.Buffer{}
			prev := log.Writer()
			log.SetOutput(out)
			t.Cleanup(func() { log.SetOutput(prev) })

			a := novoAmbiente()
			a.usuarios.errInserir = c.errInsert
			_, _, err := a.casos().CriarUsuario(ctx, principal(1, kernel.RoleAdmin),
				identidade.DadosUsuario{Nome: "Ana", Email: "Segredo.Dup@RP.com", Role: "normal"})
			assert.ErrorIs(t, err, c.errInsert)
			if c.loga {
				var e *kernel.Erro
				require.True(t, errors.As(err, &e))
				assert.Equal(t, kernel.KindConflito, e.Kind())
				assert.Contains(t, out.String(), "[usuarios] email duplicado recusado (uk_usuarios_email)")
			} else {
				assert.NotContains(t, out.String(), "email duplicado recusado")
			}
			for _, v := range []string{"segredo.dup", "Segredo.Dup", "@rp.com", "@RP.com"} {
				assert.NotContains(t, out.String(), v, "o log nunca leva o e-mail")
			}
		})
	}
}
