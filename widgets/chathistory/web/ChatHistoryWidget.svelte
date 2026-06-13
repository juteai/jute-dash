<script lang="ts">
  import { Plus } from "lucide-svelte";
  import { chatStore } from "$lib/chatStore";
  import { navigationStore } from "$lib/navigationStore";
  import {
    availabilityLabel,
    availabilityTone,
    getAgentAvailability,
  } from "$lib/agents";
  import {
    WidgetActionButton,
    WidgetBadge,
    WidgetEmptyState,
    WidgetList,
    WidgetListItem,
    WidgetMeta,
    WidgetSectionHeader,
    WidgetStack,
  } from "$lib/components/widget-content";
  import type { Agent, AgentAvailability, Conversation } from "$lib/types";

  export let agents: Agent[] = [];
  export let selectedAgent: Agent | undefined;
  export let selectedAvailability: AgentAvailability = "unknown";

  $: availableAgents = agents.filter(
    (agent) => getAgentAvailability(agent) === "available",
  );
  $: activeAgent =
    selectedAgent ??
    availableAgents[0] ??
    agents.find((agent) => agent.enabled) ??
    agents[0];
  $: activeAvailability = activeAgent
    ? getAgentAvailability(activeAgent)
    : selectedAvailability;

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
    return agent ? agent.name : "Assistant";
  }

  function formatRelativeTime(dateStr: string): string {
    if (!dateStr) return "";
    try {
      const parsed = Date.parse(dateStr);
      if (isNaN(parsed)) return "";
      const now = Date.now();
      const diffMs = now - parsed;
      const diffSec = Math.floor(diffMs / 1000);
      const diffMin = Math.floor(diffSec / 60);
      const diffHour = Math.floor(diffMin / 60);
      const diffDay = Math.floor(diffHour / 24);

      if (diffSec < 60) {
        return "just now";
      } else if (diffMin < 60) {
        return `${diffMin}m ago`;
      } else if (diffHour < 24) {
        return `${diffHour}h ago`;
      } else {
        return `${diffDay}d ago`;
      }
    } catch {
      return "";
    }
  }

  function getConversationStatusDetails(status: string) {
    const s = (status || "").toLowerCase();
    switch (s) {
      case "streaming":
      case "running":
      case "thinking":
        return {
          label: "running",
          tone: "active" as const,
          pulse: true,
        };
      case "completed":
      case "success":
        return {
          label: "completed",
          tone: "success" as const,
          pulse: false,
        };
      case "failed":
      case "error":
        return {
          label: "failed",
          tone: "danger" as const,
          pulse: false,
        };
      case "idle":
      default:
        return {
          label: s || "idle",
          tone: "neutral" as const,
          pulse: false,
        };
    }
  }

  function widgetAvailabilityTone(availability: AgentAvailability) {
    if (availability === "available") {
      return "success";
    }
    const tone = availabilityTone(availability);
    if (tone === "warning" || tone === "danger" || tone === "active") {
      return tone;
    }
    return "neutral";
  }
</script>

<WidgetStack>
  <WidgetSectionHeader title="Saved Chats" count={conversations.length}>
    <WidgetActionButton slot="action" label="New Chat" on:click={handleNewChat}>
      <Plus size={16} />
    </WidgetActionButton>
  </WidgetSectionHeader>

  <WidgetList gap="tight">
    {#if recentConversations.length === 0}
      <WidgetEmptyState
        message="No recent chat yet. Start with a quick household request."
      />
    {:else}
      {#each recentConversations as conv}
        {@const statusDetails = getConversationStatusDetails(conv.status)}
        <WidgetListItem
          direction="column"
          clickable
          on:click={() => handleOpenConversation(conv)}
        >
          <span class="chat-card-title"
            >{conv.title || "Untitled conversation"}</span
          >
          <WidgetMeta>
            <span>{getAgentName(conv.agentId)}</span>
            {#if formatRelativeTime(conv.updatedAt)}
              <span>{formatRelativeTime(conv.updatedAt)}</span>
            {/if}
            <WidgetBadge tone={statusDetails.tone} pulse={statusDetails.pulse}>
              {statusDetails.label}
            </WidgetBadge>
          </WidgetMeta>
        </WidgetListItem>
      {/each}
    {/if}
  </WidgetList>

  <div class="chat-history-footer">
    {#if activeAgent}
      <span class="active-agent-info">
        Active: <span class="active-agent-name">{activeAgent.name}</span>
      </span>
      <WidgetBadge tone={widgetAvailabilityTone(activeAvailability)}>
        {availabilityLabel(activeAvailability)}
      </WidgetBadge>
    {:else}
      <WidgetBadge tone="warning">setup needed</WidgetBadge>
      <span class="offline-text">Add an A2A agent to start.</span>
    {/if}
  </div>
</WidgetStack>

<style>
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

  :global(.widget-list-item--clickable:hover) .chat-card-title {
    color: var(--active);
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
