import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

// ESLint flat config (Next 16 removeu `next lint`; o script `lint` roda `eslint .`).
export default defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    rules: {
      // Regra do React Compiler (eslint-plugin-react-hooks 7). Refatorado em
      // FE-03: estado derivado no render, "ajustar estado quando a prop muda"
      // (src/lib/useResetOnOpen.ts) e setState so em callbacks assincronos.
      "react-hooks/set-state-in-effect": "error",
    },
  },
  globalIgnores([
    ".next/**",
    "out/**",
    "build/**",
    "coverage/**",
    "node_modules/**",
    "next-env.d.ts",
  ]),
]);
