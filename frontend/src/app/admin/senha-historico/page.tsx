"use client";

import { useEffect, useMemo, useState } from "react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import { Table, Badge, Column } from "@/components/ui/Table";
import { Select } from "@/components/ui/Select";
import { apiListSenhaHistorico } from "@/lib/api";
import { SenhaHistoricoItem, TipoReset } from "@/lib/types";

type SortKey = "created_at" | "usuario_nome" | "tipo_reset" | "resetado_por_nome" | "ip_origem";
type SortDir = "asc" | "desc";

// Mapeia a sortKey interna do frontend para o campo aceito pelo backend em
// order_by. Apenas id, usuario_id, tipo_reset e created_at estao na
// whitelist do backend — usuario_nome, resetado_por_nome e ip_origem nao
// existem la, entao nao enviamos order_by para essas colunas (a ordenacao
// cai no default do backend: id desc).
const ORDER_BY_MAP: Partial<Record<SortKey, string>> = {
  created_at: "created_at",
  tipo_reset: "tipo_reset",
};

const TIPO_OPTIONS: { value: "" | TipoReset; label: string }[] = [
  { value: "", label: "Todos os tipos" },
  { value: "proprio", label: "Proprio" },
  { value: "admin", label: "Admin" },
  { value: "primeiro_login", label: "Primeiro Login" },
];

const LIMIT_OPTIONS = [
  { value: "10", label: "10 por pagina" },
  { value: "20", label: "20 por pagina" },
  { value: "50", label: "50 por pagina" },
  { value: "100", label: "100 por pagina" },
];

function tipoBadgeColor(tipo: TipoReset): "blue" | "yellow" | "green" {
  if (tipo === "proprio") return "blue";
  if (tipo === "admin") return "yellow";
  return "green";
}

function tipoLabel(tipo: TipoReset): string {
  if (tipo === "proprio") return "Proprio";
  if (tipo === "admin") return "Admin";
  return "Primeiro Login";
}

