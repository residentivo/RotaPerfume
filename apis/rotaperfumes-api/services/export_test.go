package services

import "time"

// SetRefreshClockForTest expõe o relógio injetável do RefreshTokenService
// para os testes do pacote services_test (compilado só em `go test`).
func SetRefreshClockForTest(s *RefreshTokenService, now func() time.Time) {
	s.setClock(now)
}
