import { writable } from 'svelte/store';
import {
  addWidget as editorAddWidget,
  ensureLayoutVariants,
  moveWidget as editorMoveWidget,
  resizeWidget as editorResizeWidget,
  removeWidget as editorRemoveWidget,
  setVariantGridSize as editorSetVariantGridSize,
  setWidgetMode as editorSetWidgetMode,
  reorderWidget as editorReorderWidget,
  updateWidget as editorUpdateWidget
} from '$lib/layout-editor';
import {
  getWidgetCatalog,
  saveWidgetLayout,
  resetWidgetLayout
} from '$lib/hubClient';
import type { WidgetLayout, WidgetCatalogItem } from '$lib/types';

export interface LayoutState {
  editMode: boolean;
  draftLayout: WidgetLayout | undefined;
  configuringWidgetId: string;
  activeVariantId: string;
  widgetCatalog: WidgetCatalogItem[];
  editIssue: string;
  saving: boolean;
}

const initialState: LayoutState = {
  editMode: false,
  draftLayout: undefined,
  configuringWidgetId: '',
  activeVariantId: '',
  widgetCatalog: [],
  editIssue: '',
  saving: false
};

function createLayoutStore() {
  const { subscribe, update } = writable<LayoutState>(initialState);

  return {
    subscribe,
    initCatalog: async (fetcher: typeof fetch = window.fetch) => {
      try {
        const catalog = await getWidgetCatalog(fetcher);
        update((s) => ({ ...s, widgetCatalog: catalog, editIssue: '' }));
      } catch (err) {
        update((s) => ({
          ...s,
          editIssue:
            'Widget catalog is unavailable. Existing widgets can still be adjusted.'
        }));
        throw err;
      }
    },
    enterEdit: (currentLayout: WidgetLayout, activeVariantId = '') => {
      const draft = ensureLayoutVariants(currentLayout);
      update((s) => ({
        ...s,
        editMode: true,
        draftLayout: draft,
        activeVariantId,
        editIssue: '',
        configuringWidgetId: ''
      }));
    },
    cancelEdit: () => {
      update((s) => ({
        ...s,
        editMode: false,
        draftLayout: undefined,
        activeVariantId: '',
        editIssue: '',
        configuringWidgetId: ''
      }));
    },
    saveEdit: async (
      stale: boolean,
      fetcher: typeof fetch = window.fetch,
      onSuccess: (savedLayout: WidgetLayout) => void
    ) => {
      let layoutToSave: WidgetLayout | undefined;
      update((s) => {
        if (s.saving || stale) return s;
        layoutToSave = s.draftLayout;
        return { ...s, saving: true, editIssue: '' };
      });

      if (!layoutToSave) return;

      try {
        const saved = await saveWidgetLayout(fetcher, layoutToSave);
        update((s) => ({
          ...s,
          editMode: false,
          draftLayout: undefined,
          configuringWidgetId: '',
          activeVariantId: '',
          saving: false
        }));
        onSuccess(saved);
      } catch (err) {
        update((s) => ({
          ...s,
          saving: false,
          editIssue:
            'Layout was not saved. Check that the hub is running, then try again.'
        }));
        throw err;
      }
    },
    resetLayout: async (
      profileId: string,
      fetcher: typeof fetch = window.fetch,
      onSuccess: (resetLayout: WidgetLayout) => void
    ) => {
      update((s) => ({ ...s, saving: true, editIssue: '' }));
      try {
        const reset = await resetWidgetLayout(fetcher, profileId);
        update((s) => ({
          ...s,
          draftLayout: ensureLayoutVariants(reset),
          saving: false
        }));
        onSuccess(reset);
      } catch (err) {
        update((s) => ({
          ...s,
          saving: false,
          editIssue: 'Default layout could not be restored.'
        }));
        throw err;
      }
    },
    addWidget: (
      catalogItem: WidgetCatalogItem,
      mode: 'ui' | 'headless' = 'ui'
    ) => {
      update((s) => {
        if (!s.draftLayout) return s;
        const res = editorAddWidget(s.draftLayout, catalogItem, mode);
        return {
          ...s,
          draftLayout: res.layout,
          editIssue: res.error || ''
        };
      });
    },
    setWidgetHeadless: (widgetId: string) => {
      update((s) => {
        if (!s.draftLayout) return s;
        return {
          ...s,
          draftLayout: editorSetWidgetMode(s.draftLayout, widgetId, 'headless')
        };
      });
    },
    restoreWidget: (widgetId: string) => {
      update((s) => {
        if (!s.draftLayout) return s;
        return {
          ...s,
          draftLayout: editorSetWidgetMode(s.draftLayout, widgetId, 'ui')
        };
      });
    },
    reorderWidget: (widgetId: string, direction: -1 | 1) => {
      update((s) => {
        if (!s.draftLayout) return s;
        return {
          ...s,
          draftLayout: editorReorderWidget(
            s.draftLayout,
            widgetId,
            direction,
            s.activeVariantId
          )
        };
      });
    },
    setActiveVariant: (variantId: string) => {
      update((s) => ({ ...s, activeVariantId: variantId }));
    },
    setVariantGridSize: (columns: number, rows: number) => {
      update((s) => {
        if (!s.draftLayout) return s;
        return {
          ...s,
          draftLayout: editorSetVariantGridSize(
            s.draftLayout,
            s.activeVariantId,
            columns,
            rows
          )
        };
      });
    },
    openWidgetConfig: (widgetId: string) => {
      update((s) => ({ ...s, configuringWidgetId: widgetId }));
    },
    closeWidgetConfig: () => {
      update((s) => ({ ...s, configuringWidgetId: '' }));
    },
    saveWidgetConfig: (patch: {
      title: string;
      settings: Record<string, unknown>;
      connectionRefs: Record<string, string>;
      mode: 'ui' | 'headless';
    }) => {
      update((s) => {
        if (!s.draftLayout || !s.configuringWidgetId) return s;
        let next = editorUpdateWidget(s.draftLayout, s.configuringWidgetId, {
          title: patch.title,
          settings: patch.settings,
          connectionRefs: patch.connectionRefs
        });
        next = editorSetWidgetMode(next, s.configuringWidgetId, patch.mode);
        return {
          ...s,
          draftLayout: next,
          configuringWidgetId: ''
        };
      });
    },
    moveWidget: (widgetId: string, x: number, y: number) => {
      update((s) => {
        if (!s.draftLayout) return s;
        return {
          ...s,
          draftLayout: editorMoveWidget(
            s.draftLayout,
            widgetId,
            x,
            y,
            s.activeVariantId
          )
        };
      });
    },
    resizeWidget: (widgetId: string, w: number, h: number) => {
      update((s) => {
        if (!s.draftLayout) return s;
        return {
          ...s,
          draftLayout: editorResizeWidget(
            s.draftLayout,
            widgetId,
            w,
            h,
            s.activeVariantId
          )
        };
      });
    },
    removeWidget: (widgetId: string) => {
      update((s) => {
        if (!s.draftLayout) return s;
        return {
          ...s,
          draftLayout: editorRemoveWidget(s.draftLayout, widgetId)
        };
      });
    },
    setEditIssue: (issue: string) => {
      update((s) => ({ ...s, editIssue: issue }));
    }
  };
}

export const layoutStore = createLayoutStore();
