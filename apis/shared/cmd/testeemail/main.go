// Command testeemail é um DIAGNÓSTICO de envio de e-mail: envia UMA mensagem
// de teste usando as mesmas variáveis SMTP_* da API e loga TODOS os passos
// (carga do .env, DNS, TCP, TLS/certificado, banner, EHLO, STARTTLS, AUTH,
// MAIL FROM, RCPT TO, DATA, QUIT) com a transcrição SMTP completa — a senha e
// as credenciais do AUTH nunca são logadas. O log é sempre ligado (não depende
// de VERBOSE). A lógica fica em tools/testeemail; este main é só a
// composition root.
//
// Uso (PowerShell, a partir de apis/shared):
//
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail -to admin@rotaperfumes.com.br -modo ssl
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail -tls-server-name mail.rotaperfumes.com.br
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail -inseguro -from admin@rotaperfumes.com.br
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail -h
//
// ATENÇÃO (PowerShell 5.1): NÃO use -flag=valor quando o valor tem ponto
// (ex.: -to=admin@x.com.br). O PS 5.1 quebra o argumento no primeiro ponto e
// o programa recebe "-to=admin@x" e ".com.br" separados. Use espaço
// (-to admin@x.com.br) ou aspas ("-to=admin@x.com.br").
//
// Flags: -from, -to, -modo auto|ssl|starttls|plain, -tls-server-name,
// -tls-ca-file, -salvar-cert, -inseguro, -timeout, -ehlo. Sai com código 1 em
// caso de falha. Sem flags, -modo/-tls-server-name/-tls-ca-file seguem
// SMTP_TLS_MODE/SMTP_TLS_SERVER_NAME/SMTP_TLS_CA_FILE (as mesmas da API).
//
// Servidor com certificado próprio (autoassinado): grave o certificado com
// -salvar-cert smtp-ca.pem, confira o SHA-256 logado com o administrador do
// servidor e só então use SMTP_TLS_CA_FILE=smtp-ca.pem e
// SMTP_TLS_SERVER_NAME=<nome do SAN do certificado>.
package main

import (
	"log"
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/testeemail"
	// Fixa time.Local em -03:00 para o cabeçalho Date e o corpo do e-mail.
	_ "github.com/rotaperfumes/shared/tz"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	os.Exit(cmdutil.Executar("testeemail", testeemail.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
