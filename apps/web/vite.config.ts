import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [sveltekit()],
  test: {
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
    fs: {
      allow: ['../../widgets', '.']
    }
  }
});