function formatDateTime(iso: string): string {
  if (!iso) return "-";
  try {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    return d.toLocaleString("pt-BR", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  } catch {
    return iso;
  }
}

export default function SenhaHistoricoPage() {
  const [items, setItems] = useState<SenhaHistoricoItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [tipo, setTipo] = useState<"" | TipoReset>("");

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [total, setTotal] = useState(0);
  const [pages, setPages] = useState(0);

  const [sortKey, setSortKey] = useState<SortKey>("created_at");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await apiListSenhaHistorico(
        page,
        limit,
        undefined,
        tipo || undefined,
        ORDER_BY_MAP[sortKey],
        sortDir
      );
      setItems(res.data);
      setTotal(res.total);
      setPages(res.pages);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erro ao carregar historico de senhas. O endpoint /api/senha-historico pode nao existir no backend.";
      setError(message);
      setItems([]);
      setTotal(0);
      setPages(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, limit, tipo, sortKey, sortDir]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const handleRefresh = () => {
    load();
  };

  // Reset para pagina 1 quando filtros mudam
  useEffect(() => {
    setPage(1);
  }, [search, tipo]);

  // Busca continua client-side (aplicada sobre os itens da pagina atual);
  // a ordenacao agora e feita pela API quando o campo esta na whitelist do
  // backend (ver ORDER_BY_MAP e load()).
  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    let list = items;
    if (term) {
      list = list.filter((it) => {
        const usuario = (it.usuario_nome || "").toLowerCase();
        const resetador = (it.resetado_por_nome || "").toLowerCase();
        const ip = (it.ip_origem || "").toLowerCase();
        return (
          usuario.includes(term) ||
          resetador.includes(term) ||
          ip.includes(term) ||
          String(it.usuario_id).includes(term)
        );
      });
    }
    return list;
  }, [items, search]);

  const SortableHeader = ({
    label,
    sortKeyName,
    align,
  }: {
    label: string;
    sortKeyName: SortKey;
    align?: "left" | "center" | "right";
  }) => (
    <button
      type="button"
      onClick={() => handleSort(sortKeyName)}
      className={[
        "inline-flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-slate-600 hover:text-slate-900",
        align === "center"
          ? "justify-center"
          : align === "right"
          ? "justify-end"
          : "",
      ].join(" ")}
    >
      {label}
      {sortKey === sortKeyName && (
        <span className="text-primary-600">
          {sortDir === "asc" ? "↑" : "↓"}
        </span>
      )}
    </button>
  );

  const columns: Column<SenhaHistoricoItem>[] = [
    {
      key: "created_at",
      header: "Data/Hora",
      sortable: true,
      sortValue: (it) => new Date(it.created_at).getTime(),
      render: (it) => (
        <span className="font-mono text-xs text-slate-700">
          {formatDateTime(it.created_at)}
        </span>
      ),
    },
    {
      key: "usuario_nome",
      header: "Usuario",
      sortable: true,
      sortValue: (it) => it.usuario_nome,
      render: (it) => (
        <div className="flex flex-col">
          <span className="font-medium text-slate-900">
            #{it.usuario_id} - {it.usuario_nome || "(sem nome)"}
          </span>
        </div>
      ),
    },
    {
      key: "tipo_reset",
      header: "Tipo Reset",
      width: "160px",
      align: "center",
      sortable: true,
      sortValue: (it) => it.tipo_reset,
      render: (it) => (
        <Badge color={tipoBadgeColor(it.tipo_reset)}>
          {tipoLabel(it.tipo_reset)}
        </Badge>
      ),
    },
    {
      key: "resetado_por_nome",
      header: "Resetado Por",
      sortable: true,
      sortValue: (it) => it.resetado_por_nome || "",
      render: (it) =>
        it.resetado_por_nome ? (
          <span className="text-slate-700">
            #{it.resetado_por_id} - {it.resetado_por_nome}
          </span>
        ) : (
          <span className="text-slate-400">-</span>
        ),
    },
    {
      key: "ip_origem",
      header: "IP Origem",
      width: "160px",
      sortable: true,
      sortValue: (it) => it.ip_origem || "",
      render: (it) =>
        it.ip_origem ? (
          <span className="font-mono text-xs text-slate-600">
            {it.ip_origem}
          </span>
        ) : (
          <span className="text-slate-400">-</span>
        ),
    },
  ];

  const startItem = total === 0 ? 0 : (page - 1) * limit + 1;
  const endItem = Math.min(page * limit, total);

  return (
    <div>
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">
            Auditoria de Senha
          </h1>
          <p className="mt-1 text-sm text-slate-500">
            Historico de todas as alteracoes de senha do sistema. Apenas
            administradores tem acesso.
          </p>
        </div>
        <Button
          onClick={handleRefresh}
          variant="secondary"
          title="Recarregar"
        >
          <svg
            className="mr-1.5 inline-block h-4 w-4"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 4v6h6M20 20v-6h-6M5 13a8 8 0 0014.36 4M19 11A8 8 0 004.64 7"
            />
          </svg>
          Atualizar
        </Button>
      </div>

      {error && (
        <div className="mb-4">
          <Alert variant="error" onClose={() => setError(null)}>
            {error}
          </Alert>
        </div>
      )}

      <Card padded={false}>
        <div className="border-b border-slate-200 p-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
            <div className="flex flex-1 flex-col gap-3 sm:flex-row sm:items-end">
              <div className="flex-1 sm:max-w-xs">
                <Input
                  label="Buscar"
                  placeholder="Nome, IP ou ID..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  icon={
                    <svg
                      className="h-5 w-5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M21 21l-4.35-4.35M11 19a8 8 0 100-16 8 8 0 000 16z"
                      />
                    </svg>
                  }
                />
              </div>
              <div className="sm:w-56">
                <Select
                  label="Tipo de Reset"
                  options={TIPO_OPTIONS}
                  value={tipo}
                  onChange={(e) =>
                    setTipo(e.target.value as "" | TipoReset)
                  }
                />
              </div>
              <div className="sm:w-44">
                <Select
                  label="Itens por pagina"
                  options={LIMIT_OPTIONS}
                  value={String(limit)}
                  onChange={(e) => setLimit(Number(e.target.value))}
                />
              </div>
            </div>
            <div className="text-sm text-slate-500">
              {total === 0
                ? "0 registros"
                : `${startItem}-${endItem} de ${total} ${
                    total === 1 ? "registro" : "registros"
                  }`}
            </div>
          </div>
        </div>

        <div className="p-4">
          <Table
            columns={columns}
            data={filtered}
            keyExtractor={(it) => it.id}
            loading={loading}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={(key) => handleSort(key as SortKey)}
            emptyMessage={
              search || tipo
                ? "Nenhum registro encontrado para os filtros aplicados."
                : "Nenhuma alteracao de senha registrada."
            }
          />
        </div>

        {pages > 1 && (
          <div className="flex flex-col items-center justify-between gap-3 border-t border-slate-200 px-4 py-3 sm:flex-row">
            <div className="text-sm text-slate-500">
              Pagina <strong>{page}</strong> de <strong>{pages}</strong>
            </div>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="secondary"
                disabled={page <= 1}
                onClick={() => setPage(1)}
                title="Primeira pagina"
              >
                {"<<"}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                title="Pagina anterior"
              >
                {"<"}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={page >= pages}
                onClick={() => setPage((p) => Math.min(pages, p + 1))}
                title="Proxima pagina"
              >
                {">"}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={page >= pages}
                onClick={() => setPage(pages)}
                title="Ultima pagina"
              >
                {">>"}
              </Button>
            </div>
          </div>
        )}
      </Card>

      <div className="mt-4 text-xs text-slate-400">
        <strong>Legenda dos tipos:</strong>{" "}
        <Badge color="blue">Proprio</Badge> alteracao feita pelo proprio
        usuario,{" "}
        <Badge color="yellow">Admin</Badge> reset feito por um administrador,{" "}
        <Badge color="green">Primeiro Login</Badge> alteracao obrigatoria no
        primeiro acesso.
        <br />
        <strong>Nota:</strong> Se a lista estiver vazia, verifique se os
        endpoints <code>GET /api/senha-historico</code> e{" "}
        <code>GET /api/senha-historico/&#123;usuario_id&#125;</code> estao
        implementados no backend.
      </div>
    </div>
  );
}
