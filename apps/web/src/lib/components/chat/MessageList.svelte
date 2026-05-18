<script lang="ts">
  import Markdown from '$lib/components/chat/Markdown.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import type { ChatMessage } from '$lib/types';

  export let messages: ChatMessage[] = [];
  export let emptyTitle = 'Ask Jute anything';
  export let emptyMessage = 'Choose an agent and start with a short request.';
  export let onRetry: (message: ChatMessage) => Promise<void> | void = () => {};
</script>

<div class="message-list" aria-live="polite">
  {#if messages.length === 0}
    <div class="chat-empty">
      <div class="chat-empty-title">{emptyTitle}</div>
      <p>{emptyMessage}</p>
    </div>
  {:else}
    {#each messages as message}
      <article class={`message-bubble message-bubble--${message.role} ${message.status ? `message-bubble--${message.status}` : ''}`}>
        <div class="message-role">
          <span>{message.role}</span>
          {#if message.status}
            <span>{message.status}</span>
          {/if}
        </div>
        <Markdown content={message.content} />
        {#if message.status === 'failed'}
          <div class="message-actions">
            <Button size="sm" variant="outline" on:click={() => onRetry(message)}>Retry</Button>
          </div>
        {/if}
      </article>
    {/each}
  {/if}
</div>
