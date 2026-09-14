"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Produto, ProdutoInput } from "@/lib/types";

interface ProdutoModalProps {
  open: boolean;
  mode: "create" | "edit";
  produto: Produto | null;
  onClose: () => void;
  onSubmit: (data: ProdutoInput) => Promise<void>;
}

export function ProdutoModal({
  open,
  mode,
  produto,
  onClose,
  onSubmit,
}: ProdutoModalProps) {
  const [sku, setSku] = useState("");
  const [descricao, setDescricao] = useState("");
  const [categoria, setCategoria] = useState("");
  const [marca, setMarca] = useState("");
  const [notaOlfativa, setNotaOlfativa] = useState("");
  const [precoTabela, setPrecoTabela] = useState("");
  const [custoUnitario, setCustoUnitario] = useState("");
  const [unidade, setUnidade] = useState("");
  const [dataLancamento, setDataLancamento] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setError(null);
      setSubmitting(false);
      if (mode === "edit" && produto) {
        setSku(produto.sku);
        setDescricao(produto.descricao);
        setCategoria(produto.categoria);
        setMarca(produto.marca);
        setNotaOlfativa(produto.nota_olfativa || "");
        setPrecoTabela(String(produto.preco_tabela));
        setCustoUnitario(String(produto.custo_unitario));
        setUnidade(produto.unidade);
        setDataLancamento(
          produto.data_lancamento ? produto.data_lancamento.slice(0, 10) : ""
        );
      } else {
        setSku("");
        setDescricao("");
        setCategoria("");
        setMarca("");
        setNotaOlfativa("");
        setPrecoTabela("");
        setCustoUnitario("");
        setUnidade("");
        setDataLancamento("");
      }
    }
  }, [open, mode, produto]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (mode === "create" && !sku.trim()) {
      setError("SKU e obrigatorio.");
      return;
    }
    if (!descricao.trim()) {
      setError("Descricao e obrigatoria.");
      return;
    }
    if (!categoria.trim()) {
      setError("Categoria e obrigatoria.");
      return;
    }
    if (!marca.trim()) {
      setError("Marca e obrigatoria.");
      return;
    }
    if (!unidade.trim()) {
      setError("Unidade e obrigatoria.");
      return;
    }
    const precoNum = Number(precoTabela);
    if (precoTabela.trim() === "" || Number.isNaN(precoNum) || precoNum < 0) {
      setError("Preco de tabela deve ser um numero maior ou igual a zero.");
      return;
    }
    const custoNum = Number(custoUnitario);
    if (
      custoUnitario.trim() === "" ||
      Number.isNaN(custoNum) ||
      custoNum < 0
    ) {
      setError("Custo unitario deve ser um numero maior ou igual a zero.");
      return;
    }

    setSubmitting(true);
    try {
      await onSubmit({
        sku: sku.trim(),
        descricao: descricao.trim(),
        categoria: categoria.trim(),
        marca: marca.trim(),
        nota_olfativa: notaOlfativa.trim() || undefined,
        preco_tabela: precoNum,
        custo_unitario: custoNum,
        unidade: unidade.trim(),
        data_lancamento: dataLancamento || undefined,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Erro ao salvar produto.";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Novo Produto" : "Editar Produto"}
      size="md"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Alert variant="error">{error}</Alert>}

        {mode === "create" && (
          <Input
            label="SKU"
            value={sku}
            onChange={(e) => setSku(e.target.value)}
            placeholder="Ex: PRF-0001"
            required
            autoFocus
          />
        )}

        <Input
          label="Descricao"
          value={descricao}
          onChange={(e) => setDescricao(e.target.value)}
          placeholder="Ex: Perfume Exemplo 100ml"
          required
          autoFocus={mode === "edit"}
        />

        <div className="grid grid-cols-2 gap-3">
          <Input
            label="Categoria"
            value={categoria}
            onChange={(e) => setCategoria(e.target.value)}
            placeholder="Ex: Perfumaria"
            required
          />
          <Input
            label="Marca"
            value={marca}
            onChange={(e) => setMarca(e.target.value)}
            placeholder="Ex: Marca X"
            required
          />
        </div>

        <Input
          label="Nota olfativa"
          value={notaOlfativa}
          onChange={(e) => setNotaOlfativa(e.target.value)}
          placeholder="Ex: Amadeirado"
        />

        <div className="grid grid-cols-2 gap-3">
          <Input
            label="Preco de tabela"
            type="number"
            step="0.01"
            min="0"
            value={precoTabela}
            onChange={(e) => setPrecoTabela(e.target.value)}
            placeholder="0,00"
            required
          />
          <Input
            label="Custo unitario"
            type="number"
            step="0.01"
            min="0"
            value={custoUnitario}
            onChange={(e) => setCustoUnitario(e.target.value)}
            placeholder="0,00"
            required
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Input
            label="Unidade"
            value={unidade}
            onChange={(e) => setUnidade(e.target.value)}
            placeholder="Ex: UN"
            required
          />
          <Input
            label="Data de lancamento"
            type="date"
            value={dataLancamento}
            onChange={(e) => setDataLancamento(e.target.value)}
            helperText="Opcional"
          />
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button type="submit" loading={submitting}>
            {mode === "create" ? "Criar produto" : "Salvar alteracoes"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
