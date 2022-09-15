package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"

	"github.com/rodoufu/btc-block-time/pkg/manager"
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

	// Calculating the odds of a block taking more than 2 hours
	poisson := distuv.Poisson{
		Lambda: 12,
		Src:    rand.NewSource(uint64(time.Now().UnixMilli())),
	}
	prob := poisson.Prob(0)
	log.WithFields(logrus.Fields{
		"probability":       prob * 100,
		"chances_of_one_in": 1 / prob,
	}).Info("chances of not getting a block within 120 minutes")

	bm := manager.NewBlockManager()
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
