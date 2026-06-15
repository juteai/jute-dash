import { sveltekit } from '@sveltejs/kit/vite';
import basicSsl from '@vitejs/plugin-basic-ssl';
import { defineConfig } from 'vitest/config';

export default defineConfig(({ mode }) => ({
  plugins: [mode === 'https' ? basicSsl() : undefined, sveltekit()],
  test: {
    include: ['src/**/*.test.ts'],
    exclude: ['e2e/**', 'node_modules/**', '.svelte-kit/**', 'coverage/**'],
    coverage: {
      provider: 'v8',
      include: [
        'src/lib/a2aConversation.ts',
        'src/lib/a2aParser.ts',
        'src/lib/agents.ts',
        'src/lib/chatStore.ts',
        'src/lib/displaySanitizer.ts',
        'src/lib/layout-editor.ts',
        'src/lib/logger.ts',
        'src/lib/messageQueue.ts',
        'src/lib/navigationStore.ts',
        'src/lib/utils.ts'
      ],
      thresholds: {
        statements: 60,
        branches: 45,
        functions: 55,
        lines: 60
      }
    }
  },
  server: {
    proxy: {
      '/api/v1': 'http://127.0.0.1:8787',
      '/healthz': 'http://127.0.0.1:8787'
    },
    fs: {
      allow: ['../../widgets', '../../themes', '.']
    }
  }
}));
