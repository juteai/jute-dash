import { expect, it } from 'vitest';
import { autoLinkWidgetConnections } from './connectionAutoLink';
import type {
  AdapterConnection,
  WidgetCatalogItem,
  WidgetLayout
} from './types';

const catalog: WidgetCatalogItem[] = [
  {
    kind: 'spotify',
    name: 'Spotify',
    description: '',
    defaultTitle: 'Spotify',
    defaultW: 6,
    defaultH: 2,
    minW: 4,
    minH: 2,
    defaultSize: 'wide',
    overflow: 'clip',
    allowMultiple: false,
    connectionRequirements: [
      {
        slot: 'account',
        kind: 'spotify',
        displayName: 'Spotify Account',
        required: true
      }
    ]
  }
];

const layout: WidgetLayout = {
  profileId: 'default-dashboard',
  widgets: [
    {
      id: 'spotify',
      kind: 'spotify',
      title: 'Spotify',
      x: 0,
      y: 0,
      w: 6,
      h: 2,
      minW: 4,
      minH: 2,
      size: 'wide',
      settings: {},
      visible: true
    }
  ]
};

it('links a missing widget connection ref when exactly one matching connection exists', () => {
  const connections: AdapterConnection[] = [
    {
      id: 'Spotify',
      kind: 'spotify',
      name: 'Jute',
      settings: {},
      enabled: true
    }
  ];

  const result = autoLinkWidgetConnections(layout, catalog, connections);

  expect(result.changed).toBe(true);
  expect(result.layout.widgets[0].connectionRefs).toEqual({
    account: 'Spotify'
  });
  expect(layout.widgets[0].connectionRefs).toBeUndefined();
});

it('does not link when multiple matching connections exist', () => {
  const connections: AdapterConnection[] = [
    {
      id: 'Spotify',
      kind: 'spotify',
      name: 'Jute',
      settings: {},
      enabled: true
    },
    {
      id: 'other',
      kind: 'spotify',
      name: 'Other',
      settings: {},
      enabled: true
    }
  ];

  const result = autoLinkWidgetConnections(layout, catalog, connections);

  expect(result.changed).toBe(false);
  expect(result.layout.widgets[0].connectionRefs).toBeUndefined();
});
