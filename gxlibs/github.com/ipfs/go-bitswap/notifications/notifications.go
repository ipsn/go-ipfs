package notifications

import (
	"context"
	"sync"

	pubsub "github.com/cskr/pubsub"
	blocks "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-block-format"
	cid "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
)

const bufferSize = 16

type PubSub interface {
	Publish(block blocks.Block)
	Subscribe(ctx context.Context, keys ...cid.Cid) <-chan blocks.Block
	Shutdown()
}

func New() PubSub {
	return &impl{
		wrapped: *pubsub.New(bufferSize),
		closed:  make(chan struct{}),
	}
}

type impl struct {
	lk      sync.RWMutex
	wrapped pubsub.PubSub

	closed chan struct{}
}

func (ps *impl) Publish(block blocks.Block) {
	ps.lk.RLock()
	defer ps.lk.RUnlock()
	select {
	case <-ps.closed:
		return
	default:
	}

	ps.wrapped.Pub(block, block.Cid().KeyString())
}

func (ps *impl) Shutdown() {
	ps.lk.Lock()
	defer ps.lk.Unlock()
	select {
	case <-ps.closed:
		return
	default:
	}
	close(ps.closed)
	ps.wrapped.Shutdown()
}

// Subscribe returns a channel of blocks for the given |keys|. |blockChannel|
// is closed if the |ctx| times out or is cancelled, or after sending len(keys)
// blocks.
func (ps *impl) Subscribe(ctx context.Context, keys ...cid.Cid) <-chan blocks.Block {

	blocksCh := make(chan blocks.Block, len(keys))
	valuesCh := make(chan interface{}, len(keys)) // provide our own channel to control buffer, prevent blocking
	if len(keys) == 0 {
		close(blocksCh)
		return blocksCh
	}

	// prevent shutdown
	ps.lk.RLock()
	defer ps.lk.RUnlock()

	select {
	case <-ps.closed:
		close(blocksCh)
		return blocksCh
	default:
	}

	ps.wrapped.AddSubOnceEach(valuesCh, toStrings(keys)...)
	go func() {
		defer func() {
			close(blocksCh)

			ps.lk.RLock()
			defer ps.lk.RUnlock()
			// Don't touch the pubsub instance if we're
			// already closed.
			select {
			case <-ps.closed:
				return
			default:
			}

			ps.wrapped.Unsub(valuesCh)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ps.closed:
			case val, ok := <-valuesCh:
				if !ok {
					return
				}
				block, ok := val.(blocks.Block)
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case blocksCh <- block: // continue
				case <-ps.closed:
				}
			}
		}
	}()

	return blocksCh
}

func toStrings(keys []cid.Cid) []string {
	strs := make([]string, 0, len(keys))
	for _, key := range keys {
		strs = append(strs, key.KeyString())
	}
	return strs
}
