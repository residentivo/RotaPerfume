package tz_test

import (
	"testing"
	"time"

	"github.com/rotaperfumes/shared/tz"
)

func TestInit_FixaTimeLocalEmMenos3SemDST(t *testing.T) {
	if time.Local != tz.Local {
		t.Fatalf("time.Local não foi fixado pelo pacote tz: %v", time.Local)
	}
	// 2018-11-04 foi início de horário de verão em America/Sao_Paulo:
	// a meia-noite não existia. Com o fuso fixo, ela existe.
	d, err := time.ParseInLocation("2006-01-02", "2018-11-04", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if d.Day() != 4 || d.Hour() != 0 {
		t.Fatalf("esperado 2018-11-04 00:00, obtido %v", d)
	}
	if _, off := d.Zone(); off != -3*60*60 {
		t.Fatalf("offset esperado -10800, obtido %d", off)
	}
}
