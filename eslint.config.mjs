const eslintConfig = {
  root: true,
  extends: ["next/core-web-vitals", "next/typescript"],
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
  ignores: ["node_modules/**", ".next/**", "out/**", "build/**", "next-env.d.ts"],
};

export default eslintConfig;
