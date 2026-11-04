package deploy_test

// DEPLOY-08: o salto em DOCKER-USER precisa de --ctdir ORIGINAL. Sem ele, a
// resposta do container (origem 172.30.42.2) casa o mesmo tuple original,
// salta para ROTAPERFUMES-DB, não casa nenhum -s de dev e cai no DROP: da
// máquina liberada o TCP dava timeout.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Salto de uma execução anterior ao DEPLOY-08 (mesmo IP e porta, sem ctdir).
const saltoSemCtdir = "-A DOCKER-USER -p tcp -m conntrack --ctorigdst 192.168.168.106/32 --ctorigdstport 3306 -j ROTAPERFUMES-DB"

func TestDEPLOY08_Firewall_SaltoComCtdirOriginal(t *testing.T) {
	for _, com32 := range []bool{false, true} {
		nome := "iptables_-S_sem_32"
		if com32 {
			nome = "iptables_-S_com_32"
		}
		t.Run(nome, func(t *testing.T) {
			e := novoEnsaio(t)
			if com32 {
				e.setState("ctorigdst_com_32", "")
			}
			e.setState("chain_DOCKER-USER", saltoSemCtdir+"\n-A DOCKER-USER -j RETURN\n")
			r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.105")
			require.NoError(t, r.err, r.out)

			assert.Equal(t, saltoFW+"\n-A DOCKER-USER -j RETURN\n", e.state("chain_DOCKER-USER"),
				"só o salto novo (com --ctdir ORIGINAL); o antigo sem ctdir sai, mesmo com IP/porta iguais")

			// O novo entra (-I) antes de o antigo sair (-D): sem janela aberta.
			log := e.log()
			iIns := strings.Index(log, "iptables -w -I DOCKER-USER 1 -p tcp -m conntrack --ctorigdst 192.168.168.106 --ctorigdstport 3306 --ctdir ORIGINAL -j ROTAPERFUMES-DB")
			iDel := strings.Index(log, "iptables -w -D DOCKER-USER -p tcp -m conntrack --ctorigdst 192.168.168.106")
			require.GreaterOrEqual(t, iIns, 0, log)
			require.GreaterOrEqual(t, iDel, 0, log)
			assert.Less(t, iIns, iDel)
			assert.NotContains(t, log[iDel:strings.Index(log[iDel:], "\n")+iDel], "--ctdir", "o removido é o antigo")

			fw := e.arq("sbin/rotaperfumes-db-firewall")
			assert.Contains(t, fw, `--ctorigdstport "$PORTA" --ctdir ORIGINAL -j "$CHAIN"`)
			assert.Contains(t, log, "iptables -w -C DOCKER-USER -p tcp -m conntrack --ctorigdst 192.168.168.106 --ctorigdstport 3306 --ctdir ORIGINAL -j ROTAPERFUMES-DB",
				"conferência pós-unit exige o salto com ctdir")

			// Reexecução: nada muda nem duplica.
			r2 := e.rodar("--dev-ip", "192.168.168.105")
			require.NoError(t, r2.err, r2.out)
			assert.Equal(t, saltoFW+"\n-A DOCKER-USER -j RETURN\n", e.state("chain_DOCKER-USER"))
		})
	}
}
