"use client";

import { useCallback, useEffect, useState } from "react";
import { Navbar } from "@/components/layout/Navbar";
import { Card, CardHeader } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { useSessionUser, useVendedorDesligado } from "@/lib/session";
import {
  apiDashboardMetrics,
  apiDashboardVendas,
  apiDashboardVendedores,
  apiDashboardClientes,
} from "@/lib/api";
import {
  DashboardMetrics,
  VendasSeries,
  VendedorRanking,
  ClienteDashboardMetrics,
} from "@/lib/types";

// ─── Helpers ────────────────────────────────────────────────────────────────

// Valores monetarios chegam do backend como float com centavos (ex.: 1234.56);
// o Intl formata com 2 casas decimais, sem truncar. Valores ausentes/invalidos
// viram 0 para nunca exibir "NaN".
const currencyFormatter = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

function fmtCurrency(value: number | null | undefined): string {
  const n = Number(value);
  return currencyFormatter.format(Number.isFinite(n) ? n : 0);
}

function fmtNumber(value: number): string {
  return new Intl.NumberFormat("pt-BR").format(value);
}

function fmtPercent(value: number): string {
  return `${Math.min(100, Math.max(0, value)).toFixed(1)}%`;
}

function fmtDate(dateStr: string): string {
  const [, m, d] = dateStr.split("-").map(Number);
  return `${String(d).padStart(2, "0")}/${String(m).padStart(2, "0")}`;
}

type PeriodFilter = "today" | "week" | "month";

const PERIOD_LABELS: Record<PeriodFilter, string> = {
  today: "Hoje",
  week: "Semana",
  month: "Mes",
};

// ─── KPI Card ────────────────────────────────────────────────────────────────

