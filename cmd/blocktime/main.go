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

	"github.com/rodoufu/btc-block-time/pkg/btc"
	"github.com/rodoufu/btc-block-time/pkg/persistence"
)

func main() {
	logger := logrus.New()
	formatter := &logrus.TextFormatter{}
	formatter.TimestampFormat = "2006-01-02 15:04:05"
	logger.SetFormatter(formatter)
	log := logger.WithFields(logrus.Fields{})
	ctx, cancel := context.WithCancel(context.Background())
	done := ctx.Done()
	defer cancel()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Warn("got signal")
		cancel()
	}()

	fileName := "blocks.csv"
	log.Info("reading blocks")
	blocks, err := persistence.ReadBlocks(ctx, fileName)
	if err != nil {
		log.WithError(err).Warn("problem reading blocks")
	}

	log.Info("scheduling write blocks")
	defer func() {
		if len(blocks) == 0 {
			log.Warn("no blocks to save")
			return
		}

		log.WithField("number_of_blocks", len(blocks)).Info("saving blocks")
		writeCtx := context.Background()
		err = persistence.WriteBlocks(writeCtx, fileName, blocks)
		if err != nil {
			log.WithError(err).Error("problem saving blocks")
		}
	}()

	nextHeightToFetch := int64(len(blocks))
	maxParallelRequests := int64(1)
	waitTime := 50 * time.Millisecond
	waitAfterNumberOfRequests := int64(100)

	restyClient := resty.New()
	log.Info("getting latest block")
	var latestBlock *btc.Block
	latestBlock, err = btc.GetLatestBlock(ctx, restyClient)
	if err != nil {
		log.WithError(err).Fatal("problem getting latest block")
	}
	log = log.WithFields(logrus.Fields{
		"first_height": nextHeightToFetch,
		"last_height":  latestBlock.Height,
	})
	log.WithField("height", latestBlock.Height).Info("got latest block")

	blocksChan := make(chan *btc.Block, maxParallelRequests)
	defer close(blocksChan)
	heightBlock := map[int64]*btc.Block{}

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
	for i := nextHeightToFetch; i < latestBlock.Height; i++ {
		select {
		case <-done:
			return
		default:
			waitGroup.Add(1)
			go func(height int64) {
				defer waitGroup.Done()
				block, blockErr := btc.GetBlock(ctx, restyClient, height)
				if blockErr != nil {
					log.WithError(blockErr).WithField("height", height).Error("problem fetching block")
					cancel()
					return
				}
				blocksChan <- block
			}(i)

			if (i-nextHeightToFetch)%maxParallelRequests == maxParallelRequests-1 {
				waitGroup.Wait()
			}

			if (i-nextHeightToFetch)%waitAfterNumberOfRequests == waitAfterNumberOfRequests-1 {
				log.WithField("height", i).Info("loading blocks")
				time.Sleep(waitTime)
			}
		}
	}

	waitGroup.Wait()
}
