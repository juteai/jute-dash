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
