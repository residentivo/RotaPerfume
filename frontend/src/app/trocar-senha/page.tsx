"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Turnstile, TurnstileHandle, isTurnstileEnabled } from "@/components/ui/Turnstile";
import { apiChangePassword } from "@/lib/api";
import { getUser } from "@/lib/auth";
import { useTimeoutSeguro } from "@/lib/useMensagemTemporaria";
import { vlog } from "@/lib/vlog";

const FILE = "trocar-senha/page.tsx";

export default function TrocarSenhaPage() {
  vlog(FILE, "TrocarSenhaPage", "obtendo router de navegacao");
  const router = useRouter();
  vlog(FILE, "TrocarSenhaPage", "inicializando estado do campo senha atual");
  const [senhaAtual, setSenhaAtual] = useState("");
  vlog(FILE, "TrocarSenhaPage", "inicializando estado do campo nova senha");
  const [novaSenha, setNovaSenha] = useState("");
  vlog(FILE, "TrocarSenhaPage", "inicializando estado do campo confirmar senha");
  const [confirmarSenha, setConfirmarSenha] = useState("");
  vlog(FILE, "TrocarSenhaPage", "inicializando estado de loading");
  const [loading, setLoading] = useState(false);
  vlog(FILE, "TrocarSenhaPage", "inicializando estado de erro geral");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "TrocarSenhaPage", "inicializando estado de mensagem de sucesso");
  const [success, setSuccess] = useState<string | null>(null);
  vlog(FILE, "TrocarSenhaPage", "inicializando estado de erros por campo");
  const [fieldErrors, setFieldErrors] = useState<{
    senhaAtual?: string;
    novaSenha?: string;
    confirmarSenha?: string;
  }>({});
  vlog(FILE, "TrocarSenhaPage", "inicializando estado do token do captcha");
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  vlog(FILE, "TrocarSenhaPage", "criando ref do widget Turnstile");
  const turnstileRef = useRef<TurnstileHandle>(null);
  vlog(FILE, "TrocarSenhaPage", "obtendo agendador de timeout seguro");
  const { agendar } = useTimeoutSeguro();

  vlog(FILE, "TrocarSenhaPage", "registrando efeito de verificacao de sessao");
  useEffect(() => {
    // Verificar se o usuário está logado
    vlog(FILE, "TrocarSenhaPage.useEffect", "verificando se ha usuario logado");
    if (!getUser()) {
      vlog(FILE, "TrocarSenhaPage.useEffect", "sem usuario; redirecionando para /login");
      router.replace("/login");
      return;
    }
  }, [router]);

  vlog(FILE, "TrocarSenhaPage", "definindo funcao validate");
  const validate = (): boolean => {
    vlog(FILE, "TrocarSenhaPage.validate", "criando objeto de erros por campo");
    const errs: {
      senhaAtual?: string;
      novaSenha?: string;
      confirmarSenha?: string;
    } = {};

    vlog(FILE, "TrocarSenhaPage.validate", "verificando se senha atual esta vazia");
    if (!senhaAtual) {
      vlog(FILE, "TrocarSenhaPage.validate", "senha atual vazia; registrando erro");
      errs.senhaAtual = "Senha atual e obrigatoria";
    }

    vlog(FILE, "TrocarSenhaPage.validate", "verificando regras da nova senha");
    if (!novaSenha) {
      vlog(FILE, "TrocarSenhaPage.validate", "nova senha vazia; registrando erro");
      errs.novaSenha = "Nova senha e obrigatoria";
    } else if (senhaAtual && novaSenha === senhaAtual) {
      vlog(FILE, "TrocarSenhaPage.validate", "nova senha igual a atual; registrando erro");
      errs.novaSenha = "A nova senha nao pode ser igual a senha atual";
    } else if (novaSenha.length < 8) {
      vlog(FILE, "TrocarSenhaPage.validate", "nova senha curta demais; registrando erro");
      errs.novaSenha = "Nova senha deve ter pelo menos 8 caracteres";
    } else {
      vlog(FILE, "TrocarSenhaPage.validate", "contando classes de caracteres da nova senha");
      const classesCount = [
        /[a-z]/.test(novaSenha),
        /[A-Z]/.test(novaSenha),
        /[0-9]/.test(novaSenha),
        /[^a-zA-Z0-9]/.test(novaSenha),
      ].filter(Boolean).length;
      vlog(FILE, "TrocarSenhaPage.validate", "verificando se ha ao menos 3 classes de caracteres");
      if (classesCount < 3) {
        vlog(FILE, "TrocarSenhaPage.validate", "complexidade insuficiente; registrando erro");
        errs.novaSenha =
          "Nova senha deve conter ao menos 3 dos 4 tipos: letra minuscula, letra maiuscula, digito e simbolo";
      }
    }

    vlog(FILE, "TrocarSenhaPage.validate", "verificando confirmacao da senha");
    if (!confirmarSenha) {
      vlog(FILE, "TrocarSenhaPage.validate", "confirmacao vazia; registrando erro");
      errs.confirmarSenha = "Confirmacao de senha e obrigatoria";
    } else if (novaSenha !== confirmarSenha) {
      vlog(FILE, "TrocarSenhaPage.validate", "confirmacao diferente da nova senha; registrando erro");
      errs.confirmarSenha = "As senhas nao conferem";
    }

    vlog(FILE, "TrocarSenhaPage.validate", "atualizando erros por campo (qtd=%d)", Object.keys(errs).length);
    setFieldErrors(errs);
    return Object.keys(errs).length === 0;
  };

  vlog(FILE, "TrocarSenhaPage", "definindo handler handleSubmit");
  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    vlog(FILE, "TrocarSenhaPage.handleSubmit", "prevenindo submit padrao do formulario");
    e.preventDefault();
    vlog(FILE, "TrocarSenhaPage.handleSubmit", "limpando erro geral");
    setError(null);
    vlog(FILE, "TrocarSenhaPage.handleSubmit", "limpando mensagem de sucesso");
    setSuccess(null);
    vlog(FILE, "TrocarSenhaPage.handleSubmit", "validando campos do formulario");
    if (!validate()) return;

    vlog(FILE, "TrocarSenhaPage.handleSubmit", "ativando loading");
    setLoading(true);
    vlog(FILE, "TrocarSenhaPage.handleSubmit", "iniciando chamada de troca de senha");
    try {
      vlog(FILE, "TrocarSenhaPage.handleSubmit", "chamando apiChangePassword (captcha presente=%s)", captchaToken !== null);
      await apiChangePassword(senhaAtual, novaSenha, captchaToken ?? undefined);
      vlog(FILE, "TrocarSenhaPage.handleSubmit", "senha alterada; exibindo mensagem de sucesso");
      setSuccess("Senha alterada com sucesso!");
      // Redirecionar para dashboard após breve delay. FE-11: o timer e
      // cancelado se a tela desmontar antes (o usuario ja saiu da tela).
      vlog(FILE, "TrocarSenhaPage.handleSubmit", "agendando redirecionamento para /dashboard em 1500ms");
      agendar(() => router.replace("/dashboard"), 1500);
    } catch (err) {
      vlog(FILE, "TrocarSenhaPage.handleSubmit", "falha na troca de senha; extraindo mensagem do erro");
      const rawMessage = err instanceof Error ? err.message : "";
      vlog(FILE, "TrocarSenhaPage.handleSubmit", "verificando se a falha foi de captcha");
      const isCaptchaFailure = /captcha|turnstile/i.test(rawMessage);
      // Falha de captcha: mensagem genérica, sem expor detalhes técnicos do
      // motivo. Demais falhas mantêm a mensagem retornada pela API.
      vlog(FILE, "TrocarSenhaPage.handleSubmit", "exibindo mensagem de erro (falha de captcha=%s)", isCaptchaFailure);
      setError(
        isCaptchaFailure
          ? "Nao foi possivel validar o captcha. Tente novamente."
          : rawMessage || "Erro ao alterar senha"
      );
      // Token do Turnstile e de uso unico: apos qualquer falha, reseta o
      // widget para forcar um novo desafio antes de reenviar.
      vlog(FILE, "TrocarSenhaPage.handleSubmit", "descartando token do captcha usado");
      setCaptchaToken(null);
      vlog(FILE, "TrocarSenhaPage.handleSubmit", "resetando widget Turnstile");
      turnstileRef.current?.reset();
    } finally {
      vlog(FILE, "TrocarSenhaPage.handleSubmit", "desativando loading");
      setLoading(false);
    }
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-gradient-to-br from-slate-900 via-blue-900 to-slate-900 px-3 py-8 sm:px-4">
      {/* Decorative blobs */}
      <div className="pointer-events-none absolute -top-32 -left-32 h-96 w-96 rounded-full bg-blue-500/30 blur-3xl" />
      <div className="pointer-events-none absolute -bottom-32 -right-32 h-96 w-96 rounded-full bg-sky-500/20 blur-3xl" />

      <div className="relative z-10 w-full max-w-md">
        <div className="mb-6 text-center">
          <div className="mx-auto mb-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-600 shadow-lg shadow-primary-500/40">
            <span className="text-2xl font-bold text-white">R</span>
          </div>
          <h1 className="text-3xl font-bold tracking-tight text-white">
            Rota<span className="text-primary-400">Perfumes</span>
          </h1>
          <p className="mt-1 text-sm text-slate-300">
            Sistema de Gestao de Vendas
          </p>
        </div>

        <Card padded={false} className="border-slate-200/20 bg-white/95 p-4 shadow-2xl backdrop-blur sm:p-6">
          <div className="mb-1 flex items-center gap-2">
            <svg
              className="h-6 w-6 text-primary-600"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
              />
            </svg>
            <h2 className="text-xl font-semibold text-slate-900">
              Trocar Senha
            </h2>
          </div>
          <p className="mb-6 text-sm text-slate-500">
            Por seguranca, voce precisa alterar sua senha antes de continuar.
          </p>

          {error && (
            <div className="mb-4">
              <Alert variant="error" onClose={() => setError(null)}>
                {error}
              </Alert>
            </div>
          )}

          {success && (
            <div className="mb-4">
              <Alert variant="success" onClose={() => setSuccess(null)}>
                {success}
              </Alert>
            </div>
          )}

          <form onSubmit={handleSubmit} noValidate className="space-y-4">
            <Input
              label="Senha Atual"
              type="password"
              placeholder="********"
              autoComplete="current-password"
              value={senhaAtual}
              onChange={(e) => setSenhaAtual(e.target.value)}
              error={fieldErrors.senhaAtual}
              icon={
                <svg
                  className="h-5 w-5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                  />
                </svg>
              }
            />

            <Input
              label="Nova Senha"
              type="password"
              placeholder="********"
              autoComplete="new-password"
              value={novaSenha}
              onChange={(e) => setNovaSenha(e.target.value)}
              error={fieldErrors.novaSenha}
              helperText="Minimo 8 caracteres, com pelo menos 3 de: minuscula, maiuscula, numero, simbolo"
              icon={
                <svg
                  className="h-5 w-5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
                  />
                </svg>
              }
            />

            <Input
              label="Confirmar Nova Senha"
              type="password"
              placeholder="********"
              autoComplete="new-password"
              value={confirmarSenha}
              onChange={(e) => setConfirmarSenha(e.target.value)}
              error={fieldErrors.confirmarSenha}
              icon={
                <svg
                  className="h-5 w-5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              }
            />

            <Turnstile
              ref={turnstileRef}
              onVerify={(token) => setCaptchaToken(token)}
              onExpire={() => setCaptchaToken(null)}
              onError={() => setCaptchaToken(null)}
            />

            <Button
              type="submit"
              variant="primary"
              size="lg"
              loading={loading}
              disabled={isTurnstileEnabled && !captchaToken}
              fullWidth
            >
              {loading ? "Alterando..." : "Alterar Senha"}
            </Button>
          </form>

          <p className="mt-6 text-center text-xs text-slate-400">
            Esta acao e obrigatoria para continuar usando o sistema.
          </p>
        </Card>
      </div>
    </div>
  );
}
