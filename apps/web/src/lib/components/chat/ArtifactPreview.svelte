<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { X } from 'lucide-svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import Markdown from '$lib/components/chat/Markdown.svelte';

  export let artifact: { title: string; content: string };

  const dispatch = createEventDispatcher<{
    close: void;
  }>();
</script>

<aside class="artifact-preview-panel" aria-label="Artifact preview">
  <header class="artifact-preview-header">
    <span class="artifact-preview-title">{artifact.title}</span>
    <IconButton
      label="Close preview"
      variant="ghost"
      on:click={() => dispatch('close')}
    >
      <X size={18} />
    </IconButton>
  </header>
  <div class="artifact-preview-body">
    <Markdown content={artifact.content} />
  </div>
</aside>

<style>
  .artifact-preview-panel {
    border-left: 1px solid var(--border);
    background: var(--surface);
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
    animation: slide-in-right 0.25s ease-out;
  }

  .artifact-preview-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px;
    border-bottom: 1px solid var(--border);
    background: var(--surface-muted);
  }

  .artifact-preview-title {
    font-weight: 760;
    font-size: 0.94rem;
    color: var(--foreground);
  }

  .artifact-preview-body {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    font-size: 0.9rem;
    line-height: 1.5;
    color: var(--foreground);
  }

  @keyframes slide-in-right {
    from {
      transform: translateX(100%);
    }
    to {
      transform: translateX(0);
    }
  }
</style>
