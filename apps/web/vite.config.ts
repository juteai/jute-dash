import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [sveltekit()],
  test: {
    coverage: {
      provider: 'v8',
      include: [
        'src/lib/agents.ts',
        'src/lib/api.ts',
        'src/lib/layout-editor.ts'
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
    fs: {
      allow: ['../../widgets', '.']
    }
  }
});
