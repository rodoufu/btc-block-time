package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
)

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

	poisson := distuv.Poisson{
		Lambda: 12,
		Src:    rand.NewSource(uint64(time.Now().UnixMilli())),
	}
	prob := poisson.Prob(0)
	log.WithFields(logrus.Fields{
		"probability":       prob * 100,
		"chances_of_one_in": 1 / prob,
	}).Info("chances of not getting a block within 120 minutes")

	bm := &blockManager{
		blocks:                    nil,
		latestBlock:               nil,
		fileName:                  "blocks.csv",
		client:                    resty.New(),
		saveEvery:                 10 * time.Minute,
		waitTime:                  50 * time.Millisecond,
		maxParallelRequests:       12,
		waitAfterNumberOfRequests: 100,
	}
	var err error
	done := ctx.Done()

FindBlocks:
	for bm.ShouldLoad() {
		select {
		case <-done:
			break FindBlocks
		default:
			log.WithField("block_so_far", bm.Len()).Info("loading blocks")
			if err = bm.LoadBlocks(ctx, log); err != nil && err != context.Canceled {
				log.WithError(err).Fatal("problem loading blocks")
				time.Sleep(time.Second)
			}
		}
	}

	bm.CheckLongerThan(log, 2*time.Hour)
}
