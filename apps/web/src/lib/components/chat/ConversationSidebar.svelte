<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { Plus } from 'lucide-svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import type { Conversation } from '$lib/types';

  export let conversations: Conversation[] = [];
  export let selectedConversationId = '';
  export let composerDisabled = false;

  const dispatch = createEventDispatcher<{
    select: { conversationId: string };
    new: void;
    manageAgents: void;
  }>();

  function formatConversationTime(value: string) {
    if (!value) {
      return '';
    }
    return new Intl.DateTimeFormat(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    }).format(new Date(value));
  }
</script>

<aside class="conversation-sidebar" aria-label="Conversation history">
  <div class="conversation-sidebar-header">
    <div>
      <strong>History</strong>
      <span>{conversations.length} saved</span>
    </div>
    <IconButton
      label="New conversation"
      variant="outline"
      disabled={composerDisabled}
      on:click={() => dispatch('new')}
    >
      <Plus size={17} />
    </IconButton>
  </div>

  <div class="conversation-list">
    {#if conversations.length === 0}
      <div class="conversation-empty">
        {#if composerDisabled}
          No available agent yet.
          <button
            type="button"
            class="conversation-link-button"
            on:click={() => dispatch('manageAgents')}>Add agent</button
          >
        {:else}
          Agent-backed history is empty or unsupported.
        {/if}
      </div>
    {:else}
      {#each conversations as conversation (conversation.id)}
        <button
          type="button"
          class:conversation-item--active={conversation.id ===
            selectedConversationId}
          class="conversation-item"
          on:click={() =>
            dispatch('select', { conversationId: conversation.id })}
        >
          <span>{conversation.title || 'Conversation'}</span>
          <small>{formatConversationTime(conversation.updatedAt)}</small>
        </button>
      {/each}
    {/if}
  </div>
</aside>

<style>
  .conversation-sidebar {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    min-height: 0;
    border-right: 1px solid var(--border);
    background: var(--surface-muted);
  }

  .conversation-sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 12px;
    border-bottom: 1px solid var(--border);
  }

  .conversation-sidebar-header strong {
    display: block;
    color: var(--foreground);
    font-size: 0.92rem;
  }

  .conversation-sidebar-header span {
    display: block;
    margin-top: 2px;
    color: var(--muted);
    font-size: 0.76rem;
    font-weight: 650;
  }

  .conversation-list {
    display: grid;
    align-content: start;
    gap: 6px;
    min-height: 0;
    overflow-y: auto;
    padding: 10px;
    scrollbar-gutter: stable;
  }

  .conversation-empty {
    padding: 12px;
    color: var(--muted);
    font-size: 0.84rem;
    line-height: 1.35;
  }

  .conversation-link-button {
    display: block;
    margin-top: 10px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 8px 10px;
    cursor: pointer;
    font-weight: 720;
    width: 100%;
    text-align: center;
  }

  .conversation-item {
    display: grid;
    gap: 4px;
    width: 100%;
    border: 1px solid transparent;
    border-radius: 8px;
    background: transparent;
    color: var(--foreground);
    padding: 10px;
    text-align: left;
    cursor: pointer;
    font: inherit;
  }

  .conversation-item:hover,
  .conversation-item--active {
    border-color: var(--border);
    background: var(--surface);
  }

  .conversation-item span {
    overflow: hidden;
    font-size: 0.88rem;
    font-weight: 720;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .conversation-item small {
    color: var(--muted);
    font-size: 0.74rem;
    font-weight: 650;
  }

  @media (max-width: 640px) {
    .conversation-sidebar {
      max-height: 160px;
      border-right: 0;
      border-bottom: 1px solid var(--border);
    }

    .conversation-list {
      grid-auto-flow: column;
      grid-auto-columns: minmax(190px, 1fr);
      overflow-x: auto;
      overflow-y: hidden;
    }
  }
</style>
