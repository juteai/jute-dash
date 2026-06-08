<script lang="ts">
  import { Plus } from 'lucide-svelte';
  import { chatStore } from '$lib/chatStore';
  import { navigationStore } from '$lib/navigationStore';
  import { availabilityLabel, availabilityTone, getAgentAvailability } from '$lib/agents';
  import Badge from '$lib/components/ui/Badge.svelte';
  import type { Agent, AgentAvailability, ChatMessage, Conversation } from '$lib/types';

  export let agents: Agent[] = [];
  export let selectedAgent: Agent | undefined;
  export let selectedAvailability: AgentAvailability = 'unknown';

  $: availableAgents = agents.filter((agent) => getAgentAvailability(agent) === 'available');
  $: activeAgent = selectedAgent ?? availableAgents[0] ?? agents.find((agent) => agent.enabled) ?? agents[0];
  $: activeAvailability = activeAgent ? getAgentAvailability(activeAgent) : selectedAvailability;

  $: conversations = $chatStore.conversations;
  $: recentConversations = conversations.slice(0, 5);

  async function handleOpenConversation(conv: Conversation) {
    navigationStore.openChat();
    await chatStore.loadConversation(conv.id, conv.agentId, fetch);
  }

  async function handleNewChat() {
    navigationStore.openChat();
    await chatStore.newConversation(agents, fetch);
  }

  function getAgentName(agentId: string): string {
    const agent = agents.find((a) => a.id === agentId);
    return agent ? agent.name : 'Assistant';
  }

  function formatRelativeTime(dateStr: string): string {
    if (!dateStr) return '';
    try {
      const parsed = Date.parse(dateStr);
      if (isNaN(parsed)) return '';
      const now = Date.now();
      const diffMs = now - parsed;
      const diffSec = Math.floor(diffMs / 1000);
      const diffMin = Math.floor(diffSec / 60);
      const diffHour = Math.floor(diffMin / 60);
      const diffDay = Math.floor(diffHour / 24);

      if (diffSec < 60) {
        return 'just now';
      } else if (diffMin < 60) {
        return `${diffMin}m ago`;
      } else if (diffHour < 24) {
        return `${diffHour}h ago`;
      } else {
        return `${diffDay}d ago`;
      }
    } catch {
      return '';
    }
  }

  function getConversationStatusDetails(status: string) {
    const s = (status || '').toLowerCase();
    switch (s) {
      case 'streaming':
      case 'running':
      case 'thinking':
        return {
          label: 'running',
          color: 'var(--active)',
          bg: 'color-mix(in srgb, var(--active) 12%, transparent)',
          pulse: true,
          icon: '●'
        };
      case 'completed':
      case 'success':
        return {
          label: 'completed',
          color: 'var(--success)',
          bg: 'color-mix(in srgb, var(--success) 12%, transparent)',
          pulse: false,
          icon: '✓'
        };
      case 'failed':
      case 'error':
        return {
          label: 'failed',
          color: 'var(--danger)',
          bg: 'color-mix(in srgb, var(--danger) 12%, transparent)',
          pulse: false,
          icon: '⚠'
        };
      case 'idle':
      default:
        return {
          label: s || 'idle',
          color: 'var(--muted)',
          bg: 'color-mix(in srgb, var(--muted) 12%, transparent)',
          pulse: false,
          icon: '○'
        };
    }
  }
</script>

