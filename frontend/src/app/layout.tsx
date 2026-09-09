import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "RotaPerfumes - Sistema de Gestao",
  description: "Sistema de gestao de clientes, estoque e pedidos",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="pt-BR">
      <body className="antialiased">{children}</body>
    </html>
  );
}
