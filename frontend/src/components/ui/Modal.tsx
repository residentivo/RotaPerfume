"use client";

import { ReactNode, useEffect } from "react";
import { vlog } from "@/lib/vlog";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  size?: "sm" | "md" | "lg";
}

const sizeClasses: Record<string, string> = {
  sm: "max-w-md",
  md: "max-w-lg",
  lg: "max-w-2xl",
};

export function Modal({ open, onClose, title, children, size = "md" }: ModalProps) {
  // Fechar com ESC
  vlog("Modal.tsx", "Modal", "registrando efeito de fechar com ESC");
  useEffect(() => {
    vlog("Modal.tsx", "Modal.useEffect", "verificando se o modal está aberto:", open);
    if (!open) return;
    vlog("Modal.tsx", "Modal.useEffect", "criando handler de teclado");
    const handler = (e: KeyboardEvent) => {
      vlog("Modal.tsx", "Modal.handler", "verificando se a tecla é Escape");
      if (e.key === "Escape") onClose();
    };
    vlog("Modal.tsx", "Modal.useEffect", "registrando listener de keydown");
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, onClose]);

  vlog("Modal.tsx", "Modal", "verificando se deve renderizar o modal:", open);
  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      aria-modal="true"
      role="dialog"
    >
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Panel */}
      <div
        className={[
          "relative w-full rounded-xl bg-white shadow-2xl",
          sizeClasses[size],
        ].join(" ")}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-slate-900">{title}</h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 transition-colors"
            aria-label="Fechar"
          >
            <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
              <path
                fillRule="evenodd"
                d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                clipRule="evenodd"
              />
            </svg>
          </button>
        </div>

        {/* Body */}
        <div className="px-6 py-5">{children}</div>
      </div>
    </div>
  );
}
