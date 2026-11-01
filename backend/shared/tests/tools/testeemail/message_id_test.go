// Package testeemail_test testa por fora o tools/testeemail (desde o ARQ-07,
// todos os testes do pacote ficam aqui, §7.2-8).
package testeemail_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/tools/testeemail"
)

// O Message-ID leva o instante, 8 bytes aleatórios em hex e o domínio; dois
// IDs no mesmo instante não se repetem (ARQ-06: ramo sem teste).
func TestGerarMessageID(t *testing.T) {
	agora := time.Unix(0, 1700000000123456789)
	id := testeemail.GerarMessageID(agora, "exemplo.com")
	assert.Regexp(t, regexp.MustCompile(`^<1700000000123456789\.[0-9a-f]{16}\.testeemail@exemplo\.com>$`), id)
	assert.NotEqual(t, id, testeemail.GerarMessageID(agora, "exemplo.com"))
}
