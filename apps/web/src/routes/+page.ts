import { fallbackDashboard, getDashboard } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  try {
    return await getDashboard(fetch);
  } catch {
    return fallbackDashboard();
  }
};
