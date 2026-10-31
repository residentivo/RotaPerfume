"use client";

import { useEffect, useRef, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { Produto, ProdutoInput } from "@/lib/types";
import { useResetOnOpen } from "@/lib/useResetOnOpen";
import { enviarImagemProduto, removerImagemProduto } from "@/lib/api";
import { useSessionUser } from "@/lib/session";
import {
  TIPOS_IMAGEM_PRODUTO,
  urlImagemProduto,
  validarArquivoImagem,
} from "@/lib/imagemProduto";
import { vlog } from "@/lib/vlog";

const F = "ProdutoModal.tsx";

interface ProdutoModalProps {
  open: boolean;
  mode: "create" | "edit";
  produto: Produto | null;
  onClose: () => void;
  onSubmit: (data: ProdutoInput) => Promise<void>;
  /** IMG-01: chamado com o produto devolvido pela API apos enviar/remover a imagem. */
  onImagemAlterada?: (produto: Produto) => void;
}

export function ProdutoModal({
  open,
  mode,
  produto,
  onClose,
  onSubmit,
  onImagemAlterada,
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

        {/* IMG-01: imagem so na edicao (upload/remocao exigem produto salvo). */}
        {mode === "edit" && produto && (
          <ImagemProdutoSecao
            open={open}
            produto={produto}
            onImagemAlterada={onImagemAlterada}
          />
        )}

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

interface ImagemProdutoSecaoProps {
  open: boolean;
  produto: Produto;
  onImagemAlterada?: (produto: Produto) => void;
}

/**
 * IMG-01: imagem atual (ou placeholder), seletor com preview e botoes
 * Enviar/Trocar e Remover (somente admin). Mantem a imagem em estado local
 * para nao trocar a prop `produto` e resetar o formulario em edicao.
 */
function ImagemProdutoSecao({ open, produto, onImagemAlterada }: ImagemProdutoSecaoProps) {
  vlog(F, "ImagemProdutoSecao", "lendo usuário da sessão");
  const sessionUser = useSessionUser();
  vlog(F, "ImagemProdutoSecao", "verificando se é admin");
  const isAdmin = sessionUser?.role === "admin";
  vlog(F, "ImagemProdutoSecao", "criando estado imagemUrl");
  const [imagemUrl, setImagemUrl] = useState<string | null>(null);
  vlog(F, "ImagemProdutoSecao", "criando estado arquivo");
  const [arquivo, setArquivo] = useState<File | null>(null);
  vlog(F, "ImagemProdutoSecao", "criando estado previewUrl");
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  vlog(F, "ImagemProdutoSecao", "criando estado enviando");
  const [enviando, setEnviando] = useState<"enviar" | "remover" | null>(null);
  vlog(F, "ImagemProdutoSecao", "criando estado erro");
  const [erro, setErro] = useState<string | null>(null);
  vlog(F, "ImagemProdutoSecao", "criando estado sucesso");
  const [sucesso, setSucesso] = useState<string | null>(null);
  vlog(F, "ImagemProdutoSecao", "criando ref do input de arquivo");
  const inputRef = useRef<HTMLInputElement>(null);

  vlog(F, "ImagemProdutoSecao", "registrando reset da imagem ao abrir");
  useResetOnOpen(open, [produto], () => {
    vlog(F, "ImagemProdutoSecao.reset", "carregando imagem atual do produto, id:", produto.id);
    setImagemUrl(produto.imagem_url ?? null);
    vlog(F, "ImagemProdutoSecao.reset", "limpando arquivo, preview e mensagens");
    setArquivo(null);
    setPreviewUrl(null);
    setEnviando(null);
    setErro(null);
    setSucesso(null);
  });

  // Libera o object URL do preview quando ele e trocado ou no unmount.
  vlog(F, "ImagemProdutoSecao", "registrando liberação do preview");
  useEffect(() => {
    if (!previewUrl) return;
    return () => {
      vlog(F, "ImagemProdutoSecao.useEffect", "revogando object URL do preview");
      URL.revokeObjectURL(previewUrl);
    };
  }, [previewUrl]);

  const limparSelecao = () => {
    vlog(F, "ImagemProdutoSecao.limparSelecao", "limpando arquivo e preview");
    setArquivo(null);
    setPreviewUrl(null);
    if (inputRef.current) inputRef.current.value = "";
  };

  const handleArquivo = (e: React.ChangeEvent<HTMLInputElement>) => {
    vlog(F, "ImagemProdutoSecao.handleArquivo", "lendo arquivo selecionado");
    const file = e.target.files?.[0] ?? null;
    setErro(null);
    setSucesso(null);
    if (!file) {
      limparSelecao();
      return;
    }
    vlog(F, "ImagemProdutoSecao.handleArquivo", "validando arquivo no cliente (UX)");
    const invalido = validarArquivoImagem(file);
    if (invalido) {
      vlog(F, "ImagemProdutoSecao.handleArquivo", "arquivo recusado no cliente");
      setErro(invalido);
      limparSelecao();
      return;
    }
    vlog(F, "ImagemProdutoSecao.handleArquivo", "gerando preview do arquivo");
    setArquivo(file);
    setPreviewUrl(URL.createObjectURL(file));
  };

  const handleEnviar = async () => {
    if (!arquivo) return;
    vlog(F, "ImagemProdutoSecao.handleEnviar", "enviando imagem, produto:", produto.id);
    setEnviando("enviar");
    setErro(null);
    setSucesso(null);
    try {
      const atualizado = await enviarImagemProduto(produto.id, arquivo);
      vlog(F, "ImagemProdutoSecao.handleEnviar", "imagem enviada; atualizando estado");
      setImagemUrl(atualizado.imagem_url ?? null);
      limparSelecao();
      setSucesso("Imagem enviada com sucesso.");
      onImagemAlterada?.(atualizado);
    } catch (err) {
      vlog(F, "ImagemProdutoSecao.handleEnviar", "falha ao enviar imagem");
      setErro(err instanceof Error ? err.message : "Erro ao enviar imagem.");
    } finally {
      setEnviando(null);
    }
  };

  const handleRemover = async () => {
    vlog(F, "ImagemProdutoSecao.handleRemover", "pedindo confirmação de remoção");
    if (!window.confirm(`Remover a imagem do produto "${produto.descricao}"?`)) return;
    vlog(F, "ImagemProdutoSecao.handleRemover", "removendo imagem, produto:", produto.id);
    setEnviando("remover");
    setErro(null);
    setSucesso(null);
    try {
      const atualizado = await removerImagemProduto(produto.id);
      vlog(F, "ImagemProdutoSecao.handleRemover", "imagem removida; atualizando estado");
      setImagemUrl(atualizado.imagem_url ?? null);
      limparSelecao();
      setSucesso("Imagem removida com sucesso.");
      onImagemAlterada?.(atualizado);
    } catch (err) {
      vlog(F, "ImagemProdutoSecao.handleRemover", "falha ao remover imagem");
      setErro(err instanceof Error ? err.message : "Erro ao remover imagem.");
    } finally {
      setEnviando(null);
    }
  };

  vlog(F, "ImagemProdutoSecao", "calculando URL exibida (preview ou atual)");
  const urlAtual = urlImagemProduto(imagemUrl);
  const urlExibida = previewUrl ?? urlAtual;

  return (
    <fieldset className="rounded-lg border border-slate-200 p-3">
      <legend className="px-1 text-sm font-medium text-slate-700">Imagem</legend>

      {erro && (
        <div className="mb-3">
          <Alert variant="error" onClose={() => setErro(null)}>
            {erro}
          </Alert>
        </div>
      )}
      {sucesso && (
        <div className="mb-3">
          <Alert variant="success" onClose={() => setSucesso(null)}>
            {sucesso}
          </Alert>
        </div>
      )}

      <div className="flex flex-col gap-3 sm:flex-row sm:items-start">
        {urlExibida ? (
          // eslint-disable-next-line @next/next/no-img-element -- IMG-01: imagem servida pela API (rota publica), sem otimizacao do next/image
          <img
            src={urlExibida}
            alt={previewUrl ? `Pre-visualizacao: ${produto.descricao}` : produto.descricao}
            loading="lazy"
            decoding="async"
            className="h-28 w-28 flex-shrink-0 rounded-lg border border-slate-200 bg-slate-50 object-cover"
          />
        ) : (
          <div
            className="flex h-28 w-28 flex-shrink-0 items-center justify-center rounded-lg border border-dashed border-slate-300 bg-slate-50 text-xs text-slate-400"
            aria-label="Produto sem imagem"
          >
            Sem imagem
          </div>
        )}

        {isAdmin && (
          <div className="flex flex-1 flex-col gap-2">
            <label className="text-sm text-slate-700" htmlFor={`imagem-produto-${produto.id}`}>
              Arquivo (JPEG, PNG ou WebP, ate 1 MB)
            </label>
            <input
              ref={inputRef}
              id={`imagem-produto-${produto.id}`}
              type="file"
              accept={TIPOS_IMAGEM_PRODUTO}
              onChange={handleArquivo}
              disabled={enviando !== null}
              className="block w-full text-sm text-slate-600 file:mr-3 file:rounded-lg file:border-0 file:bg-slate-200 file:px-3 file:py-1.5 file:text-sm file:font-medium file:text-slate-800 hover:file:bg-slate-300"
            />
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                size="sm"
                onClick={handleEnviar}
                disabled={!arquivo || enviando !== null}
                loading={enviando === "enviar"}
              >
                {imagemUrl ? "Trocar imagem" : "Enviar imagem"}
              </Button>
              {previewUrl && (
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  onClick={limparSelecao}
                  disabled={enviando !== null}
                >
                  Descartar selecao
                </Button>
              )}
              {imagemUrl && (
                <Button
                  type="button"
                  size="sm"
                  variant="danger"
                  onClick={handleRemover}
                  disabled={enviando !== null}
                  loading={enviando === "remover"}
                >
                  Remover imagem
                </Button>
              )}
            </div>
          </div>
        )}
      </div>
    </fieldset>
  );
}
