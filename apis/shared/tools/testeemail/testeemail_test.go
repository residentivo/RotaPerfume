package testeemail

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEscolherModo(t *testing.T) {
	casos := []struct {
		modo, porta, esperado string
	}{
		{"auto", "465", ModoSSL},
		{"auto", "587", ModoStartTLS},
		{"auto", "25", ModoStartTLS},
		{"", "465", ModoSSL},
		{"AUTO", " 465 ", ModoSSL},
		{"ssl", "587", ModoSSL},
		{"starttls", "465", ModoStartTLS},
		{"plain", "465", ModoPlain},
	}
	for _, c := range casos {
		obtido, err := EscolherModo(c.modo, c.porta)
		if err != nil {
			t.Fatalf("EscolherModo(%q,%q) erro inesperado: %v", c.modo, c.porta, err)
		}
		if obtido != c.esperado {
			t.Errorf("EscolherModo(%q,%q) = %q, esperado %q", c.modo, c.porta, obtido, c.esperado)
		}
	}
	if _, err := EscolherModo("tls", "465"); err == nil {
		t.Error("EscolherModo com modo inválido deveria falhar")
	}
}

func TestValidarEndereco(t *testing.T) {
	end, err := ValidarEndereco("  admin@rotaperfumes.com.br ")
	if err != nil || end != "admin@rotaperfumes.com.br" {
		t.Fatalf("esperado endereço aparado, obtido %q, %v", end, err)
	}
	end, err = ValidarEndereco("Admin <admin@rotaperfumes.com.br>")
	if err != nil || end != "admin@rotaperfumes.com.br" {
		t.Fatalf("esperado addr-spec, obtido %q, %v", end, err)
	}
	for _, ruim := range []string{"a@b.com\r\nBcc: x@y.com", "a@b.com\n", "\ra@b.com"} {
		if _, err := ValidarEndereco(ruim); !errors.Is(err, ErrQuebraDeLinha) {
			t.Errorf("ValidarEndereco(%q) deveria rejeitar CR/LF, erro = %v", ruim, err)
		}
	}
	for _, ruim := range []string{"", "   ", "sem-arroba", "admin@"} {
		if _, err := ValidarEndereco(ruim); err == nil {
			t.Errorf("ValidarEndereco(%q) deveria falhar", ruim)
		}
	}
}

func TestDominio(t *testing.T) {
	if d := Dominio("henrique.rodrigues@rotaperfumes.com.br"); d != "rotaperfumes.com.br" {
		t.Errorf("Dominio = %q", d)
	}
	if d := Dominio("sem-arroba"); d != "localhost" {
		t.Errorf("Dominio sem @ = %q", d)
	}
}

func TestMontarMensagem(t *testing.T) {
	data := time.Date(2026, 10, 9, 14, 30, 0, 0, time.FixedZone("-03", -3*3600))
	msg, err := MontarMensagem(Mensagem{
		De:        "henrique.rodrigues@rotaperfumes.com.br",
		Para:      "admin@rotaperfumes.com.br",
		Assunto:   AssuntoPadrao,
		Corpo:     "linha 1\nlinha 2",
		Data:      data,
		MessageID: "<1.testeemail@rotaperfumes.com.br>",
	})
	if err != nil {
		t.Fatalf("MontarMensagem erro: %v", err)
	}
	s := string(msg)
	esperados := []string{
		"From: henrique.rodrigues@rotaperfumes.com.br\r\n",
		"To: admin@rotaperfumes.com.br\r\n",
		"Subject: =?utf-8?q?",
		"Date: Fri, 09 Oct 2026 14:30:00 -0300\r\n",
		"Message-ID: <1.testeemail@rotaperfumes.com.br>\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=\"utf-8\"\r\n",
		"Content-Transfer-Encoding: 8bit\r\n",
		"\r\n\r\nlinha 1\r\nlinha 2",
	}
	for _, e := range esperados {
		if !strings.Contains(s, e) {
			t.Errorf("mensagem não contém %q:\n%s", e, s)
		}
	}
	cabecalhos := strings.SplitN(s, "\r\n\r\n", 2)[0]
	if strings.ContainsAny(strings.ReplaceAll(cabecalhos, "\r\n", ""), "\r\n—çã") {
		t.Errorf("cabeçalhos devem ser ASCII e sem quebras soltas:\n%s", cabecalhos)
	}
}

func TestMontarMensagemRejeitaCRLF(t *testing.T) {
	base := Mensagem{De: "a@b.com", Para: "c@d.com", Assunto: "x", MessageID: "<1@b.com>", Data: time.Now()}
	casos := map[string]func(m *Mensagem){
		"From":    func(m *Mensagem) { m.De = "a@b.com\r\nBcc: x@y.com" },
		"To":      func(m *Mensagem) { m.Para = "c@d.com\n" },
		"Subject": func(m *Mensagem) { m.Assunto = "oi\r\nBcc: x@y.com" },
	}
	for nome, alterar := range casos {
		m := base
		alterar(&m)
		if _, err := MontarMensagem(m); !errors.Is(err, ErrQuebraDeLinha) {
			t.Errorf("%s com CR/LF deveria ser rejeitado, erro = %v", nome, err)
		}
	}
}

func TestCorpoPadrao(t *testing.T) {
	data := time.Date(2026, 10, 9, 14, 30, 5, 0, time.FixedZone("-03", -3*3600))
	c := CorpoPadrao(data, "192.168.168.106", "465", "ssl", "mail.x.com.br")
	for _, e := range []string{"09/10/2026 14:30:05 -03:00", "192.168.168.106:465", "ssl", "mail.x.com.br"} {
		if !strings.Contains(c, e) {
			t.Errorf("corpo não contém %q", e)
		}
	}
}
