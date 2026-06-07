package filesync

import (
	"context"

	"github.com/riverqueue/river"
)

type ConfigSyncArgs struct{}

func (ConfigSyncArgs) Kind() string { return "config_sync" }

func (ConfigSyncArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	}
}

type ConfigSyncWorker struct {
	river.WorkerDefaults[ConfigSyncArgs]

	syncer Syncer
}

func NewConfigSyncWorker(syncer Syncer) *ConfigSyncWorker {
	return &ConfigSyncWorker{
		syncer: syncer,
	}
}

func (w *ConfigSyncWorker) Work(ctx context.Context, _ *river.Job[ConfigSyncArgs]) error {
	return w.syncer.Sync(ctx)
}
