import { initialDashboard } from '$lib/hubClient';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
  return initialDashboard();
};
