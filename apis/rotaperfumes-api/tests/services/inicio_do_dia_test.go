package services_test

// SEC-01/RISCO-01: a carteira criada junto com o cliente
// (CreateClienteNaCarteira) grava data_inicio = meia-noite em time.Local do
// dia corrente, convertendo o instante para o fuso local antes de truncar.
// Assim, um instante UTC já no dia seguinte continua contando como "hoje" no
// fuso do sistema.
//
// Pela API pública não dá para fixar o relógio; a matriz usa fusos extremos
// (UTC-12 e UTC+14, 26h de distância): em qualquer instante real pelo menos
// um deles está num dia de calendário diferente do dia UTC, o que exercita a
// conversão antes do truncamento de forma determinística.

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

// dataInicioCarteira cria cliente+carteira e devolve o data_inicio gravado,
// junto com os instantes imediatamente antes e depois da chamada.
func dataInicioCarteira(t *testing.T) (got, antes, depois time.Time) {
	t.Helper()
	db, mock := dlNewDB(t)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO clientes`).WillReturnResult(sqlmock.NewResult(555, 1))
	mock.ExpectExec(`INSERT INTO carteiras \(cliente_id, vendedor_id, data_inicio, data_fim\)`).
		WithArgs(int64(555), int64(10), dlCapturaTempo{&got}, nil).
		WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectCommit()

	antes = time.Now()
	_, err := services.NewClienteService(db, &config.Config{}).CreateClienteNaCarteira(context.Background(), db,
		services.ClienteInput{CNPJ: "11222333000181", RazaoSocial: "Empresa", Segmento: "varejo", Cidade: "Curitiba", UF: "PR"}, 10)
	depois = time.Now()
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	return got, antes, depois
}

func meiaNoiteLocal(t time.Time) time.Time {
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func TestCreateClienteNaCarteira_DataInicioEhMeiaNoiteLocal(t *testing.T) {
	fusos := append(dlFusos(t), time.FixedZone("-03", -3*3600))
	diaDiferenteDoUTC := 0
	for _, loc := range fusos {
		t.Run(loc.String(), func(t *testing.T) {
			dlComLocal(t, loc)
			got, antes, depois := dataInicioCarteira(t)

			// Tolera a virada de dia entre antes e depois da chamada.
			assert.True(t, got.Equal(meiaNoiteLocal(antes)) || got.Equal(meiaNoiteLocal(depois)),
				"data_inicio %s deve ser a meia-noite local de hoje (%s)", got, meiaNoiteLocal(antes))
			assert.Equal(t, time.Local, got.Location(), "data_inicio em time.Local")
			assert.Zero(t, got.Hour()+got.Minute()+got.Second()+got.Nanosecond(), "sem hora")

			uy, um, ud := antes.UTC().Date()
			ly, lm, ld := antes.In(time.Local).Date()
			if uy != ly || um != lm || ud != ld {
				diaDiferenteDoUTC++
				gy, gm, gd := got.Date()
				assert.Equal(t, []int{ly, int(lm), ld}, []int{gy, int(gm), gd}, "vale o dia local, não o dia UTC")
			}
		})
	}
	assert.Positive(t, diaDiferenteDoUTC, "a matriz deve incluir ao menos um fuso em dia diferente do UTC")
}
