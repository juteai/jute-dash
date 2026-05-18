<script lang="ts">
  import { browser } from '$app/environment';
  import { marked } from 'marked';

  export let content = '';

  let html = '';
  let renderToken = 0;

  $: renderMarkdown(content);

  async function renderMarkdown(markdown: string) {
    const token = ++renderToken;
    const raw = String(marked.parse(markdown || '', { async: false }));
    if (!browser) {
      html = escapeHTML(markdown || '');
      return;
    }

    const { default: DOMPurify } = await import('dompurify');
    if (token === renderToken) {
      html = DOMPurify.sanitize(raw, {
        USE_PROFILES: { html: true },
        ADD_ATTR: ['target', 'rel']
      });
    }
  }

  function escapeHTML(value: string) {
    return value
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#039;');
  }
</script>

<div class="markdown-content">
  {@html html}
</div>
