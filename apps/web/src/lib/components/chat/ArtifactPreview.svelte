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