<div class="chat-history-widget">
  <div class="chat-history-header">
    <div class="chat-history-title-row">
      <h3 class="widget-title">Saved Chats</h3>
      <span class="chats-count-badge">{conversations.length}</span>
    </div>
    <button
      type="button"
      class="new-chat-icon-btn"
      on:click={handleNewChat}
      title="New Chat"
      aria-label="New Chat"
    >
      <Plus size={16} />
    </button>
  </div>

  <div class="chat-history-list">
    {#if recentConversations.length === 0}
      <p class="empty-text">No recent chat yet. Start with a quick household request.</p>
    {:else}
      {#each recentConversations as conv}
        {@const statusDetails = getConversationStatusDetails(conv.status)}
        <button
          type="button"
          class="chat-card"
          on:click={() => handleOpenConversation(conv)}
        >
          <span class="chat-card-title">{conv.title || 'Untitled conversation'}</span>
          <span class="chat-card-meta">
            <span class="chat-card-agent">{getAgentName(conv.agentId)}</span>
            {#if formatRelativeTime(conv.updatedAt)}
              <span class="meta-separator">&middot;</span>
              <span class="chat-card-time">{formatRelativeTime(conv.updatedAt)}</span>
            {/if}
            <span class="meta-separator">&middot;</span>
            <span
              class="chat-card-status"
              style="--status-color: {statusDetails.color}; --status-bg: {statusDetails.bg};"
            >
              {#if statusDetails.pulse}
                <span class="status-pulse-dot"></span>
              {:else}
                <span class="status-icon">{statusDetails.icon}</span>
              {/if}
              <span class="status-label">{statusDetails.label}</span>
            </span>
          </span>
        </button>
      {/each}
    {/if}
  </div>

  <div class="chat-history-footer">
    {#if activeAgent}
      <span class="active-agent-info">
        Active: <span class="active-agent-name">{activeAgent.name}</span>
      </span>
      <Badge tone={availabilityTone(activeAvailability)}>{availabilityLabel(activeAvailability)}</Badge>
    {:else}
      <Badge tone="warning">setup needed</Badge>
      <span class="offline-text">Add an A2A agent to start.</span>
    {/if}
  </div>
</div>

<style>
  .chat-history-widget {
    display: flex;
    flex-direction: column;
    height: 100%;
    width: 100%;
    overflow: hidden;
    gap: clamp(8px, 2.5cqmin, 12px);
  }

  .chat-history-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding-bottom: 2px;
  }

  .chat-history-title-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .widget-title {
    font-size: var(--widget-label-size, 0.75rem);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
    margin: 0;
  }

  .chats-count-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 0.65rem;
    font-weight: 700;
    background: var(--surface-strong);
    color: var(--foreground);
    padding: 1px 6px;
    border-radius: 999px;
    border: 1px solid var(--border);
    line-height: 1;
  }

  .new-chat-icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: 6px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--muted);
    cursor: pointer;
    transition: all 0.2s ease;
    padding: 0;
  }

  .new-chat-icon-btn:hover {
    background: var(--surface-strong);
    color: var(--active);
    border-color: var(--border-strong);
  }

  .new-chat-icon-btn:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 1px;
  }

  .empty-text {
    color: var(--muted);
    font-size: var(--widget-body-size);
  }

  .chat-history-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 2cqmin, 10px);
    padding-right: 4px;
    user-select: none;
  }

  .chat-card {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    padding: clamp(8px, 3cqmin, 12px);
    border-radius: 8px;
    background: var(--surface-muted);
    border: 1px solid var(--border);
    transition: all 0.2s ease;
    text-align: left;
    width: 100%;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .chat-card:hover {
    transform: scale(1.01);
    border-color: var(--border-strong);
    background: var(--surface-strong);
  }

  .chat-card:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: -1px;
  }

  .chat-card-title {
    font-size: var(--widget-body-size);
    font-weight: 500;
    line-height: 1.4;
    color: var(--foreground);
    display: -webkit-box;
    -webkit-box-orient: vertical;
    line-clamp: 2;
    -webkit-line-clamp: 2;
    overflow: hidden;
    width: 100%;
  }

  .chat-card:hover .chat-card-title {
    color: var(--active);
  }

  .chat-card-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: clamp(0.6rem, 4cqmin, 0.75rem);
    color: var(--muted);
  }

  .meta-separator {
    opacity: 0.5;
  }

  .chat-card-status {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 1px 5px;
    border-radius: 3px;
    background: var(--status-bg);
    color: var(--status-color);
    font-size: 0.6rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    line-height: 1;
  }

  .status-pulse-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background-color: currentColor;
    animation: status-pulse 1.8s infinite ease-in-out;
  }

  @keyframes status-pulse {
    0% {
      opacity: 0.5;
      transform: scale(0.85);
    }
    50% {
      opacity: 1;
      transform: scale(1.15);
    }
    100% {
      opacity: 0.5;
      transform: scale(0.85);
    }
  }

  .status-icon {
    font-size: 0.6rem;
    font-weight: bold;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }

  .status-label {
    line-height: 1;
  }

  .chat-history-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding-top: clamp(6px, 2cqmin, 10px);
    border-top: 1px dashed var(--border);
    color: var(--muted);
    font-size: clamp(0.65rem, 4cqmin, 0.75rem);
    min-height: 28px;
  }

  .active-agent-info {
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .active-agent-name {
    color: var(--foreground);
    font-weight: 600;
  }

  .offline-text {
    font-size: 0.7rem;
    color: var(--danger);
  }
</style>
