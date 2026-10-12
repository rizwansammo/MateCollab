import storybook from 'eslint-plugin-storybook';
import { base, noRawColors, react } from '@matecollab/config/eslint';

export default [...base, ...react, ...noRawColors, ...storybook.configs['flat/recommended']];
