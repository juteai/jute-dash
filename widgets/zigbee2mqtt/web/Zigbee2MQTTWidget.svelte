<script lang="ts">
  import { DeviceList } from "$lib/components/integration-controls";
  import WidgetStack from "$lib/components/widget-content/WidgetStack.svelte";

  export let data: any = {};
  export let dispatch: (
    action: string,
    args?: any,
  ) => Promise<any> = async () => {};
  export let stale = false;

  $: devices = (data?.devices ?? []).map((device: any) => ({
    id: device.id,
    name: device.name,
    type: device.type,
    active: !!device.state,
    value: device.value,
    toggleable: device.type === "switch" || device.type === "light",
  }));

  function toggleDevice(device: { id: string; active?: boolean }) {
    return dispatch("toggle", { device_id: device.id, state: !device.active });
  }
</script>

<WidgetStack {stale}>
  <DeviceList
    title="Zigbee Devices"
    emptyMessage="No Zigbee devices discovered."
    {devices}
    disabled={stale}
    onToggle={toggleDevice}
  />
</WidgetStack>
