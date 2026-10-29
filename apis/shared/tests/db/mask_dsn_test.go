package db_test

// F-08 / ARQ-05 (TestBrain): o mascaramento do DSN no log de db.Open,
// testado de FORA do pacote (convenção: nenhum teste interno em pacote de
// produção). maskDSN só roda depois de um ping bem-sucedido; aqui o ping é
// respondido por um servidor MySQL mínimo em 127.0.0.1 (handshake v10 +
// OK para o login e para o COM_PING), sem banco real. O caso de senha com
// '@' é o GAP SEC-15 e NÃO é fixado aqui.

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/db"
)

// fakeMySQL aceita conexões e responde OK a tudo (login, COM_PING) até o
// COM_QUIT. Devolve o host:porta.
func fakeMySQL(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go atenderMySQL(c)
		}
	}()
	return ln.Addr().String()
}

func escreverPacote(w io.Writer, seq byte, payload []byte) error {
	h := []byte{byte(len(payload)), byte(len(payload) >> 8), byte(len(payload) >> 16), seq}
	_, err := w.Write(append(h, payload...))
	return err
}

func lerPacote(r io.Reader) (seq byte, payload []byte, err error) {
	var h [4]byte
	if _, err = io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	n := int(h[0]) | int(h[1])<<8 | int(h[2])<<16
	payload = make([]byte, n)
	_, err = io.ReadFull(r, payload)
	return h[3], payload, err
}

// pacoteOK: header 0x00, affected rows 0, last insert id 0, status
// SERVER_STATUS_AUTOCOMMIT, 0 warnings.
var pacoteOK = []byte{0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00}

func atenderMySQL(c net.Conn) {
	defer c.Close()
	const (
		clientLongPassword = 0x00000001
		clientProtocol41   = 0x00000200
		clientTransactions = 0x00002000
		clientSecureConn   = 0x00008000
		clientPluginAuth   = 0x00080000
	)
	caps := uint32(clientLongPassword | clientProtocol41 | clientTransactions | clientSecureConn | clientPluginAuth)
	var hs bytes.Buffer
	hs.WriteByte(10)                  // protocol version
	hs.WriteString("8.0.36-fake\x00") // server version
	hs.Write([]byte{1, 0, 0, 0})      // connection id
	hs.WriteString("abcdefgh")        // auth-plugin-data-part-1
	hs.WriteByte(0)                   // filler
	_ = binary.Write(&hs, binary.LittleEndian, uint16(caps))
	hs.WriteByte(45)             // charset utf8mb4
	hs.Write([]byte{0x02, 0x00}) // status flags
	_ = binary.Write(&hs, binary.LittleEndian, uint16(caps>>16))
	hs.WriteByte(21)                   // auth-plugin-data length
	hs.Write(make([]byte, 10))         // reserved
	hs.WriteString("ijklmnopqrst\x00") // auth-plugin-data-part-2 (12 + NUL)
	hs.WriteString("mysql_native_password\x00")
	if escreverPacote(c, 0, hs.Bytes()) != nil {
		return
	}
	// Handshake response do cliente → OK.
	seq, _, err := lerPacote(c)
	if err != nil || escreverPacote(c, seq+1, pacoteOK) != nil {
		return
	}
	for {
		seq, p, err := lerPacote(c)
		if err != nil || len(p) == 0 || p[0] == 0x01 { // COM_QUIT
			return
		}
		if escreverPacote(c, seq+1, pacoteOK) != nil {
			return
		}
	}
}

func TestF08_MaskDSN(t *testing.T) {
	host := fakeMySQL(t)
	casos := []struct {
		nome, dsn, want string
	}{
		{"usuário e senha: senha mascarada", "user:segredo@tcp(" + host + ")/rota?parseTime=true", "user:****@tcp(" + host + ")/rota?parseTime=true"},
		{"sem credenciais: DSN inalterado", "tcp(" + host + ")/rota", "tcp(" + host + ")/rota"},
		{"usuário sem senha: DSN inalterado", "user@tcp(" + host + ")/rota", "user@tcp(" + host + ")/rota"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var buf bytes.Buffer
			saida, flags := log.Writer(), log.Flags()
			log.SetOutput(&buf)
			log.SetFlags(0)
			t.Cleanup(func() { log.SetOutput(saida); log.SetFlags(flags) })

			conn, err := db.Open(c.dsn)
			require.NoError(t, err)
			require.NoError(t, conn.Close())

			linha := strings.TrimSpace(buf.String())
			assert.Equal(t, "[db] conexão MariaDB estabelecida (host="+c.want+")", linha)
			assert.NotContains(t, linha, "segredo", "a senha nunca vai para o log")
		})
	}
}
