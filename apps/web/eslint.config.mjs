import nextPlugin from '@next/eslint-plugin-next';
import { base, noRawColors, react } from '@matecollab/config/eslint';

export default [
  ...base,
  ...react,
  ...noRawColors,
  {
    plugins: { '@next/next': nextPlugin },
    rules: {
      ...nextPlugin.configs.recommended.rules,
      ...nextPlugin.configs['core-web-vitals'].rules,
      // Brand SVGs are rendered unchanged through <BrandMark>/<BrandLockup> (no optimisation).
      '@next/next/no-img-element': 'off',
    },
  },
];
