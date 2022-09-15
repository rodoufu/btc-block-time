package main

import (
	"context"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"

	"github.com/rodoufu/btc-block-time/pkg/blockchain"
	"github.com/rodoufu/btc-block-time/pkg/btc"
	"github.com/rodoufu/btc-block-time/pkg/entity"
	"github.com/rodoufu/btc-block-time/pkg/persistence"
)

type blockManager struct {
	blocks      []*entity.Block
	latestBlock *entity.Block
	fileName    string
	client      *resty.Client
	blocksChan  chan *entity.Block

	saveEvery                 time.Duration
	waitTime                  time.Duration
	maxParallelRequests       int64
	waitAfterNumberOfRequests int64
}

func (bm *blockManager) LoadBlocks(ctx context.Context, log *logrus.Entry) error {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	done := ctx.Done()

	log.Info("reading blocks")
	var err error
	if len(bm.blocks) == 0 {
		bm.blocks, err = persistence.ReadBlocks(ctx, bm.fileName)
		if err != nil {
			log.WithError(err).Warn("problem reading blocks")
		}
	}

	log.Info("scheduling write blocks")
	defer bm.persistBlocks(log)

	nextHeightToFetch := int64(len(bm.blocks))

	if bm.latestBlock == nil {
		log.Info("getting latest block")
		var latestBlock *btc.Block
		latestBlock, err = btc.GetLatestBlock(ctx, bm.client)
		if err != nil {
			log.WithError(err).Fatal("problem getting latest block")
		}
		bm.latestBlock = latestBlock.ToBlock()
		log.WithFields(logrus.Fields{
			"first_height": nextHeightToFetch,
			"last_height":  latestBlock.Height,
		}).Info("got latest block")
	}

	bm.blocksChan = make(chan *entity.Block, bm.maxParallelRequests)
	defer close(bm.blocksChan)

	log.Info("starting routine to add blocks")
	go bm.saveInMemory(ctx)

	waitGroup := &sync.WaitGroup{}
	log.Info("getting blocks")
	initialTime := time.Now()
	ticker := time.NewTicker(bm.saveEvery)
	defer ticker.Stop()
	for i := nextHeightToFetch; i < bm.latestBlock.Height; i++ {
		select {
		case <-done:
			return ctx.Err()
		case <-ticker.C:
			bm.persistBlocks(log)
		default:
			// no blocking
		}

		if err = bm.fetchBlock(
			ctx, log, waitGroup, cancel, i, nextHeightToFetch, initialTime,
		); err != nil {
			log.WithError(err).Error("problem fetching block")
		}
	}

	waitGroup.Wait()
	return nil
}

func (bm *blockManager) fetchBlock(
	ctx context.Context, log *logrus.Entry, waitGroup *sync.WaitGroup, cancel context.CancelFunc,
	height int64, nextHeightToFetch int64, initialTime time.Time,
) error {
	done := ctx.Done()
	select {
	case <-done:
		return ctx.Err()
	default:
		waitGroup.Add(1)
		go func(height int64) {
			defer waitGroup.Done()

			if height%bm.maxParallelRequests == 0 {
				block, blockErr := btc.GetBlock(ctx, bm.client, height)
				if blockErr != nil {
					log.WithError(blockErr).WithField("height", height).Error("problem fetching block")
					cancel()
					return
				}
				bm.blocksChan <- block.ToBlock()
			} else {
				block, blockErr := blockchain.GetBlock(ctx, bm.client, height)
				if blockErr != nil {
					log.WithError(blockErr).WithField("height", height).Error("problem fetching block")
					cancel()
					return
				}
				bm.blocksChan <- block.ToBlock()
			}
		}(height)

		if (height-nextHeightToFetch)%bm.maxParallelRequests == bm.maxParallelRequests-1 {
			waitGroup.Wait()
		}

		if (height-nextHeightToFetch)%bm.waitAfterNumberOfRequests == bm.waitAfterNumberOfRequests-1 {
			soFar := time.Now().Sub(initialTime)
			blockCount := height - nextHeightToFetch + 1
			blockTime := time.Millisecond * time.Duration(soFar.Milliseconds()/blockCount)
			timeToFinish := time.Duration(bm.latestBlock.Height-height) * blockTime
			log.WithFields(logrus.Fields{
				"height":         height,
				"block_count":    blockCount,
				"block_time":     blockTime,
				"time_to_finish": timeToFinish,
			}).Info("loading blocks")
			time.Sleep(bm.waitTime)
		}
	}
	return nil
}

func (bm *blockManager) saveInMemory(ctx context.Context) {
	done := ctx.Done()
	heightBlock := map[int64]*entity.Block{}

	for {
		select {
		case <-done:
			return
		case block, ok := <-bm.blocksChan:
			if !ok {
				return
			}
			if block.Height == int64(len(bm.blocks)) {
				bm.blocks = append(bm.blocks, block)
			} else {
				heightBlock[block.Height] = block
			}

			maxSoFar := int64(len(bm.blocks))
			if len(heightBlock) == 0 {
				continue
			}
			for height := range heightBlock {
				if height > maxSoFar {
					maxSoFar = height
				}
			}

			for j := int64(len(bm.blocks)); j <= maxSoFar; j++ {
				blockIt, ok := heightBlock[j]
				if !ok {
					break
				}
				bm.blocks = append(bm.blocks, blockIt)
				delete(heightBlock, j)
			}
		}
	}
}

func (bm *blockManager) persistBlocks(log *logrus.Entry) {
	if len(bm.blocks) == 0 {
		log.Warn("no blocks to save")
		return
	}

	log.WithField("number_of_blocks", len(bm.blocks)).Info("saving blocks")
	writeCtx := context.Background()

	if err := persistence.WriteBlocks(writeCtx, bm.fileName, bm.blocks); err != nil {
		log.WithError(err).Error("problem saving blocks")
	}
}

func (bm *blockManager) CheckLongerThan(log *logrus.Entry, threshold time.Duration) {
	lenBlocks := len(bm.blocks)
	maxMineTime := time.Nanosecond
	countMoreThan2Hours := 0
	for i := 0; i < lenBlocks-1; i++ {
		currentBlock := bm.blocks[i]
		nextBlock := bm.blocks[i+1]
		if currentBlock.Height != nextBlock.Height-1 {
			log.WithFields(logrus.Fields{
				"current_block": currentBlock,
				"next_block":    nextBlock,
			}).Fatal("missing block")
		}
		mineTime := nextBlock.Timestamp.Sub(currentBlock.Timestamp)
		if mineTime > maxMineTime {
			maxMineTime = mineTime
		}
		if mineTime > threshold {
			countMoreThan2Hours++
			log.WithFields(logrus.Fields{
				"mine_time": mineTime,
				"height":    i + 1,
			}).Info("mining took more than 2 hours")
		}
	}
	log.WithFields(logrus.Fields{
		"max_mine_time":                       maxMineTime,
		"count_mine_time_larger_than_2_hours": countMoreThan2Hours,
	}).Info("checked all mining times")
}

func (bm *blockManager) ShouldLoad() bool {
	return len(bm.blocks) == 0 || bm.blocks[len(bm.blocks)-1].Height != bm.latestBlock.Height
}

func (bm *blockManager) Len() int {
	return len(bm.blocks)
}
