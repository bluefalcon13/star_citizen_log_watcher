package processor

import (
	"context"

	"StarCitizenLogWatcher/internal/state"
	"StarCitizenLogWatcher/pkg/filter"
	"StarCitizenLogWatcher/pkg/models"
)

// Processor acts as the central event loop brain, evaluating raw log lines against the Filter Registry.
type Processor struct {
	registry *filter.Registry
	stateMgr *state.Manager
}

// NewProcessor constructs a Processor with its injected filter registry and state manager.
func NewProcessor(reg *filter.Registry, stateMgr *state.Manager) *Processor {
	return &Processor{
		registry: reg,
		stateMgr: stateMgr,
	}
}

// Run processes raw lines from inCh, evaluates them through the filter registry,
// records state changes, and forwards matched events to outCh. When inCh closes or
// ctx is cancelled, outCh is closed.
func (p *Processor) Run(ctx context.Context, inCh <-chan models.RawLogLine, outCh chan<- models.Event) {
	defer close(outCh)

	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-inCh:
			if !ok {
				return
			}

			if p.stateMgr != nil {
				p.stateMgr.IncrementLineCount()
			}

			event, matched := p.registry.Evaluate(line.Content)
			if !matched || event == nil {
				continue
			}

			if p.stateMgr != nil {
				enriched, keep := p.stateMgr.RecordAndEnrich(*event)
				if !keep {
					continue
				}
				event = &enriched
			}

			select {
			case <-ctx.Done():
				return
			case outCh <- *event:
			}
		}
	}
}