function KpiCard({
  label,
  value,
  sub,
  accent = false,
  icon,
}: {
  label: string;
  value: string;
  sub?: string;
  accent?: boolean;
  icon: React.ReactNode;
}) {
  return (
    <div
      className={[
        "relative overflow-hidden rounded-xl border bg-white p-5 shadow-sm transition-shadow hover:shadow-md",
        accent
          ? "border-emerald-200 bg-gradient-to-br from-emerald-50 to-white"
          : "border-slate-200",
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-medium text-slate-500">{label}</p>
          <p
            className={[
              "mt-1 text-2xl font-bold tracking-tight",
              accent ? "text-emerald-700" : "text-slate-900",
            ]
              .filter(Boolean)
              .join(" ")}
          >
            {value}
          </p>
          {sub && (
            <p className="mt-0.5 text-xs text-slate-400">{sub}</p>
          )}
        </div>
        <div
          className={[
            "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg",
            accent
              ? "bg-emerald-100 text-emerald-600"
              : "bg-slate-100 text-slate-500",
          ]
            .filter(Boolean)
            .join(" ")}
        >
          {icon}
        </div>
      </div>
      {accent && (
        <div className="pointer-events-none absolute -bottom-2 -right-2 h-20 w-20 rounded-full bg-emerald-100 opacity-50" />
      )}
    </div>
  );
}

// ─── Bar Chart (CSS only) ────────────────────────────────────────────────────

function BarChart({ series }: { series: VendasSeries }) {
  if (!series.pontos.length) {
    return (
      <div className="flex h-48 w-full items-center justify-center text-sm text-slate-400">
        Nenhum dado de vendas no periodo
      </div>
    );
  }

  const maxVendas = Math.max(...series.pontos.map((p) => p.total_vendas), 1);

  // Amostra maxima de 30 barras visiveis
  const sampled =
    series.pontos.length > 30
      ? series.pontos.filter((_, i) => i % Math.ceil(series.pontos.length / 30) === 0)
      : series.pontos;

  return (
    <div className="space-y-1">
      {/* Y-axis labels */}
      <div className="flex h-48 items-stretch gap-1">
        <div className="flex w-14 flex-col justify-between text-right text-xs text-slate-400">
          <span>{fmtCurrency(maxVendas)}</span>
          <span>{fmtCurrency(maxVendas / 2)}</span>
          <span>R$ 0</span>
        </div>

        {/* Bars */}
        <div className="relative flex flex-1 items-stretch gap-[2px]">
          {sampled.map((pt, i) => {
            const pct = (pt.total_vendas / maxVendas) * 100;
            return (
              <div
                key={i}
                className="group relative flex flex-1 flex-col justify-end cursor-default"
                title={`${fmtDate(pt.dia)}: ${fmtCurrency(pt.total_vendas)} (${pt.total_pedidos} pedidos)`}
              >
                {/* Barra */}
                <div
                  className="w-full rounded-t bg-primary-500 transition-all duration-200 hover:bg-primary-600"
                  style={{ height: `${Math.max(1, pct)}%` }}
                />
                {/* Tooltip */}
                <div className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1 -translate-x-1/2 whitespace-nowrap rounded bg-slate-800 px-2 py-1 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100">
                  <span className="font-medium">{fmtDate(pt.dia)}:</span>{" "}
                  {fmtCurrency(pt.total_vendas)}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* X-axis labels */}
      <div className="flex justify-between pl-14 text-xs text-slate-400">
        <span>{sampled.length > 0 ? fmtDate(sampled[0].dia) : ""}</span>
        <span>
          {sampled.length > 0 ? fmtDate(sampled[sampled.length - 1].dia) : ""}
        </span>
      </div>
    </div>
  );
}

// ─── Sellers Table ───────────────────────────────────────────────────────────

function RankingTable({ vendedores }: { vendedores: VendedorRanking[] }) {
  if (!vendedores.length) {
    return (
      <div className="py-8 text-center text-sm text-slate-400">
        Nenhum vendedor encontrado
      </div>
    );
  }

  return (
    <div className="-mx-6 overflow-x-auto">
      <table className="min-w-full text-sm">
        <thead>
          <tr className="border-b border-slate-100">
            <th className="px-6 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
              #
            </th>
            <th className="px-6 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
              Vendedor
            </th>
            <th className="px-6 py-3 text-right text-xs font-semibold uppercase tracking-wider text-slate-500">
              Vendas
            </th>
            <th className="px-6 py-3 text-right text-xs font-semibold uppercase tracking-wider text-slate-500">
              Pedidos
            </th>
            <th className="px-6 py-3 text-right text-xs font-semibold uppercase tracking-wider text-slate-500">
              Ticket Medio
            </th>
            <th className="px-6 py-3 text-right text-xs font-semibold uppercase tracking-wider text-slate-500">
              Meta
            </th>
            <th className="min-w-32 px-6 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
              Atingimento
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-50">
          {vendedores.map((v, i) => {
            const pct = v.atingimento_meta ?? 0;
            const barColor =
              pct >= 100
                ? "bg-emerald-500"
                : pct >= 70
                ? "bg-amber-400"
                : "bg-red-400";

            return (
              <tr
                key={v.vendedor_id}
                className={[
                  "transition-colors",
                  i === 0 ? "bg-amber-50/50" : "hover:bg-slate-50",
                ]
                  .filter(Boolean)
                  .join(" ")}
              >
                <td className="px-6 py-3">
                  <span
                    className={[
                      "inline-flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold",
                      i === 0
                        ? "bg-amber-400 text-amber-900"
                        : i === 1
                        ? "bg-slate-300 text-slate-700"
                        : i === 2
                        ? "bg-orange-300 text-orange-900"
                        : "bg-slate-100 text-slate-500",
                    ]
                      .filter(Boolean)
                      .join(" ")}
                  >
                    {i + 1}
                  </span>
                </td>
                <td className="px-6 py-3 font-medium text-slate-900">
                  {v.vendedor_nome}
                </td>
                <td className="px-6 py-3 text-right font-semibold text-emerald-700">
                  {fmtCurrency(v.total_vendas)}
                </td>
                <td className="px-6 py-3 text-right text-slate-600">
                  {fmtNumber(v.total_pedidos)}
                </td>
                <td className="px-6 py-3 text-right text-slate-600">
                  {fmtCurrency(v.ticket_medio)}
                </td>
                <td className="px-6 py-3 text-right text-slate-600">
                  {v.meta != null ? fmtCurrency(v.meta) : "-"}
                </td>
                <td className="px-6 py-3">
                  <div className="flex items-center gap-2">
                    <div className="min-w-0 flex-1">
                      <div className="h-2 overflow-hidden rounded-full bg-slate-100">
                        <div
                          className={["h-full rounded-full transition-all", barColor].join(" ")}
                          style={{ width: `${Math.min(100, pct)}%` }}
                        />
                      </div>
                    </div>
                    <span className="min-w-10 text-right text-xs font-medium text-slate-600">
                      {pct > 0 ? fmtPercent(pct) : "-"}
                    </span>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ─── Simple Horizontal Bar List (segmento / UF) ────────────────────────────────

function HorizontalBarList({
  items,
  colorClass = "bg-primary-500",
}: {
  items: { label: string; total: number }[];
  colorClass?: string;
}) {
  if (!items.length) {
    return (
      <div className="py-6 text-center text-sm text-slate-400">
        Sem dados disponiveis
      </div>
    );
  }

  const max = Math.max(...items.map((i) => i.total), 1);

  return (
    <div className="space-y-3">
      {items.map((item) => {
        const pct = (item.total / max) * 100;
        return (
          <div key={item.label}>
            <div className="mb-1 flex items-center justify-between text-xs">
              <span className="font-medium text-slate-700">{item.label}</span>
              <span className="text-slate-500">{fmtNumber(item.total)}</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-slate-100">
              <div
                className={["h-full rounded-full transition-all", colorClass].join(" ")}
                style={{ width: `${Math.max(2, pct)}%` }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ─── Goal Progress Card ───────────────────────────────────────────────────────

function GoalProgress({
  label,
  current,
  target,
}: {
  label: string;
  current: number;
  target: number;
}) {
  const pct = target > 0 ? Math.min(100, (current / target) * 100) : 0;
  const color = pct >= 100 ? "emerald" : pct >= 70 ? "amber" : "red";

  const colors = {
    emerald: {
      bar: "bg-emerald-500",
      text: "text-emerald-700",
      bg: "bg-emerald-50 border-emerald-200",
    },
    amber: {
      bar: "bg-amber-400",
      text: "text-amber-700",
      bg: "bg-amber-50 border-amber-200",
    },
    red: {
      bar: "bg-red-400",
      text: "text-red-700",
      bg: "bg-red-50 border-red-200",
    },
  };

  const c = colors[color];

  return (
    <div className={`rounded-lg border p-4 ${c.bg}`}>
      <div className="mb-2 flex items-center justify-between">
        <p className="text-sm font-medium text-slate-700">{label}</p>
        <p className={`text-sm font-bold ${c.text}`}>{fmtPercent(pct)}</p>
      </div>
      <div className="h-3 overflow-hidden rounded-full bg-white/80">
        <div
          className={["h-full rounded-full transition-all duration-500", c.bar].join(" ")}
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className="mt-1.5 text-xs text-slate-500">
        {fmtCurrency(current)} de {fmtCurrency(target)}
      </p>
    </div>
  );
}

// ─── SVG Icons ───────────────────────────────────────────────────────────────

function IconCurrency() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 6v12m-3-2.818l.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

function IconCart() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M15.75 10.5V6a3.75 3.75 0 10-7.5 0v4.5m11.356-1.993l1.263 12c.07.665-.45 1.243-1.119 1.243H4.25a1.125 1.125 0 01-1.12-1.243l1.264-12A1.125 1.125 0 015.513 7.5h12.974c.576 0 1.059.435 1.119 1.007zM8.625 10.5a.375.375 0 11-.75 0 .375.375 0 01.75 0zm7.5 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z"
      />
    </svg>
  );
}

function IconTicket() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M16.5 6v.75m0 3v.75m0 3v.75m0 3V18m-9-5.25h5.25M7.5 15h3M3.375 5.25c-.621 0-1.125.504-1.125 1.125v3.026a2.999 2.999 0 010 5.198v3.026c0 .621.504 1.125 1.125 1.125h17.25c.621 0 1.125-.504 1.125-1.125v-3.026a2.999 2.999 0 010-5.198V6.375c0-.621-.504-1.125-1.125-1.125H3.375z"
      />
    </svg>
  );
}

function IconRanking() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.563.563 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.563.563 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z"
      />
    </svg>
  );
}

function IconUsers() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={1.5}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z"
      />
    </svg>
  );
}

// ─── Content ─────────────────────────────────────────────────────────────────

// ─── Meu Desempenho (usuario normal) ─────────────────────────────────────────

function MeuDesempenho({ vendedor }: { vendedor: VendedorRanking | undefined }) {
  if (!vendedor) {
    return (
      <div className="py-8 text-center text-sm text-slate-400">
        Nenhum dado de desempenho encontrado
      </div>
    );
  }

  const pct = vendedor.atingimento_meta ?? 0;
  const barColor =
    pct >= 100 ? "bg-emerald-500" : pct >= 70 ? "bg-amber-400" : "bg-red-400";

  const stats: { label: string; value: string; highlight?: boolean }[] = [
    { label: "Vendas", value: fmtCurrency(vendedor.total_vendas ?? 0), highlight: true },
    { label: "Pedidos", value: fmtNumber(vendedor.total_pedidos ?? 0) },
    { label: "Ticket Medio", value: fmtCurrency(vendedor.ticket_medio ?? 0) },
    { label: "Meta", value: vendedor.meta != null ? fmtCurrency(vendedor.meta) : "-" },
  ];

  return (
    <div className="space-y-4">
      <p className="text-sm font-medium text-slate-900">
        {vendedor.vendedor_id} - {vendedor.vendedor_nome}
      </p>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((s) => (
          <div key={s.label} className="rounded-lg border border-slate-100 bg-slate-50 p-3">
            <p className="text-xs font-medium uppercase tracking-wider text-slate-500">
              {s.label}
            </p>
            <p
              className={[
                "mt-1 text-lg font-semibold",
                s.highlight ? "text-emerald-700" : "text-slate-900",
              ].join(" ")}
            >
              {s.value}
            </p>
          </div>
        ))}
      </div>
      <div>
        <div className="mb-1 flex items-center justify-between text-xs">
          <span className="font-medium text-slate-600">Atingimento da meta</span>
          <span className="font-medium text-slate-600">
            {pct > 0 ? fmtPercent(pct) : "-"}
          </span>
        </div>
        <div className="h-2 overflow-hidden rounded-full bg-slate-100">
          <div
            className={["h-full rounded-full transition-all", barColor].join(" ")}
            style={{ width: `${Math.min(100, Math.max(0, pct))}%` }}
          />
        </div>
      </div>
    </div>
  );
}

function DashboardContent() {
  // ProtectedRoute so renderiza os filhos apos validar a sessao em
  // /api/auth/me; a sessao em memoria e a fonte da verdade (revalidada ao
  // voltar o foco). O escopo real (admin x vendedor) e aplicado pelo
  // backend; aqui so muda a apresentacao.
  const currentUser = useSessionUser();
  const desligadoNaSessao = useVendedorDesligado();
  const isAdmin = currentUser?.role === "admin";
  const semVendedor = !isAdmin && !currentUser?.id_vendedor;

  const [periodo, setPeriodo] = useState<PeriodFilter>("month");
  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null);
  const [vendasSeries, setVendasSeries] = useState<VendasSeries | null>(null);
  const [vendedores, setVendedores] = useState<VendedorRanking[]>([]);
  const [clienteMetrics, setClienteMetrics] = useState<ClienteDashboardMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [chartsDias, setChartsDias] = useState(30);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [m, v, vd, cm] = await Promise.all([
        apiDashboardMetrics(periodo),
        apiDashboardVendas(chartsDias),
        apiDashboardVendedores(1, 10),
        apiDashboardClientes(periodo),
      ]);
      setMetrics(m);
      setVendasSeries(v);
      setVendedores((Array.isArray(vd?.data) ? vd.data : []).slice(0, 10));
      setClienteMetrics(cm);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao carregar dados");
    } finally {
      setLoading(false);
    }
  }, [periodo, chartsDias]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Dados reais vindos da API. Quando ainda nao carregados, usa valores
  // zerados — nunca dados inventados.
  const displayMetrics: DashboardMetrics = metrics ?? {
    total_vendas: 0,
    total_pedidos: 0,
    ticket_medio: 0,
    total_clientes: 0,
    meta_mes: undefined,
    atingimento_meta: 0,
    periodo,
  };

  const displayVendas: VendasSeries = {
    dias: vendasSeries?.dias ?? chartsDias,
    pontos: Array.isArray(vendasSeries?.pontos) ? vendasSeries.pontos : [],
  };

  // Usuario normal: o ranking vem com no maximo 1 linha (a dele).
  const meuDesempenho = isAdmin ? undefined : vendedores[0];
  // Vendedor desligado: flag vem da API (/api/dashboard/metrics ou
  // /api/auth/me via sessao em memoria), nunca do cache local — o vinculo
  // pode continuar existindo no usuario, mas o vendedor ter data_desligamento.
  const vendedorDesligado =
    !isAdmin && (metrics?.vendedor_desligado === true || desligadoNaSessao);
  const temMeta = displayMetrics.meta_mes != null && displayMetrics.meta_mes > 0;

  return (
    <div className="min-h-screen bg-slate-50">
      <Navbar />
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Header */}
        <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold text-slate-900">Dashboard</h1>
            <p className="mt-0.5 text-sm text-slate-500">
              {isAdmin
                ? "Visao geral das metricas de vendas"
                : "Suas metricas de vendas"}
            </p>
          </div>

          {/* Period Filter */}
          <div className="flex items-center gap-1 rounded-lg border border-slate-200 bg-white p-1">
            {(["today", "week", "month"] as PeriodFilter[]).map((p) => (
              <button
                key={p}
                onClick={() => setPeriodo(p)}
                className={[
                  "rounded-md px-4 py-1.5 text-sm font-medium transition-all",
                  periodo === p
                    ? "bg-primary-600 text-white shadow-sm"
                    : "text-slate-600 hover:bg-slate-100",
                ]
                  .filter(Boolean)
                  .join(" ")}
              >
                {PERIOD_LABELS[p]}
              </button>
            ))}
          </div>
        </div>

        {/* Error */}
        {error && (
          <Alert variant="error" className="mb-4">
            {error}
          </Alert>
        )}

        {/* UI-02 (decisao do usuario: opcao b): o aviso aparece acima e os
            KPIs/grafico continuam visiveis, zerados (a API devolve os dados
            zerados nesses casos). */}
        {semVendedor ? (
          <Alert variant="warning" className="mb-6">
            Usuario sem vendedor vinculado. As metricas de vendas sao exibidas
            apenas para usuarios vinculados a um vendedor; solicite o vinculo a
            um administrador.
          </Alert>
        ) : vendedorDesligado ? (
          <Alert variant="warning" className="mb-6">
            Vendedor desligado. O vendedor vinculado ao seu usuario possui data
            de desligamento, por isso nao ha metricas de vendas nem clientes
            na sua carteira; procure um administrador.
          </Alert>
        ) : null}

        {/* KPI Cards */}
        <div className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <KpiCard
            label={isAdmin ? "Total de Vendas" : "Minhas Vendas"}
            value={fmtCurrency(displayMetrics.total_vendas)}
            sub={`Periodo: ${PERIOD_LABELS[periodo]}`}
            accent
            icon={<IconCurrency />}
          />
          <KpiCard
            label={isAdmin ? "Pedidos" : "Meus Pedidos"}
            value={fmtNumber(displayMetrics.total_pedidos)}
            sub="Pedidos realizados"
            icon={<IconCart />}
          />
          <KpiCard
            label={isAdmin ? "Ticket Medio" : "Meu Ticket Medio"}
            value={fmtCurrency(displayMetrics.ticket_medio)}
            sub="Por pedido"
            icon={<IconTicket />}
          />
          {isAdmin ? (
            <KpiCard
              label="Ranking Vendedores"
              value={vendedores.length > 0 ? fmtCurrency(vendedores[0]?.total_vendas ?? 0) : "-"}
              sub={vendedores[0] ? `Lider: ${vendedores[0].vendedor_nome}` : "Carregando..."}
              icon={<IconRanking />}
            />
          ) : (
            <KpiCard
              label="Minha Meta"
              value={temMeta ? fmtCurrency(displayMetrics.meta_mes ?? 0) : "-"}
              sub={
                temMeta
                  ? `Atingido: ${fmtPercent(
                      (displayMetrics.total_vendas / (displayMetrics.meta_mes ?? 1)) * 100
                    )}`
                  : "Sem meta cadastrada"
              }
              icon={<IconRanking />}
            />
          )}
        </div>

        {/* Charts Row */}
        <div className="mb-6 grid gap-6 lg:grid-cols-3">
          {/* Grafico de Vendas */}
          <Card className="lg:col-span-2">
            <CardHeader
              title={
                isAdmin
                  ? `Vendas nos Últimos ${chartsDias} Dias`
                  : `Minhas Vendas nos Últimos ${chartsDias} Dias`
              }
              subtitle={`Total: ${fmtCurrency(displayVendas.pontos.reduce((s, p) => s + p.total_vendas, 0))}`}
              action={
                <select
                  className="rounded-md border border-slate-200 bg-white px-3 py-1.5 text-sm text-slate-600 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
                  value={chartsDias}
                  onChange={(e) => setChartsDias(Number(e.target.value))}
                >
                  <option value={7}>7 dias</option>
                  <option value={14}>14 dias</option>
                  <option value={30}>30 dias</option>
                  <option value={60}>60 dias</option>
                </select>
              }
            />
            {loading && vendasSeries === null ? (
              <div className="flex h-48 items-center justify-center">
                <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-200 border-t-primary-600" />
              </div>
            ) : (
              <BarChart series={displayVendas} />
            )}
          </Card>

          {/* Metas */}
          <Card>
            <CardHeader title={isAdmin ? "Acompanhamento de Metas" : "Minha Meta"} />
            <div className="space-y-4">
              {temMeta ? (
                <GoalProgress
                  label={isAdmin ? "Meta Mensal de Vendas" : "Minha Meta Mensal"}
                  current={displayMetrics.total_vendas}
                  target={displayMetrics.meta_mes ?? 0}
                />
              ) : (
                <div className="py-8 text-center text-sm text-slate-400">
                  {isAdmin
                    ? "Nenhuma meta de vendas cadastrada"
                    : "Nenhuma meta cadastrada para voce"}
                </div>
              )}
            </div>
          </Card>
        </div>

        {/* Ranking Table (admin) / Meu Desempenho (normal) */}
        <Card>
          <CardHeader
            title={isAdmin ? "Ranking de Vendedores" : "Meu Desempenho"}
            subtitle={
              isAdmin
                ? "Top 10 por volume de vendas no periodo"
                : "Seus indicadores no ranking de vendas"
            }
            action={
              <Button
                variant="ghost"
                size="sm"
                onClick={loadData}
                loading={loading}
              >
                Atualizar
              </Button>
            }
          />
          {loading && vendedores.length === 0 ? (
            <div className="flex items-center justify-center py-12">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-200 border-t-primary-600" />
            </div>
          ) : isAdmin ? (
            <RankingTable vendedores={vendedores} />
          ) : (
            <MeuDesempenho vendedor={meuDesempenho} />
          )}
        </Card>

        {/* Clientes */}
        {/* Admin: base global. Usuario normal: backend escopa pela carteira
            do vendedor vinculado. */}
        <div className="mt-6">
          {!isAdmin && (
            <div className="mb-3">
              <h2 className="text-lg font-semibold text-slate-900">
                Clientes da minha carteira
              </h2>
              <p className="text-sm text-slate-500">
                Somente clientes vinculados ao seu vendedor
              </p>
            </div>
          )}
          <div className="mb-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <KpiCard
              label={isAdmin ? "Total de Clientes" : "Meus Clientes"}
              value={fmtNumber(clienteMetrics?.total_clientes ?? 0)}
              sub={`Periodo: ${PERIOD_LABELS[periodo]}`}
              icon={<IconUsers />}
            />
            <KpiCard
              label={isAdmin ? "Clientes Ativos" : "Meus Clientes Ativos"}
              value={fmtNumber(clienteMetrics?.total_ativos ?? 0)}
              sub="Situacao ativa"
              accent
              icon={<IconUsers />}
            />
            <KpiCard
              label={isAdmin ? "Clientes Inativos" : "Meus Clientes Inativos"}
              value={fmtNumber(clienteMetrics?.total_inativos ?? 0)}
              sub="Situacao inativa"
              icon={<IconUsers />}
            />
            <KpiCard
              label={isAdmin ? "Novos no Periodo" : "Meus Novos no Periodo"}
              value={fmtNumber(clienteMetrics?.novos_no_periodo ?? 0)}
              sub={`Cadastrados em: ${PERIOD_LABELS[periodo]}`}
              icon={<IconUsers />}
            />
          </div>

          <div className="grid gap-6 lg:grid-cols-2">
            <Card>
              <CardHeader
                title={isAdmin ? "Clientes por Segmento" : "Meus Clientes por Segmento"}
                subtitle={
                  isAdmin
                    ? "Distribuicao por segmento de mercado"
                    : "Distribuicao da sua carteira por segmento de mercado"
                }
              />
              {loading && clienteMetrics === null ? (
                <div className="flex h-32 items-center justify-center">
                  <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-200 border-t-primary-600" />
                </div>
              ) : (
                <HorizontalBarList
                  items={(clienteMetrics?.por_segmento ?? []).map((s) => ({
                    label: s.segmento || "Nao informado",
                    total: s.total,
                  }))}
                  colorClass="bg-primary-500"
                />
              )}
            </Card>

            <Card>
              <CardHeader
                title={isAdmin ? "Clientes por UF" : "Meus Clientes por UF"}
                subtitle={
                  isAdmin
                    ? "Distribuicao por estado"
                    : "Distribuicao da sua carteira por estado"
                }
              />
              {loading && clienteMetrics === null ? (
                <div className="flex h-32 items-center justify-center">
                  <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-200 border-t-primary-600" />
                </div>
              ) : (
                <HorizontalBarList
                  items={(clienteMetrics?.por_uf ?? []).map((u) => ({
                    label: u.uf || "Nao informado",
                    total: u.total,
                  }))}
                  colorClass="bg-emerald-500"
                />
              )}
            </Card>
          </div>
        </div>

        {/* Loading overlay when refreshing */}
        {loading && !error && (
          <div className="fixed bottom-6 right-6 z-50">
            <div className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm text-slate-600 shadow-lg">
              <div className="h-4 w-4 animate-spin rounded-full border-2 border-primary-200 border-t-primary-600" />
              Atualizando dados...
            </div>
          </div>
        )}
      </main>
    </div>
  );
}

// ─── Page Export ─────────────────────────────────────────────────────────────

export default function DashboardPage() {
  return (
    <ProtectedRoute>
      <DashboardContent />
    </ProtectedRoute>
  );
}
