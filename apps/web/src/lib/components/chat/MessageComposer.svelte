<script lang="ts">
  import { Mic, Send, X } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import Textarea from '$lib/components/ui/Textarea.svelte';
  import type { ChatState } from '$lib/types';

  export let state: ChatState = 'idle';
  export let disabled = false;
  export let onSubmit: (value: string) => Promise<void> | void = () => {};
  export let onCancel: () => void = () => {};

  let value = '';

  $: canSend = value.trim().length > 0 && !disabled && state !== 'thinking' && state !== 'streaming';

  async function submit() {
    const text = value.trim();
    if (!text || !canSend) {
      return;
    }
    value = '';
    await onSubmit(text);
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      void submit();
    }
  }
</script>

<form class="message-composer" on:submit|preventDefault={submit}>
  <IconButton label="Voice input" variant="outline" disabled={state === 'thinking'}>
    <Mic size={19} />
  </IconButton>
  <Textarea
    bind:value
    rows={1}
    placeholder="Ask your home assistant"
    disabled={disabled}
    on:keydown={handleKeydown}
  />
  {#if state === 'thinking' || state === 'streaming'}
    <IconButton label="Cancel response" variant="outline" on:click={onCancel}>
      <X size={19} />
    </IconButton>
  {:else}
    <Button type="submit" size="md" disabled={!canSend}>
      <Send size={18} />
      <span>Send</span>
    </Button>
  {/if}
</form>
