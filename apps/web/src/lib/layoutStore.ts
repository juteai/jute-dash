import { writable } from 'svelte/store';
import {
  addWidget as editorAddWidget,
  addDashboardScreen as editorAddDashboardScreen,
  canAddCatalogWidget,
  duplicateDashboardScreen as editorDuplicateDashboardScreen,
  ensureLayoutScreens,
  layoutForScreen,
  moveWidget as editorMoveWidget,
  removeDashboardScreen as editorRemoveDashboardScreen,
  resizeWidget as editorResizeWidget,
  removeWidget as editorRemoveWidget,
  renameDashboardScreen as editorRenameDashboardScreen,
  reorderDashboardScreen as editorReorderDashboardScreen,
  replaceScreenLayout,
  setActiveScreen as editorSetActiveScreen,
  setVariantGridSize as editorSetVariantGridSize,
  setWidgetMode as editorSetWidgetMode,
  reorderWidget as editorReorderWidget,
  updateWidget as editorUpdateWidget
} from '$lib/layout-editor';
import {
  getWidgetCatalog,
  saveActiveDashboardScreen,
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
      const draft = ensureLayoutScreens(currentLayout);
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
          draftLayout: ensureLayoutScreens(reset),
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
        const screenId = s.draftLayout.activeScreenId ?? '';
        if (!canAddCatalogWidget(s.draftLayout, catalogItem)) {
          return {
            ...s,
            editIssue: `${catalogItem.name} is already added to this dashboard.`
          };
        }
        const res = editorAddWidget(
          layoutForScreen(s.draftLayout, screenId),
          catalogItem,
          mode
        );
        return {
          ...s,
          draftLayout: replaceScreenLayout(s.draftLayout, screenId, res.layout),
          editIssue: res.error || ''
        };
      });
    },
    setWidgetHeadless: (widgetId: string) => {
      update((s) => {
        if (!s.draftLayout) return s;
        const screenId = s.draftLayout.activeScreenId ?? '';
        return {
          ...s,
          draftLayout: replaceScreenLayout(
            s.draftLayout,
            screenId,
            editorSetWidgetMode(
              layoutForScreen(s.draftLayout, screenId),
              widgetId,
              'headless'
            )
          )
        };
      });
    },
    restoreWidget: (widgetId: string) => {
      update((s) => {
        if (!s.draftLayout) return s;
        const screenId = s.draftLayout.activeScreenId ?? '';
        return {
          ...s,
          draftLayout: replaceScreenLayout(
            s.draftLayout,
            screenId,
            editorSetWidgetMode(
              layoutForScreen(s.draftLayout, screenId),
              widgetId,
              'ui'
            )
          )
        };
      });
    },
    reorderWidget: (widgetId: string, direction: -1 | 1) => {
      update((s) => {
        if (!s.draftLayout) return s;
        const screenId = s.draftLayout.activeScreenId ?? '';
        return {
          ...s,
          draftLayout: replaceScreenLayout(
            s.draftLayout,
            screenId,
            editorReorderWidget(
              layoutForScreen(s.draftLayout, screenId),
              widgetId,
              direction,
              s.activeVariantId
            )
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
        const screenId = s.draftLayout.activeScreenId ?? '';
        return {
          ...s,
          draftLayout: replaceScreenLayout(
            s.draftLayout,
            screenId,
            editorSetVariantGridSize(
              layoutForScreen(s.draftLayout, screenId),
              s.activeVariantId,
              columns,
              rows
            )
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
        const screenId = s.draftLayout.activeScreenId ?? '';
        let screenLayout = layoutForScreen(s.draftLayout, screenId);
        screenLayout = editorUpdateWidget(screenLayout, s.configuringWidgetId, {
          title: patch.title,
          settings: patch.settings,
          connectionRefs: patch.connectionRefs
        });
        screenLayout = editorSetWidgetMode(
          screenLayout,
          s.configuringWidgetId,
          patch.mode
        );
        return {
          ...s,
          draftLayout: replaceScreenLayout(
            s.draftLayout,
            screenId,
            screenLayout
          ),
          configuringWidgetId: ''
        };
      });
    },
    moveWidget: (widgetId: string, x: number, y: number) => {
      update((s) => {
        if (!s.draftLayout) return s;
        const screenId = s.draftLayout.activeScreenId ?? '';
        return {
          ...s,
          draftLayout: replaceScreenLayout(
            s.draftLayout,
            screenId,
            editorMoveWidget(
              layoutForScreen(s.draftLayout, screenId),
              widgetId,
              x,
              y,
              s.activeVariantId
            )
          )
        };
      });
    },
    resizeWidget: (widgetId: string, w: number, h: number) => {
      update((s) => {
        if (!s.draftLayout) return s;
        const screenId = s.draftLayout.activeScreenId ?? '';
        return {
          ...s,
          draftLayout: replaceScreenLayout(
            s.draftLayout,
            screenId,
            editorResizeWidget(
              layoutForScreen(s.draftLayout, screenId),
              widgetId,
              w,
              h,
              s.activeVariantId
            )
          )
        };
      });
    },
    removeWidget: (widgetId: string) => {
      update((s) => {
        if (!s.draftLayout) return s;
        const screenId = s.draftLayout.activeScreenId ?? '';
        return {
          ...s,
          draftLayout: replaceScreenLayout(
            s.draftLayout,
            screenId,
            editorRemoveWidget(
              layoutForScreen(s.draftLayout, screenId),
              widgetId
            )
          )
        };
      });
    },
    setActiveScreen: async (
      screenId: string,
      fetcher: typeof fetch = window.fetch,
      onSuccess?: (savedLayout: WidgetLayout) => void
    ) => {
      let editMode = false;
      update((s) => {
        editMode = s.editMode;
        if (s.draftLayout) {
          return {
            ...s,
            draftLayout: editorSetActiveScreen(s.draftLayout, screenId)
          };
        }
        return s;
      });
      if (editMode) return;
      const saved = await saveActiveDashboardScreen(fetcher, screenId);
      onSuccess?.(saved);
    },
    addScreen: () => {
      update((s) =>
        s.draftLayout
          ? { ...s, draftLayout: editorAddDashboardScreen(s.draftLayout) }
          : s
      );
    },
    renameScreen: (screenId: string, label: string) => {
      update((s) =>
        s.draftLayout
          ? {
              ...s,
              draftLayout: editorRenameDashboardScreen(
                s.draftLayout,
                screenId,
                label
              )
            }
          : s
      );
    },
    duplicateScreen: (screenId: string) => {
      update((s) =>
        s.draftLayout
          ? {
              ...s,
              draftLayout: editorDuplicateDashboardScreen(
                s.draftLayout,
                screenId,
                s.widgetCatalog
              )
            }
          : s
      );
    },
    removeScreen: (screenId: string) => {
      update((s) =>
        s.draftLayout
          ? {
              ...s,
              draftLayout: editorRemoveDashboardScreen(s.draftLayout, screenId)
            }
          : s
      );
    },
    reorderScreen: (screenId: string, direction: -1 | 1) => {
      update((s) =>
        s.draftLayout
          ? {
              ...s,
              draftLayout: editorReorderDashboardScreen(
                s.draftLayout,
                screenId,
                direction
              )
            }
          : s
      );
    },
    setEditIssue: (issue: string) => {
      update((s) => ({ ...s, editIssue: issue }));
    }
  };
}

export const layoutStore = createLayoutStore();
