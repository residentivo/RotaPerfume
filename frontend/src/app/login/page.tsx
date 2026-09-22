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

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [senha, setSenha] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{ email?: string; senha?: string }>({});
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  const turnstileRef = useRef<TurnstileHandle>(null);

  useEffect(() => {
    if (getUser()) {
      router.replace(isAdmin() ? "/dashboard" : "/pagamentos");
    }
  }, [router]);

  const validate = (): boolean => {
    const errs: { email?: string; senha?: string } = {};
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!email) {
      errs.email = "Email e obrigatorio";
    } else if (!emailRegex.test(email)) {
      errs.email = "Email invalido";
    }
    if (!senha) {
      errs.senha = "Senha e obrigatoria";
    }
    setFieldErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError(null);
    if (!validate()) return;

    setLoading(true);
    try {
      const res = await apiLogin(email, senha, captchaToken ?? undefined);

      // Tokens sao definidos pelo backend via Set-Cookie HttpOnly (nao acessiveis via JS).
      // Apenas o usuario e persistido no client para controle de UI.
      saveUser(res.user);

      // Verificar se precisa trocar a senha
      if (res.trocar_senha) {
        router.replace("/trocar-senha");
      } else {
        router.replace(isAdmin() ? "/dashboard" : "/pagamentos");
      }
    } catch (err) {
      const rawMessage = err instanceof Error ? err.message : "";
      const isCaptchaFailure = /captcha|turnstile/i.test(rawMessage);
      // Falha de captcha: mensagem genérica, sem expor detalhes técnicos do
      // motivo. Demais falhas (ex: credenciais inválidas) mantêm a mensagem
      // retornada pela API.
      setError(
        isCaptchaFailure
          ? "Nao foi possivel validar o captcha. Tente novamente."
          : rawMessage || "Erro ao fazer login"
      );
      // Token do Turnstile e de uso unico: apos qualquer falha, reseta o
      // widget para forcar um novo desafio antes de reenviar.
      setCaptchaToken(null);
      turnstileRef.current?.reset();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-gradient-to-br from-slate-900 via-blue-900 to-slate-900 px-4">
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

        <Card className="border-slate-200/20 bg-white/95 shadow-2xl backdrop-blur">
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
