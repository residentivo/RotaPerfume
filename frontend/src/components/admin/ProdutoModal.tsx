"use client";

import { useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Produto, ProdutoInput } from "@/lib/types";
import { useResetOnOpen } from "@/lib/useResetOnOpen";
import { vlog } from "@/lib/vlog";

const F = "ProdutoModal.tsx";

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
  vlog(F, "ProdutoModal", "criando estado sku, modo:", mode);
  const [sku, setSku] = useState("");
  vlog(F, "ProdutoModal", "criando estado descricao");
  const [descricao, setDescricao] = useState("");
  vlog(F, "ProdutoModal", "criando estado categoria");
  const [categoria, setCategoria] = useState("");
  vlog(F, "ProdutoModal", "criando estado marca");
  const [marca, setMarca] = useState("");
  vlog(F, "ProdutoModal", "criando estado notaOlfativa");
  const [notaOlfativa, setNotaOlfativa] = useState("");
  vlog(F, "ProdutoModal", "criando estado precoTabela");
  const [precoTabela, setPrecoTabela] = useState("");
  vlog(F, "ProdutoModal", "criando estado custoUnitario");
  const [custoUnitario, setCustoUnitario] = useState("");
  vlog(F, "ProdutoModal", "criando estado unidade");
  const [unidade, setUnidade] = useState("");
  vlog(F, "ProdutoModal", "criando estado dataLancamento");
  const [dataLancamento, setDataLancamento] = useState("");
  vlog(F, "ProdutoModal", "criando estado submitting");
  const [submitting, setSubmitting] = useState(false);
  vlog(F, "ProdutoModal", "criando estado error");
  const [error, setError] = useState<string | null>(null);

  // Reseta o formulario ao abrir (ou quando as props mudam com o modal
  // aberto) durante o render, sem setState em efeito — ver useResetOnOpen.
  vlog(F, "ProdutoModal", "registrando reset do formulário ao abrir");
  useResetOnOpen(open, [mode, produto], () => {
    vlog(F, "ProdutoModal.reset", "limpando erro");
    setError(null);
    vlog(F, "ProdutoModal.reset", "limpando estado de envio");
    setSubmitting(false);
    vlog(F, "ProdutoModal.reset", "verificando se é edição com produto, id:", produto?.id);
    if (mode === "edit" && produto) {
      vlog(F, "ProdutoModal.reset", "preenchendo SKU");
      setSku(produto.sku);
      vlog(F, "ProdutoModal.reset", "preenchendo descrição");
      setDescricao(produto.descricao);
      vlog(F, "ProdutoModal.reset", "preenchendo categoria");
      setCategoria(produto.categoria);
      vlog(F, "ProdutoModal.reset", "preenchendo marca");
      setMarca(produto.marca);
      vlog(F, "ProdutoModal.reset", "preenchendo nota olfativa");
      setNotaOlfativa(produto.nota_olfativa || "");
      vlog(F, "ProdutoModal.reset", "preenchendo preço de tabela");
      setPrecoTabela(String(produto.preco_tabela));
      vlog(F, "ProdutoModal.reset", "preenchendo custo unitário");
      setCustoUnitario(String(produto.custo_unitario));
      vlog(F, "ProdutoModal.reset", "preenchendo unidade");
      setUnidade(produto.unidade);
      vlog(F, "ProdutoModal.reset", "preenchendo data de lançamento");
      setDataLancamento(
        produto.data_lancamento ? produto.data_lancamento.slice(0, 10) : ""
      );
    } else {
      vlog(F, "ProdutoModal.reset", "limpando SKU");
      setSku("");
      vlog(F, "ProdutoModal.reset", "limpando descrição");
      setDescricao("");
      vlog(F, "ProdutoModal.reset", "limpando categoria");
      setCategoria("");
      vlog(F, "ProdutoModal.reset", "limpando marca");
      setMarca("");
      vlog(F, "ProdutoModal.reset", "limpando nota olfativa");
      setNotaOlfativa("");
      vlog(F, "ProdutoModal.reset", "limpando preço de tabela");
      setPrecoTabela("");
      vlog(F, "ProdutoModal.reset", "limpando custo unitário");
      setCustoUnitario("");
      vlog(F, "ProdutoModal.reset", "limpando unidade");
      setUnidade("");
      vlog(F, "ProdutoModal.reset", "limpando data de lançamento");
      setDataLancamento("");
    }
  });

  const handleSubmit = async (e: React.FormEvent) => {
    vlog(F, "ProdutoModal.handleSubmit", "impedindo submit padrão do form");
    e.preventDefault();
    vlog(F, "ProdutoModal.handleSubmit", "limpando erro");
    setError(null);

    vlog(F, "ProdutoModal.handleSubmit", "validando SKU na criação");
    if (mode === "create" && !sku.trim()) {
      vlog(F, "ProdutoModal.handleSubmit", "SKU ausente");
      setError("SKU e obrigatorio.");
      return;
    }
    vlog(F, "ProdutoModal.handleSubmit", "validando descrição");
    if (!descricao.trim()) {
      vlog(F, "ProdutoModal.handleSubmit", "descrição ausente");
      setError("Descricao e obrigatoria.");
      return;
    }
    vlog(F, "ProdutoModal.handleSubmit", "validando categoria");
    if (!categoria.trim()) {
      vlog(F, "ProdutoModal.handleSubmit", "categoria ausente");
      setError("Categoria e obrigatoria.");
      return;
    }
    vlog(F, "ProdutoModal.handleSubmit", "validando marca");
    if (!marca.trim()) {
      vlog(F, "ProdutoModal.handleSubmit", "marca ausente");
      setError("Marca e obrigatoria.");
      return;
    }
    vlog(F, "ProdutoModal.handleSubmit", "validando unidade");
    if (!unidade.trim()) {
      vlog(F, "ProdutoModal.handleSubmit", "unidade ausente");
      setError("Unidade e obrigatoria.");
      return;
    }
    vlog(F, "ProdutoModal.handleSubmit", "convertendo preço de tabela");
    const precoNum = Number(precoTabela);
    vlog(F, "ProdutoModal.handleSubmit", "validando preço de tabela:", precoNum);
    if (precoTabela.trim() === "" || Number.isNaN(precoNum) || precoNum < 0) {
      vlog(F, "ProdutoModal.handleSubmit", "preço de tabela inválido");
      setError("Preco de tabela deve ser um numero maior ou igual a zero.");
      return;
    }
    vlog(F, "ProdutoModal.handleSubmit", "convertendo custo unitário");
    const custoNum = Number(custoUnitario);
    vlog(F, "ProdutoModal.handleSubmit", "validando custo unitário:", custoNum);
    if (
      custoUnitario.trim() === "" ||
      Number.isNaN(custoNum) ||
      custoNum < 0
    ) {
      vlog(F, "ProdutoModal.handleSubmit", "custo unitário inválido");
      setError("Custo unitario deve ser um numero maior ou igual a zero.");
      return;
    }

    vlog(F, "ProdutoModal.handleSubmit", "marcando envio em andamento");
    setSubmitting(true);
    try {
      vlog(F, "ProdutoModal.handleSubmit", "enviando produto, modo:", mode);
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
      vlog(F, "ProdutoModal.handleSubmit", "falha ao salvar produto: montando mensagem de erro");
      const message =
        err instanceof Error ? err.message : "Erro ao salvar produto.";
      vlog(F, "ProdutoModal.handleSubmit", "exibindo erro");
      setError(message);
    } finally {
      vlog(F, "ProdutoModal.handleSubmit", "finalizando envio");
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

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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

        <div className="flex flex-wrap justify-end gap-2 pt-2">
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
