import js from '@eslint/js';
import globals from 'globals';
import svelte from 'eslint-plugin-svelte';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  {
    ignores: ['.svelte-kit/**', 'build/**', 'node_modules/**']
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...svelte.configs.recommended,
  {
    files: ['**/*.{js,ts,svelte}'],
    languageOptions: {
      globals: globals.browser
    }
  },
  {
    files: ['*.config.{js,cjs,ts}', 'eslint.config.js', 'svelte.config.js'],
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node
      }
    }
  },
  {
    files: ['**/*.svelte', '**/*.svelte.js', '**/*.svelte.ts'],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser
      }
    }
  },
  {
    files: ['src/lib/components/chat/Markdown.svelte'],
    rules: {
      'svelte/no-at-html-tags': 'off'
    }
  }
);
