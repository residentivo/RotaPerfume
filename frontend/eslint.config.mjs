import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

// ESLint flat config (Next 16 removeu `next lint`; o script `lint` roda `eslint .`).
export default defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    rules: {
      // Regra nova do React Compiler (eslint-plugin-react-hooks 7). O projeto
      // usa em massa o padrao "resetar/carregar estado em useEffect ao abrir
      // modal ou mudar filtro" (~30 ocorrencias). Fica como warn ate uma
      // refatoracao dedicada; nao e bug funcional.
      "react-hooks/set-state-in-effect": "warn",
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
