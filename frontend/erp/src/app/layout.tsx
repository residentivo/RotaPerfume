import './globals.css'
import { ReactNode } from 'react'
import { AuthProvider } from '@/lib/auth'

export const metadata = {
  title: 'ERP System',
  description: 'Sistema ERP - Frontend',
}

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="pt-BR">
      <body>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  )
}
