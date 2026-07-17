// ESLint 9 flat config — wrap eslint-config-next (which is still CommonJS
// legacy-config shaped) via @eslint/eslintrc FlatCompat. Without this wrapper
// the spread `[...nextCoreWebVitals, ...nextTypescript]` throws
// "nextCoreWebVitals is not iterable" because each export is a single object
// `{ extends: [...] }`, not an array of flat-config entries.
import { FlatCompat } from "@eslint/eslintrc";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const compat = new FlatCompat({ baseDirectory: __dirname });

const eslintConfig = [
  ...compat.extends("next/core-web-vitals", "next/typescript"),
  {
    rules: {
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/no-unused-vars": "warn",
      "@typescript-eslint/no-non-null-assertion": "off",
      "react-hooks/exhaustive-deps": "warn",
      "react/no-unescaped-entities": "off",
      "@next/next/no-img-element": "off",
      "no-console": "warn",
      "no-debugger": "error",
    },
  },
  {
    ignores: ["node_modules/**", ".next/**", "out/**", "build/**", "next-env.d.ts"],
  },
];

export default eslintConfig;

