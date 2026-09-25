// Package tz fixa o fuso horário do processo (time.Local) em UTC-03:00 sem
// horário de verão. Deve ser importado em branco (import _ ".../shared/tz")
// por TODO binário que abre conexão com o banco, e o import precisa acontecer
// antes do sql.Open: o driver go-sql-driver/mysql resolve loc=Local no parse
// do DSN, capturando o time.Local vigente naquele momento.
//
// Motivo (RISCO-01): o Brasil não tem horário de verão desde 2019, mas a base
// tz "America/Sao_Paulo" (TZ do container Linux/Docker) ainda carrega as
// transições históricas. Nas datas de início do horário de verão antigo (ex.:
// 2018-11-04) a meia-noite não existe, e time.ParseInLocation devolvia 23:00
// do dia anterior — datas puras (DATE) eram gravadas com um dia a menos.
// Um fuso fixo de -03:00 elimina essas transições e torna o comportamento
// idêntico em Windows, Linux e Docker, independente da variável TZ.
package tz

import "time"

// Nome e offset do fuso fixo usado pelo sistema.
const (
	Nome           = "-03"
	OffsetSegundos = -3 * 60 * 60
)

// Local é o *time.Location fixo aplicado a time.Local.
var Local = time.FixedZone(Nome, OffsetSegundos)

func init() {
	time.Local = Local
}
