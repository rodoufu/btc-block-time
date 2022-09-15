package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"

	"github.com/rodoufu/btc-block-time/pkg/blockchain"
	"github.com/rodoufu/btc-block-time/pkg/btc"
	"github.com/rodoufu/btc-block-time/pkg/entity"
	"github.com/rodoufu/btc-block-time/pkg/persistence"
)

func loadBlocks(
	ctx context.Context, log *logrus.Entry, fileName string,
	saveEvery, waitTime time.Duration, maxParallelRequests, waitAfterNumberOfRequests int64,
) ([]*entity.Block, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	done := ctx.Done()

	log.Info("reading blocks")
	blocks, err := persistence.ReadBlocks(ctx, fileName)
	if err != nil {
		log.WithError(err).Warn("problem reading blocks")
	}

	log.Info("scheduling write blocks")
	defer persistBlocks(blocks, log, fileName)

	nextHeightToFetch := int64(len(blocks))

	restyClient := resty.New()
	log.Info("getting latest block")
	var latestBlock *btc.Block
	latestBlock, err = btc.GetLatestBlock(ctx, restyClient)
	if err != nil {
		log.WithError(err).Fatal("problem getting latest block")
	}
	log.WithFields(logrus.Fields{
		"first_height": nextHeightToFetch,
		"last_height":  latestBlock.Height,
	}).Info("got latest block")

	blocksChan := make(chan *entity.Block, maxParallelRequests)
	defer close(blocksChan)
	heightBlock := map[int64]*entity.Block{}

	log.Info("starting routine to add blocks")
	go func() {
		for {
			select {
			case <-done:
				return
			case block, ok := <-blocksChan:
				if !ok {
					return
				}
				if block.Height == int64(len(blocks)) {
					blocks = append(blocks, block)
				} else {
					heightBlock[block.Height] = block
				}

				maxSoFar := int64(len(blocks))
				if len(heightBlock) == 0 {
					continue
				}
				for height := range heightBlock {
					if height > maxSoFar {
						maxSoFar = height
					}
				}

				for j := int64(len(blocks)); j <= maxSoFar; j++ {
					block, ok := heightBlock[j]
					if !ok {
						break
					}
					blocks = append(blocks, block)
					delete(heightBlock, j)
				}
			}
		}
	}()

	waitGroup := &sync.WaitGroup{}
	log.Info("getting blocks")
	initialTime := time.Now()
	ticker := time.NewTicker(saveEvery)
	defer ticker.Stop()
	for i := nextHeightToFetch; i < latestBlock.Height; i++ {
		select {
		case <-done:
			return blocks, ctx.Err()
		case <-ticker.C:
			persistBlocks(blocks, log, fileName)
		default:
			waitGroup.Add(1)
			go func(height int64) {
				defer waitGroup.Done()

				if height%maxParallelRequests == 0 {
					block, blockErr := btc.GetBlock(ctx, restyClient, height)
					if blockErr != nil {
						log.WithError(blockErr).WithField("height", height).Error("problem fetching block")
						cancel()
						return
					}
					blocksChan <- block.ToBlock()
				} else {
					block, blockErr := blockchain.GetBlock(ctx, restyClient, height)
					if blockErr != nil {
						log.WithError(blockErr).WithField("height", height).Error("problem fetching block")
						cancel()
						return
					}
					blocksChan <- block.ToBlock()
				}
			}(i)

			if (i-nextHeightToFetch)%maxParallelRequests == maxParallelRequests-1 {
				waitGroup.Wait()
			}

			if (i-nextHeightToFetch)%waitAfterNumberOfRequests == waitAfterNumberOfRequests-1 {
				soFar := time.Now().Sub(initialTime)
				blockCount := i - nextHeightToFetch + 1
				blockTime := time.Millisecond * time.Duration(soFar.Milliseconds()/blockCount)
				timeToFinish := time.Duration(latestBlock.Height-i) * blockTime
				log.WithFields(logrus.Fields{
					"height":         i,
					"block_count":    blockCount,
					"block_time":     blockTime,
					"time_to_finish": timeToFinish,
				}).Info("loading blocks")
				time.Sleep(waitTime)
			}
		}
	}

	waitGroup.Wait()
	return blocks, nil
}

func persistBlocks(blocks []*entity.Block, log *logrus.Entry, fileName string) {
	if len(blocks) == 0 {
		log.Warn("no blocks to save")
		return
	}

	log.WithField("number_of_blocks", len(blocks)).Info("saving blocks")
	writeCtx := context.Background()

	if err := persistence.WriteBlocks(writeCtx, fileName, blocks); err != nil {
		log.WithError(err).Error("problem saving blocks")
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	formatter := &logrus.TextFormatter{}
	formatter.TimestampFormat = "2006-01-02 15:04:05"
	formatter.FullTimestamp = true

	logger := logrus.New()
	logger.SetFormatter(formatter)
	log := logger.WithFields(logrus.Fields{})

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		signalValue := <-c
		log.WithField("signal", signalValue).Warn("got signal")
		cancel()
	}()

	fileName := "blocks.csv"
	blocks, err := loadBlocks(
		ctx, log, fileName, 10*time.Minute, 50*time.Millisecond, 10, 100,
	)
	if err != nil && err != context.Canceled {
		log.WithError(err).Fatal("problem loading blocks")
	}

	lenBlocks := len(blocks)
	maxMineTime := time.Nanosecond
	countMoreThan2Hours := 0
	for i := 0; i < lenBlocks-1; i++ {
		mineTime := blocks[i+1].Timestamp.Sub(blocks[i].Timestamp)
		if mineTime > maxMineTime {
			maxMineTime = mineTime
		}
		if mineTime > 2*time.Hour {
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
