// Shared flat ESLint configuration. Packages extend it:
//   import { base, react } from '@matecollab/config/eslint';
//   export default [...base, ...react];
// Accessibility is enforced at runtime with axe (Storybook a11y addon and Playwright checks),
// which covers rendered markup rather than JSX syntax alone.

import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export const ignores = {
  ignores: [
    '**/node_modules/**',
    '**/.next/**',
    '**/dist/**',
    '**/storybook-static/**',
    '**/coverage/**',
    '**/src/generated/**',
    '**/next-env.d.ts',
  ],
};

export const base = [
  ignores,
  js.configs.recommended,
  ...tseslint.configs.strict,
  {
    languageOptions: {
      ecmaVersion: 2023,
      sourceType: 'module',
      globals: { ...globals.browser, ...globals.node },
    },
    rules: {
      '@typescript-eslint/consistent-type-imports': ['error', { fixStyle: 'inline-type-imports' }],
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      'no-console': ['error', { allow: ['warn', 'error'] }],
      eqeqeq: ['error', 'always'],
      'no-restricted-syntax': [
        'error',
        {
          selector: "MemberExpression[object.name='localStorage']",
          message:
            'Business data never lives in localStorage; use the API. UI preferences use the theme/preferences helpers.',
        },
      ],
    },
  },
  {
    files: ['**/*.{js,mjs,cjs}'],
    ...tseslint.configs.disableTypeChecked,
  },
  {
    // Command-line scripts report progress on stdout.
    files: ['**/scripts/**/*.{js,mjs}'],
    rules: { 'no-console': 'off' },
  },
];

export const react = [
  {
    files: ['**/*.{ts,tsx}'],
    plugins: { 'react-hooks': reactHooks },
    rules: reactHooks.configs.recommended.rules,
  },
];
