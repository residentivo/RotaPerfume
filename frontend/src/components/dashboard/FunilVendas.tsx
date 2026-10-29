"use client";

import { useEffect, useState } from "react";
import { Card, CardHeader } from "@/components/ui/Card";
import { Alert } from "@/components/ui/Alert";
import { apiDashboardFunil } from "@/lib/api";
import { useVendedorDesligado } from "@/lib/session";
import type {
  FunilEtapa,
  FunilJanela,
  FunilVendas as FunilVendasDados,
} from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "components/dashboard/FunilVendas.tsx";

// ─── Helpers ────────────────────────────────────────────────────────────────

// Mesmos formatadores do dashboard/page.tsx: valores ausentes/invalidos viram
// 0 para nunca exibir "NaN".
const currencyFormatter = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

function numeroSeguro(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

function fmtCurrency(value: number | null | undefined): string {
  return currencyFormatter.format(numeroSeguro(value));
}

function fmtNumber(value: number): string {
  return new Intl.NumberFormat("pt-BR").format(numeroSeguro(value));
}

function fmtPercent(value: number): string {
  return `${Math.min(100, Math.max(0, numeroSeguro(value))).toFixed(1)}%`;
}

// Janelas aceitas pelo backend (enum fixo). O seletor so envia estes valores.
export const JANELAS_FUNIL: readonly FunilJanela[] = ["30d", "90d", "365d", "all"];

export const JANELA_LABELS: Record<FunilJanela, string> = {
  "30d": "30 dias",
  "90d": "90 dias",
  "365d": "12 meses",
  all: "Tudo",
};

// Etapas conhecidas do funil (ordem topo → fundo). `etapa` e texto livre do
// banco: so e usada como rotulo/chave/cor se estiver nesta whitelist; o
// resto vira "Outras".
export const ETAPAS_FUNIL = [
  "Prospecção",
  "Qualificação",
  "Proposta enviada",
  "Negociação",
  "Fechado ganho",
] as const;

export type EtapaFunilConhecida = (typeof ETAPAS_FUNIL)[number] | "Outras";

const CORES_ETAPA: Record<EtapaFunilConhecida, string> = {
  "Prospecção": "bg-primary-200 text-primary-900",
  "Qualificação": "bg-primary-300 text-primary-900",
  "Proposta enviada": "bg-primary-400 text-white",
  "Negociação": "bg-primary-500 text-white",
  "Fechado ganho": "bg-primary-600 text-white",
  Outras: "bg-slate-300 text-slate-800",
};

export function etapaSegura(etapa: unknown): EtapaFunilConhecida {
  vlog(FILE, "etapaSegura", "verificando etapa na whitelist");
  return typeof etapa === "string" &&
    (ETAPAS_FUNIL as readonly string[]).includes(etapa)
    ? (etapa as EtapaFunilConhecida)
    : "Outras";
}

// Largura minima visivel para a barra nao sumir quando a conversao e ~0.
export const LARGURA_MINIMA_PCT = 8;

export function larguraBarra(conversaoTopoPct: number): number {
  const pct = Math.min(100, Math.max(0, numeroSeguro(conversaoTopoPct)));
  return Math.max(LARGURA_MINIMA_PCT, pct);
}

export function funilZerado(janela: FunilJanela): FunilVendasDados {
  return {
    janela,
    total_oportunidades: 0,
    etapas: ETAPAS_FUNIL.map((etapa) => ({
      etapa,
      quantidade: 0,
      valor_estimado: 0,
      conversao_anterior_pct: 0,
      conversao_topo_pct: 0,
    })),
    perdidas: { quantidade: 0, valor_estimado: 0 },
    taxa_ganho_pct: 0,
    vendedor_desligado: false,
  };
}

// Normaliza o payload: campos ausentes/invalidos viram 0 e um payload sem
// `etapas` vira o funil zerado (nunca dados inventados).
export function normalizarFunil(
  dados: unknown,
  janela: FunilJanela
): FunilVendasDados {
  vlog(FILE, "normalizarFunil", "verificando formato do payload");
  if (!dados || typeof dados !== "object") return funilZerado(janela);
  const f = dados as Partial<FunilVendasDados>;
  if (!Array.isArray(f.etapas)) return funilZerado(janela);
  vlog(FILE, "normalizarFunil", "normalizando etapas (qtd=%d)", f.etapas.length);
  return {
    janela,
    total_oportunidades: numeroSeguro(f.total_oportunidades),
    etapas: f.etapas.map((e: Partial<FunilEtapa>) => ({
      etapa: typeof e?.etapa === "string" ? e.etapa : "",
      quantidade: numeroSeguro(e?.quantidade),
      valor_estimado: numeroSeguro(e?.valor_estimado),
      conversao_anterior_pct: numeroSeguro(e?.conversao_anterior_pct),
      conversao_topo_pct: numeroSeguro(e?.conversao_topo_pct),
    })),
    perdidas: {
      quantidade: numeroSeguro(f.perdidas?.quantidade),
      valor_estimado: numeroSeguro(f.perdidas?.valor_estimado),
    },
    taxa_ganho_pct: numeroSeguro(f.taxa_ganho_pct),
    vendedor_desligado: f.vendedor_desligado === true,
  };
}

// Mensagem generica: o corpo do erro (400/401/500) nunca e exibido.
export const ERRO_FUNIL = "Nao foi possivel carregar o funil de vendas. Tente novamente.";

// ─── Componente ──────────────────────────────────────────────────────────────

export function FunilVendas({
  isAdmin,
  forcarZeros = false,
  recarga = 0,
}: {
  isAdmin: boolean;
  // FE-02: sem vendedor vinculado ou vendedor desligado (decidido pela
  // pagina), o funil e exibido zerado ignorando o payload.
  forcarZeros?: boolean;
  // Incrementado pelo botao "Atualizar" da pagina para refazer a busca.
  recarga?: number;
}) {
  vlog(FILE, "FunilVendas", "verificando se o vendedor esta desligado na sessao");
  const desligadoNaSessao = useVendedorDesligado();
  vlog(FILE, "FunilVendas", "inicializando estado da janela");
  const [janela, setJanela] = useState<FunilJanela>("all");
  vlog(FILE, "FunilVendas", "inicializando estado do funil");
  const [funil, setFunil] = useState<FunilVendasDados | null>(null);

  // Mesmo padrao do DashboardContent: loading/erro derivados da chave.
  vlog(FILE, "FunilVendas", "montando chave da busca (janela=%s, recarga=%d)", janela, recarga);
  const chave = `${janela}|${recarga}`;
  const [resultado, setResultado] = useState<{ chave: string; erro: boolean } | null>(
    null
  );
  vlog(FILE, "FunilVendas", "derivando loading pela chave do resultado");
  const loading = resultado?.chave !== chave;
  vlog(FILE, "FunilVendas", "derivando erro do resultado (loading=%s)", loading);
  const erro = !loading && resultado?.erro === true;

  vlog(FILE, "FunilVendas", "registrando efeito de carga do funil");
  useEffect(() => {
    vlog(FILE, "FunilVendas.useEffect", "inicializando flag de cancelamento");
    let cancelado = false;
    vlog(FILE, "FunilVendas.useEffect", "buscando funil (janela=%s)", janela);
    Promise.resolve()
      .then(() => apiDashboardFunil(janela))
      .then((dados) => {
        vlog(FILE, "FunilVendas.useEffect.then", "verificando se a busca foi cancelada (cancelado=%s)", cancelado);
        if (cancelado) return;
        vlog(FILE, "FunilVendas.useEffect.then", "aplicando funil normalizado");
        setFunil(normalizarFunil(dados, janela));
        setResultado({ chave, erro: false });
      })
      .catch(() => {
        vlog(FILE, "FunilVendas.useEffect.catch", "falha na busca; verificando se foi cancelada (cancelado=%s)", cancelado);
        if (cancelado) return;
        vlog(FILE, "FunilVendas.useEffect.catch", "registrando resultado de erro (mensagem generica)");
        setResultado({ chave, erro: true });
      });
    return () => {
      vlog(FILE, "FunilVendas.useEffect.cleanup", "cancelando busca anterior");
      cancelado = true;
    };
  }, [chave, janela]);

  vlog(FILE, "FunilVendas", "calculando se vendedor esta desligado (funil ou sessao)");
  const vendedorDesligado =
    !isAdmin && (funil?.vendedor_desligado === true || desligadoNaSessao);
  vlog(FILE, "FunilVendas", "calculando se zera o funil (forcarZeros=%s, desligado=%s)", forcarZeros, vendedorDesligado);
  const zerar = !isAdmin && (forcarZeros || vendedorDesligado);
  vlog(FILE, "FunilVendas", "definindo funil exibido (zerar=%s)", zerar);
  const exibido: FunilVendasDados = !zerar && funil ? funil : funilZerado(janela);
  vlog(FILE, "FunilVendas", "verificando se o funil esta vazio (total=%d)", exibido.total_oportunidades);
  const vazio = exibido.total_oportunidades <= 0;
  // Com erro e nenhuma carga bem-sucedida, o estado vazio seria falso.
  const ocultarVazio = erro && funil === null && !zerar;

  return (
    <Card className="mb-6">
      <CardHeader
        title={isAdmin ? "Funil de vendas" : "Meu funil de vendas"}
        subtitle="Oportunidades por etapa (cumulativo)"
        action={
          <div
            role="group"
            aria-label="Janela do funil"
            className="flex items-center gap-1 rounded-lg border border-slate-200 bg-white p-1"
          >
            {JANELAS_FUNIL.map((j) => (
              <button
                key={j}
                type="button"
                onClick={() => setJanela(j)}
                aria-pressed={janela === j}
                className={[
                  "rounded-md px-3 py-1.5 text-sm font-medium transition-all",
                  janela === j
                    ? "bg-primary-600 text-white shadow-xs"
                    : "text-slate-600 hover:bg-slate-100",
                ]
                  .filter(Boolean)
                  .join(" ")}
              >
                {JANELA_LABELS[j]}
              </button>
            ))}
          </div>
        }
      />

      {erro && (
        <Alert variant="error" className="mb-4">
          {ERRO_FUNIL}
        </Alert>
      )}

      {vendedorDesligado && !forcarZeros && (
        <Alert variant="warning" className="mb-4">
          Vendedor desligado. Nao ha oportunidades no funil da sua carteira;
          procure um administrador.
        </Alert>
      )}

      {loading && !zerar && funil === null ? (
        <div className="flex h-48 items-center justify-center" aria-label="Carregando funil">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-200 border-t-primary-600" />
        </div>
      ) : ocultarVazio ? null : vazio ? (
        <div className="py-8 text-center text-sm text-slate-400">
          Nenhuma oportunidade na janela selecionada
        </div>
      ) : (
        <>
          <ol className="space-y-2" aria-label="Etapas do funil de vendas">
            {exibido.etapas.map((e, i) => {
              const etapa = etapaSegura(e.etapa);
              const largura = larguraBarra(e.conversao_topo_pct);
              const descricao = `${etapa}: ${fmtNumber(e.quantidade)} oportunidades, ${fmtCurrency(
                e.valor_estimado
              )} estimados, ${fmtPercent(e.conversao_topo_pct)} do topo, ${fmtPercent(
                e.conversao_anterior_pct
              )} da etapa anterior`;
              return (
                <li key={`${i}-${etapa}`} className="flex flex-col items-center">
                  <div
                    role="img"
                    aria-label={descricao}
                    title={descricao}
                    className={[
                      "flex min-h-11 items-center justify-between gap-2 rounded-md px-3 py-1.5 text-xs transition-all sm:text-sm",
                      CORES_ETAPA[etapa],
                    ].join(" ")}
                    style={{ width: `${largura}%`, minWidth: "12rem" }}
                  >
                    <span className="truncate font-semibold">{etapa}</span>
                    <span className="shrink-0 font-bold">{fmtNumber(e.quantidade)}</span>
                  </div>
                  <div className="mt-0.5 flex flex-wrap justify-center gap-x-3 text-xs text-slate-500">
                    <span>{fmtCurrency(e.valor_estimado)}</span>
                    {i > 0 && (
                      <span>{fmtPercent(e.conversao_anterior_pct)} da etapa anterior</span>
                    )}
                  </div>
                </li>
              );
            })}
          </ol>

          <dl className="mt-6 grid gap-4 border-t border-slate-100 pt-4 sm:grid-cols-3">
            <div className="rounded-lg border border-slate-100 bg-slate-50 p-3">
              <dt className="text-xs font-medium uppercase tracking-wider text-slate-500">
                Oportunidades
              </dt>
              <dd className="mt-1 text-lg font-semibold text-slate-900">
                {fmtNumber(exibido.total_oportunidades)}
              </dd>
            </div>
            <div className="rounded-lg border border-slate-100 bg-slate-50 p-3">
              <dt className="text-xs font-medium uppercase tracking-wider text-slate-500">
                Taxa de ganho
              </dt>
              <dd className="mt-1 text-lg font-semibold text-emerald-700">
                {fmtPercent(exibido.taxa_ganho_pct)}
              </dd>
            </div>
            <div className="rounded-lg border border-slate-100 bg-slate-50 p-3">
              <dt className="text-xs font-medium uppercase tracking-wider text-slate-500">
                Perdidas
              </dt>
              <dd className="mt-1 text-lg font-semibold text-red-700">
                {fmtNumber(exibido.perdidas.quantidade)}
              </dd>
              <dd className="text-xs text-slate-500">
                {fmtCurrency(exibido.perdidas.valor_estimado)}
              </dd>
            </div>
          </dl>
        </>
      )}
    </Card>
  );
}
