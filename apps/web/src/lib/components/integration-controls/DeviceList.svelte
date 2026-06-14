<script lang="ts" context="module">
  export type DisplayDevice = {
    id: string;
    name: string;
    type?: string;
    active?: boolean;
    value?: string | number | boolean;
    toggleable?: boolean;
  };
</script>

<script lang="ts">
  import { Lightbulb } from 'lucide-svelte';
  import WidgetEmptyState from '$lib/components/widget-content/WidgetEmptyState.svelte';
  import WidgetList from '$lib/components/widget-content/WidgetList.svelte';
  import WidgetListItem from '$lib/components/widget-content/WidgetListItem.svelte';
  import WidgetSectionHeader from '$lib/components/widget-content/WidgetSectionHeader.svelte';
  import DeviceRow from './DeviceRow.svelte';

  export let title: string;
  export let emptyMessage: string;
  export let devices: DisplayDevice[] = [];
  export let disabled = false;
  export let onToggle: (
    device: DisplayDevice
  ) => Promise<void> | void = () => {};
</script>

<WidgetSectionHeader {title} count={devices.length}>
  <Lightbulb slot="icon" size={14} />
</WidgetSectionHeader>

{#if devices.length === 0}
  <WidgetEmptyState message={emptyMessage}>
    <Lightbulb slot="icon" size={32} />
  </WidgetEmptyState>
{:else}
  <WidgetList gap="tight">
    {#each devices as device (device.id)}
      <WidgetListItem>
        <DeviceRow
          name={device.name}
          type={device.type ?? ''}
          active={!!device.active}
          value={device.value}
          {disabled}
          onToggle={device.toggleable ? () => onToggle(device) : undefined}
        />
      </WidgetListItem>
    {/each}
  </WidgetList>
{/if}
