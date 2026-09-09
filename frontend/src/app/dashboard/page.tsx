"use client";

import { useCallback, useEffect, useState } from "react";
import { Navbar } from "@/components/layout/Navbar";
import { Card, CardHeader } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import {
  apiDashboardMetrics,
  apiDashboardVendas,
  apiDashboardVendedores,
} from "@/lib/api";
import {
  DashboardMetrics,
  DashboardPeriodo,
  VendasSeries,
  VendaDiaria,
  VendedorRanking,
} from "@/lib/types";

// ─── Helpers ────────────────────────────────────────────────────────────────

function fmtCurrency(value: number): string {
  return new Intl.NumberFormat("pt-BR", {
    style: "currency",
    currency: "BRL",
  }).format(value);
}

function fmtNumber(value: number): string {
  return new Intl.NumberFormat("pt-BR").format(value);
}

function fmtPercent(value: number): string {
  return `${Math.min(100, Math.max(0, value)).toFixed(1)}%`;
}

function fmtDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-").map(Number);
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
        <div className="relative flex flex-1 items-end gap-[2px]">
          {sampled.map((pt, i) => {
            const pct = (pt.total_vendas / maxVendas) * 100;
            return (
              <div
                key={i}
                className="group relative flex-1 cursor-default"
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

// ─── Content ─────────────────────────────────────────────────────────────────

function DashboardContent() {
  const [periodo, setPeriodo] = useState<PeriodFilter>("month");
  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null);
  const [vendasSeries, setVendasSeries] = useState<VendasSeries | null>(null);
  const [vendedores, setVendedores] = useState<VendedorRanking[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [chartsDias, setChartsDias] = useState(30);

  const apiPeriodo: DashboardPeriodo = periodo === "week" ? "today" : periodo;

  const loadData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [m, v, vd] = await Promise.all([
        apiDashboardMetrics(apiPeriodo),
        apiDashboardVendas(chartsDias),
        apiDashboardVendedores(1, 10),
      ]);
      setMetrics(m);
      setVendasSeries(v);
      setVendedores(vd.data.slice(0, 10));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao carregar dados");
    } finally {
      setLoading(false);
    }
  }, [apiPeriodo, chartsDias]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Dados simulados para visualizacao enquanto backend nao tem dados reais
  const hasRealData = metrics !== null && vendasSeries !== null;
  const demoMetrics: DashboardMetrics = metrics ?? {
    total_vendas: 184750,
    total_pedidos: 342,
    ticket_medio: 540,
    total_clientes: 128,
    meta_mes: 200000,
    atingimento_meta: 92.4,
    periodo: apiPeriodo,
  };

  const demoVendas: VendasSeries = vendasSeries ?? {
    dias: chartsDias,
    pontos: Array.from({ length: chartsDias }, (_, i) => {
      const d = new Date();
      d.setDate(d.getDate() - (chartsDias - 1 - i));
      const base = 4000 + Math.random() * 6000;
      const spike = i === 15 || i === 22 ? 1.8 : 1;
      return {
        dia: d.toISOString().split("T")[0],
        total_vendas: Math.round(base * spike),
        total_pedidos: Math.round(base / 500),
      };
    }),
  };

  return (
    <div className="min-h-screen bg-slate-50">
      <Navbar />
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Header */}
        <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold text-slate-900">Dashboard</h1>
            <p className="mt-0.5 text-sm text-slate-500">
              Visao geral das metricas de vendas
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
          <Alert variant="danger" className="mb-4">
            {error} — mostrando dados demonstracao.
          </Alert>
        )}

        {/* KPI Cards */}
        <div className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <KpiCard
            label="Total de Vendas"
            value={fmtCurrency(demoMetrics.total_vendas)}
            sub={`Periodo: ${PERIOD_LABELS[periodo]}`}
            accent
            icon={<IconCurrency />}
          />
          <KpiCard
            label="Pedidos"
            value={fmtNumber(demoMetrics.total_pedidos)}
            sub="Pedidos realizados"
            icon={<IconCart />}
          />
          <KpiCard
            label="Ticket Medio"
            value={fmtCurrency(demoMetrics.ticket_medio)}
            sub="Por pedido"
            icon={<IconTicket />}
          />
          <KpiCard
            label="Ranking Vendedores"
            value={vendedores.length > 0 ? fmtCurrency(vendedores[0]?.total_vendas ?? 0) : "-"}
            sub={vendedores[0] ? `Lider: ${vendedores[0].vendedor_nome}` : "Carregando..."}
            icon={<IconRanking />}
          />
        </div>

        {/* Charts Row */}
        <div className="mb-6 grid gap-6 lg:grid-cols-3">
          {/* Grafico de Vendas */}
          <Card className="lg:col-span-2">
            <CardHeader
              title="Vendas nos Ultimos 30 Dias"
              subtitle={`Total: ${fmtCurrency(demoVendas.pontos.reduce((s, p) => s + p.total_vendas, 0))}`}
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
              <BarChart series={demoVendas} />
            )}
          </Card>

          {/* Metas */}
          <Card>
            <CardHeader title="Acompanhamento de Metas" />
            <div className="space-y-4">
              <GoalProgress
                label="Meta Mensal de Vendas"
                current={demoMetrics.total_vendas}
                target={demoMetrics.meta_mes ?? demoMetrics.total_vendas * 1.1}
              />
              <GoalProgress
                label="Meta de Pedidos"
                current={demoMetrics.total_pedidos}
                target={Math.round(demoMetrics.total_pedidos * 1.15)}
              />
              <GoalProgress
                label="Meta de Clientes"
                current={demoMetrics.total_clientes}
                target={Math.round(demoMetrics.total_clientes * 1.2)}
              />
            </div>
          </Card>
        </div>

        {/* Ranking Table */}
        <Card>
          <CardHeader
            title="Ranking de Vendedores"
            subtitle="Top 10 por volume de vendas no periodo"
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
          ) : (
            <RankingTable vendedores={vendedores.length > 0 ? vendedores : [
              { vendedor_id: 1, vendedor_nome: "Carlos Silva", total_vendas: 45000, total_pedidos: 92, ticket_medio: 489, meta: 40000, atingimento_meta: 112 },
              { vendedor_id: 2, vendedor_nome: "Ana Beatriz Santos", total_vendas: 38200, total_pedidos: 78, ticket_medio: 490, meta: 40000, atingimento_meta: 95 },
              { vendedor_id: 3, vendedor_nome: "Ricardo Oliveira", total_vendas: 34100, total_pedidos: 71, ticket_medio: 480, meta: 40000, atingimento_meta: 85 },
              { vendedor_id: 4, vendedor_nome: "Fernanda Costa", total_vendas: 29500, total_pedidos: 62, ticket_medio: 476, meta: 35000, atingimento_meta: 84 },
              { vendedor_id: 5, vendedor_nome: "Bruno Almeida", total_vendas: 25800, total_pedidos: 55, ticket_medio: 469, meta: 35000, atingimento_meta: 74 },
              { vendedor_id: 6, vendedor_nome: "Patricia Lima", total_vendas: 22100, total_pedidos: 48, ticket_medio: 460, meta: 30000, atingimento_meta: 74 },
              { vendedor_id: 7, vendedor_nome: "Marcos Pereira", total_vendas: 18900, total_pedidos: 41, ticket_medio: 461, meta: 30000, atingimento_meta: 63 },
              { vendedor_id: 8, vendedor_nome: "Juliana Rocha", total_vendas: 15200, total_pedidos: 33, ticket_medio: 461, meta: 25000, atingimento_meta: 61 },
              { vendedor_id: 9, vendedor_nome: "Thiago Ferreira", total_vendas: 11800, total_pedidos: 26, ticket_medio: 454, meta: 25000, atingimento_meta: 47 },
              { vendedor_id: 10, vendedor_nome: "Luciana Martins", total_vendas: 8500, total_pedidos: 19, ticket_medio: 447, meta: 20000, atingimento_meta: 43 },
            ]} />
          )}
        </Card>

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
    <ProtectedRoute requireAdmin={true}>
      <DashboardContent />
    </ProtectedRoute>
  );
}
