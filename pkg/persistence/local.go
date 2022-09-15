package persistence

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/rodoufu/btc-block-time/pkg/btc"
)

func ReadBlocks(ctx context.Context, fileName string) ([]*btc.Block, error) {
	csvFile, err := os.Open(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "problem opening: %v", fileName)
	}

	csvReader := csv.NewReader(csvFile)
	_, err = csvReader.Read()
	if err == io.EOF {
		return nil, nil
	}
	var lineNumber int64 = 0
	var blocks []*btc.Block
	for {
		var record []string
		record, err = csvReader.Read()
		if err == io.EOF {
			break
		}

		lineNumber++
		if err != nil {
			return nil, errors.Wrapf(err, "problem reading CSV line %v", lineNumber)
		}

		var height int64
		height, err = strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid height '%v' at line %v", record[0], lineNumber)
		}
		if height+1 != lineNumber {
			return blocks, fmt.Errorf("invalid block number at line: %v", lineNumber)
		}

		var timestamp int64
		timestamp, err = strconv.ParseInt(record[1], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid timestamp '%v' at line %v", record[1], lineNumber)
		}

		blocks = append(blocks, &btc.Block{
			Height:    height,
			Timestamp: timestamp,
			Hash:      record[2],
		})
	}
	return blocks, nil
}

func WriteBlocks(ctx context.Context, fileName string, blocks []*btc.Block) error {
	if _, err := os.Stat(fileName); err == nil || !errors.Is(err, os.ErrNotExist) {
		if errRemove := os.Remove(fileName); errRemove != nil {
			return errors.Wrapf(errRemove, "problme removing file: %v", fileName)
		}
	}
	csvFile, err := os.Create(fileName)
	if err != nil {
		return errors.Wrapf(err, "problem opening: %v", fileName)
	}

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	if err = csvWriter.Write([]string{"height", "timestamp", "hash"}); err != nil {
		return errors.Wrap(err, "problem writing header")
	}

	lineNumber := 0
	for _, block := range blocks {
		lineNumber++
		if err = csvWriter.Write([]string{
			strconv.FormatInt(block.Height, 10),
			strconv.FormatInt(block.Timestamp, 10),
			block.Hash,
		}); err != nil {
			return errors.Wrapf(err, "problem writing line: %v", lineNumber)
		}
	}
	return nil
}
