"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Turnstile, TurnstileHandle, isTurnstileEnabled } from "@/components/ui/Turnstile";
import { apiLogin } from "@/lib/api";
import { saveUser, getUser, isAdmin } from "@/lib/auth";
import { vlog } from "@/lib/vlog";

const FILE = "login/page.tsx";

export default function LoginPage() {
  vlog(FILE, "LoginPage", "obtendo router de navegacao");
  const router = useRouter();
  vlog(FILE, "LoginPage", "inicializando estado do campo email");
  const [email, setEmail] = useState("");
  vlog(FILE, "LoginPage", "inicializando estado do campo senha");
  const [senha, setSenha] = useState("");
  vlog(FILE, "LoginPage", "inicializando estado de loading");
  const [loading, setLoading] = useState(false);
  vlog(FILE, "LoginPage", "inicializando estado de erro geral");
  const [error, setError] = useState<string | null>(null);
  vlog(FILE, "LoginPage", "inicializando estado de erros por campo");
  const [fieldErrors, setFieldErrors] = useState<{ email?: string; senha?: string }>({});
  vlog(FILE, "LoginPage", "inicializando estado do token do captcha");
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  vlog(FILE, "LoginPage", "criando ref do widget Turnstile");
  const turnstileRef = useRef<TurnstileHandle>(null);

  vlog(FILE, "LoginPage", "registrando efeito de redirecionamento se ja autenticado");
  useEffect(() => {
    vlog(FILE, "LoginPage.useEffect", "verificando se ha usuario salvo na sessao");
    if (getUser()) {
      vlog(FILE, "LoginPage.useEffect", "usuario ja autenticado; redirecionando (admin=%s)", isAdmin());
      router.replace(isAdmin() ? "/dashboard" : "/pagamentos");
    }
  }, [router]);

  vlog(FILE, "LoginPage", "definindo funcao validate");
  const validate = (): boolean => {
    vlog(FILE, "LoginPage.validate", "criando objeto de erros por campo");
    const errs: { email?: string; senha?: string } = {};
    vlog(FILE, "LoginPage.validate", "definindo regex de formato de email");
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    vlog(FILE, "LoginPage.validate", "verificando se email esta vazio ou com formato invalido");
    if (!email) {
      vlog(FILE, "LoginPage.validate", "email vazio; registrando erro de obrigatoriedade");
      errs.email = "Email e obrigatorio";
    } else if (!emailRegex.test(email)) {
      vlog(FILE, "LoginPage.validate", "email com formato invalido; registrando erro");
      errs.email = "Email invalido";
    }
    vlog(FILE, "LoginPage.validate", "verificando se senha esta vazia");
    if (!senha) {
      vlog(FILE, "LoginPage.validate", "senha vazia; registrando erro de obrigatoriedade");
      errs.senha = "Senha e obrigatoria";
    }
    vlog(FILE, "LoginPage.validate", "atualizando erros por campo (qtd=%d)", Object.keys(errs).length);
    setFieldErrors(errs);
    return Object.keys(errs).length === 0;
  };

  vlog(FILE, "LoginPage", "definindo handler handleSubmit");
  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    vlog(FILE, "LoginPage.handleSubmit", "prevenindo submit padrao do formulario");
    e.preventDefault();
    vlog(FILE, "LoginPage.handleSubmit", "limpando erro geral");
    setError(null);
    vlog(FILE, "LoginPage.handleSubmit", "validando campos do formulario");
    if (!validate()) return;

    vlog(FILE, "LoginPage.handleSubmit", "ativando loading");
    setLoading(true);
    vlog(FILE, "LoginPage.handleSubmit", "iniciando chamada de login");
    try {
      vlog(FILE, "LoginPage.handleSubmit", "chamando apiLogin (captcha presente=%s)", captchaToken !== null);
      const res = await apiLogin(email, senha, captchaToken ?? undefined);

      // Tokens sao definidos pelo backend via Set-Cookie HttpOnly (nao acessiveis via JS).
      // Apenas o usuario e persistido no client para controle de UI.
      vlog(FILE, "LoginPage.handleSubmit", "salvando usuario autenticado na sessao local");
      saveUser(res.user);

      // Verificar se precisa trocar a senha
      vlog(FILE, "LoginPage.handleSubmit", "verificando se precisa trocar senha (trocar_senha=%s)", !!res.trocar_senha);
      if (res.trocar_senha) {
        vlog(FILE, "LoginPage.handleSubmit", "redirecionando para /trocar-senha");
        router.replace("/trocar-senha");
      } else {
        vlog(FILE, "LoginPage.handleSubmit", "redirecionando conforme papel (admin=%s)", isAdmin());
        router.replace(isAdmin() ? "/dashboard" : "/pagamentos");
      }
    } catch (err) {
      vlog(FILE, "LoginPage.handleSubmit", "falha no login; extraindo mensagem do erro");
      const rawMessage = err instanceof Error ? err.message : "";
      vlog(FILE, "LoginPage.handleSubmit", "verificando se a falha foi de captcha");
      const isCaptchaFailure = /captcha|turnstile/i.test(rawMessage);
      // Falha de captcha: mensagem genérica, sem expor detalhes técnicos do
      // motivo. Demais falhas (ex: credenciais inválidas) mantêm a mensagem
      // retornada pela API.
      vlog(FILE, "LoginPage.handleSubmit", "exibindo mensagem de erro (falha de captcha=%s)", isCaptchaFailure);
      setError(
        isCaptchaFailure
          ? "Nao foi possivel validar o captcha. Tente novamente."
          : rawMessage || "Erro ao fazer login"
      );
      // Token do Turnstile e de uso unico: apos qualquer falha, reseta o
      // widget para forcar um novo desafio antes de reenviar.
      vlog(FILE, "LoginPage.handleSubmit", "descartando token do captcha usado");
      setCaptchaToken(null);
      vlog(FILE, "LoginPage.handleSubmit", "resetando widget Turnstile");
      turnstileRef.current?.reset();
    } finally {
      vlog(FILE, "LoginPage.handleSubmit", "desativando loading");
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
          <h2 className="mb-1 text-xl font-semibold text-slate-900">
            Entrar na sua conta
          </h2>
          <p className="mb-6 text-sm text-slate-500">
            Informe suas credenciais para acessar o sistema.
          </p>

          {error && (
            <div className="mb-4">
              <Alert variant="error" onClose={() => setError(null)}>
                {error}
              </Alert>
            </div>
          )}

          <form onSubmit={handleSubmit} noValidate className="space-y-4">
            <Input
              label="Email"
              type="email"
              placeholder="seu@email.com"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              error={fieldErrors.email}
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
                    d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                  />
                </svg>
              }
            />

            <Input
              label="Senha"
              type="password"
              placeholder="********"
              autoComplete="current-password"
              value={senha}
              onChange={(e) => setSenha(e.target.value)}
              error={fieldErrors.senha}
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
              {loading ? "Entrando..." : "Entrar"}
            </Button>
          </form>

          <p className="mt-6 text-center text-xs text-slate-400">
            Acesso restrito. Em caso de duvidas, contate o administrador.
          </p>
        </Card>
      </div>
    </div>
  );
}
