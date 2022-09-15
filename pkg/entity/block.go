package entity

import "time"

type Block struct {
	Height    int64
	Hash      string
	Timestamp time.Time
}
