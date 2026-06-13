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
    type: "light",
    active: !!device.state,
    toggleable: true,
  }));

  function toggleDevice(device: { id: string; active?: boolean }) {
    return dispatch("toggle", { device_id: device.id, state: !device.active });
  }
</script>

<WidgetStack {stale}>
  <DeviceList
    title="Philips Hue Lights"
    emptyMessage="No devices discovered yet."
    {devices}
    disabled={stale}
    onToggle={toggleDevice}
  />
</WidgetStack>
