<script lang="ts">
  import { MessageCircle, Mic, Pencil, Settings, X } from 'lucide-svelte';
  import DashboardGrid from '$lib/components/display/DashboardGrid.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import type { Agent, AgentAvailability, ChatMessage, DashboardData } from '$lib/types';

  export let data: DashboardData;
  export let editMode = false;
  export let messages: ChatMessage[] = [];
  export let theme: 'light' | 'dark' = 'light';
  export let stale = false;
  export let selectedAgent: Agent | undefined;
  export let selectedAvailability: AgentAvailability = 'unknown';
  export let onOpenChat: () => void = () => {};
  export let onEnterEdit: () => void = () => {};
  export let onExitEdit: () => void = () => {};
</script>

<section class="dashboard-view" aria-label="Jute dashboard">
  <header class="dashboard-header">
    <div class="brand-lockup">
      <img
        src={theme === 'dark' ? '/brand/logo_light.svg' : '/brand/logo_dark.svg'}
        alt="Jute"
        class="brand-logo"
      />
      <div>
        <div class="home-name">{data.config.home.name}</div>
        <div class="layout-name">{data.layout.profileId}</div>
      </div>
    </div>

    <div class="dashboard-actions">
      {#if editMode}
        <Button variant="outline" on:click={onExitEdit}>
          <X size={17} />
          <span>Done</span>
        </Button>
      {:else}
        <IconButton label="Open chat" variant="outline" on:click={onOpenChat}>
          <MessageCircle size={20} />
        </IconButton>
        <IconButton label="Voice" variant="outline">
          <Mic size={20} />
        </IconButton>
        <IconButton label="Edit dashboard" variant="outline" on:click={onEnterEdit}>
          <Pencil size={20} />
        </IconButton>
        <IconButton label="Settings" variant="outline">
          <Settings size={20} />
        </IconButton>
      {/if}
    </div>
  </header>

  {#if editMode}
    <div class="edit-toolbar" role="status">
      <div>
        <strong>Edit dashboard</strong>
        <span>Move, resize, add, and configure controls land in the next layout slice.</span>
      </div>
      <Button size="sm" variant="secondary">Add widget</Button>
    </div>
  {/if}

  <DashboardGrid
    {data}
    {editMode}
    {messages}
    {stale}
    {selectedAgent}
    {selectedAvailability}
    {onOpenChat}
  />
</section>
