import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

// Vitest + jsdom + Testing Library. O JSX dos componentes usa o runtime
// automatico (tsconfig "jsx": "react-jsx"), transformado pelo oxc do Vite.
// Os testes ficam em tests/, espelhando a estrutura de src/ (TST-01), e
// importam o codigo testado pelo alias "@/".
export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  oxc: {
    jsx: { runtime: "automatic" },
  },
  test: {
    environment: "jsdom",
    globals: false,
    setupFiles: ["./vitest.setup.ts"],
    include: ["tests/**/*.test.{ts,tsx}"],
    restoreMocks: true,
    env: {
      NEXT_PUBLIC_API_URL: "http://api.test",
    },
    coverage: {
      provider: "v8",
      reporter: ["text", "html"],
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.{ts,tsx}", "src/**/*.d.ts"],
      thresholds: {
        statements: 80,
        branches: 80,
        functions: 80,
        lines: 80,
      },
    },
  },
});
