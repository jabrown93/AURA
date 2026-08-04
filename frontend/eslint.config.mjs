import eslintReact from "@eslint-react/eslint-plugin";
import js from "@eslint/js";
import prettier from "eslint-config-prettier";
import importPlugin from "eslint-plugin-import-x";
import jsxA11y from "eslint-plugin-jsx-a11y";
import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

export default [
  /* =========================
     Global ignores
     ========================= */
  {
    ignores: ["node_modules/**", ".next/**", "dist/**", "coverage/**", "*.config.*"],
  },

  /* =========================
     Base JS rules
     ========================= */
  js.configs.recommended,

  /* =========================
     TypeScript (NON type-aware, fast)
     ========================= */
  ...tseslint.configs.recommended,

  /* =========================
     React rules (@eslint-react). Only rules matching the old
     eslint-plugin-react posture are enabled; the recommended preset's extra
     rules are deliberately not adopted here.
     ========================= */
  {
    files: ["src/**/*.{ts,tsx}"],
    plugins: eslintReact.configs["recommended-typescript"].plugins,
    rules: {
      "@eslint-react/no-missing-key": "error",
    },
  },

  /* =========================
     TypeScript + React (type-aware, scoped)
     ========================= */
  {
    files: ["src/**/*.{ts,tsx}"],
    languageOptions: {
      parserOptions: {
        project: "./tsconfig.eslint.json",
        tsconfigRootDir: import.meta.dirname,
      },
    },
    plugins: {
      "react-hooks": reactHooks,
      "jsx-a11y": jsxA11y,
      "import-x": importPlugin,
    },
    settings: {
      "import-x/resolver": {
        typescript: {
          project: "./tsconfig.eslint.json",
        },
      },
    },
    rules: {
      /* Hooks */
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",

      /* Accessibility (lightweight) */
      "jsx-a11y/alt-text": "warn",
      "jsx-a11y/anchor-is-valid": "warn",

      /* TypeScript */
      "@typescript-eslint/no-unused-vars": ["warn", { argsIgnorePattern: "^_", varsIgnorePattern: "^_" }],
      "@typescript-eslint/no-explicit-any": "warn",

      /* Imports */
      "import-x/no-unresolved": "error",
      "import-x/named": "error",
      "import-x/no-cycle": "error",
      "import-x/no-self-import": "error",
      "import-x/no-duplicates": ["error", { "prefer-inline": true }],
    },
  },

  /* =========================
     Prettier (ALWAYS LAST)
     ========================= */
  prettier,
];
