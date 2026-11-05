package config_test

// TST-20 (Lote 35): ramo de erro do integracao.Abrir (db.Open/Ping falha ->
// t.Fatalf, retorno nil e nenhum Cleanup), sem banco real: a porta é a de
// um listener do loopback aberto e fechado na hora (nada escuta nela).
// Também cobre o caminho de sucesso (pool aberto, um Cleanup que fecha) com
// um MariaDB FALSO em processo (só handshake + OK; nenhum banco real).

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/testkit/integracao"
)

// portaFechadaTST20 devolve uma porta do loopback que estava livre há pouco.
func portaFechadaTST20(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	porta := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return strconv.Itoa(porta)
}

func TestTST20_Abrir_FalhaNoOpen(t *testing.T) {
	casos := []struct {
		nome, host string
	}{
		{"127.0.0.1", "127.0.0.1"},
		{"localhost", "localhost"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			porta := portaFechadaTST20(t)
			t.Setenv("INTEGRATION", "1")
			t.Setenv("DB_HOST", c.host)
			t.Setenv("DB_PORT", porta)
			t.Setenv("DB_NAME", "rotaperfumes_tst20")
			t.Setenv("DB_USUARIO", "u-tst20")
			t.Setenv("DB_SENHA", "s-tst20-segredo")
			t.Setenv("DB_TLS_CA", "")
			t.Setenv("DB_TLS_SERVER_NAME", "")
			t.Setenv("DB_TLS_REQUIRED", "")

			// A guarda passa (loopback na allowlist): a falha vem do Open.
			cfgTB := &tbFalso{}
			cfg := integracao.Config(cfgTB)
			require.NotNil(t, cfg, cfgTB.msg)
			assert.False(t, cfgTB.falhou || cfgTB.pulou)

			tb := &tbFalso{}
			conn := integracao.Abrir(tb)
			assert.Nil(t, conn)
			assert.True(t, tb.falhou, "t.Fatalf")
			assert.False(t, tb.pulou, "não pula: o banco era esperado")
			assert.Contains(t, tb.msg, "indisponível")
			assert.Contains(t, tb.msg, "MariaDB de integração indisponível")
			assert.NotContains(t, tb.msg, "s-tst20-segredo", "a senha não aparece na falha")
			assert.Empty(t, tb.limpezas, "nenhum Cleanup: nada ficou aberto")
		})
	}
}

func pacoteTST20(w io.Writer, seq byte, payload []byte) error {
	h := []byte{byte(len(payload)), byte(len(payload) >> 8), byte(len(payload) >> 16), seq}
	_, err := w.Write(append(h, payload...))
	return err
}

func lerPacoteTST20(r io.Reader) (byte, []byte, error) {
	var h [4]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	p := make([]byte, int(h[0])|int(h[1])<<8|int(h[2])<<16)
	_, err := io.ReadFull(r, p)
	return h[3], p, err
}

// mariadbFalsoTST20: handshake v10 sem TLS (mysql_native_password), OK no
// login e OK em todo comando até o COM_QUIT. Conta os logins e os QUIT.
func mariadbFalsoTST20(t *testing.T) (porta string, logins, quits *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	logins, quits = &atomic.Int32{}, &atomic.Int32{}
	ok := []byte{0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00}
	caps := uint32(0x00000001 | 0x00000200 | 0x00002000 | 0x00008000 | 0x00080000)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var hs bytes.Buffer
				hs.WriteByte(10)
				hs.WriteString("11.8.8-MariaDB-falso-tst20\x00")
				hs.Write([]byte{1, 0, 0, 0})
				hs.WriteString("abcdefgh")
				hs.WriteByte(0)
				_ = binary.Write(&hs, binary.LittleEndian, uint16(caps))
				hs.WriteByte(45)
				hs.Write([]byte{0x02, 0x00})
				_ = binary.Write(&hs, binary.LittleEndian, uint16(caps>>16))
				hs.WriteByte(21)
				hs.Write(make([]byte, 10))
				hs.WriteString("ijklmnopqrst\x00")
				hs.WriteString("mysql_native_password\x00")
				if pacoteTST20(c, 0, hs.Bytes()) != nil {
					return
				}
				seq, _, err := lerPacoteTST20(c)
				if err != nil || pacoteTST20(c, seq+1, ok) != nil {
					return
				}
				logins.Add(1)
				for {
					seq, p, err := lerPacoteTST20(c)
					if err != nil || len(p) == 0 {
						return
					}
					if p[0] == 0x01 { // COM_QUIT
						quits.Add(1)
						return
					}
					if pacoteTST20(c, seq+1, ok) != nil {
						return
					}
				}
			}(c)
		}
	}()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port), logins, quits
}

func TestTST20_Abrir_SucessoRegistraCleanup(t *testing.T) {
	porta, logins, _ := mariadbFalsoTST20(t)
	ambienteIntegracao(t, credenciais(map[string]string{"DB_HOST": "127.0.0.1", "DB_PORT": porta, "DB_NAME": "rotaperfumes_tst20"}))

	tb := &tbFalso{}
	conn := integracao.Abrir(tb)
	require.NotNil(t, conn, tb.msg)
	assert.False(t, tb.falhou || tb.pulou, tb.msg)
	assert.GreaterOrEqual(t, logins.Load(), int32(1), "db.Open fez o Ping no MariaDB falso")
	require.Len(t, tb.limpezas, 1, "um Cleanup: fecha o pool")
	require.NoError(t, conn.Ping())

	tb.limpezas[0]()
	assert.Error(t, conn.Ping(), "pool fechado pelo Cleanup")
}
