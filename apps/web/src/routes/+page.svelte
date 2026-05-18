<script lang="ts">
  import DisplayShell from '$lib/components/display/DisplayShell.svelte';
  import type { DashboardData } from '$lib/types';

  export let data: DashboardData;
</script>

<DisplayShell {data} />
