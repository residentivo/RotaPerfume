package identidade

// Actions do Turnstile por fluxo (SEC-19). O frontend passa a mesma action
// no widget e o adapter confere a action devolvida pelo siteverify, então um
// token de um fluxo não vale no outro. Não mudar sem mudar o frontend.
const (
	AcaoCaptchaLogin      = "login"
	AcaoCaptchaResetSenha = "reset-senha"
)
