<script lang="ts">
  import DeviceList from './components/DeviceList.svelte';

  export let data: any = {};
  export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

  $: devices = data?.devices ?? [];
</script>

<div class="widget-content">
  <DeviceList {devices} {dispatch} />
</div>

<style>
  .widget-content {
    padding: 12px;
    height: 100%;
    overflow-y: auto;
  }
</style>
