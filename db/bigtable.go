package db

import (
	"context"
	"errors"
	"eth2-exporter/types"
	"fmt"

	gcp_bigtable "cloud.google.com/go/bigtable"
	"github.com/golang/protobuf/proto"
	"github.com/sirupsen/logrus"
)

var ErrBlockNotFound = errors.New("block not found")

const max_block_number = 1000000000

type Bigtable struct {
	client      *gcp_bigtable.Client
	tableData   *gcp_bigtable.Table
	tableBlocks *gcp_bigtable.Table
	chainId     string
}

func NewBigtable(project, instance, chainId string) (*Bigtable, error) {
	btClient, err := gcp_bigtable.NewClient(context.Background(), project, instance)

	if err != nil {
		return nil, err
	}

	bt := &Bigtable{
		client:      btClient,
		tableData:   btClient.Open("data"),
		tableBlocks: btClient.Open("blocks"),
		chainId:     chainId,
	}
	return bt, nil
}

func (bigtable *Bigtable) Close() {
	bigtable.client.Close()
}

func (bigtable *Bigtable) SaveBlock(block *types.Eth1Block) error {

	encodedBc, err := proto.Marshal(block)

	if err != nil {
		return err
	}
	family := "default"
	ts := gcp_bigtable.Timestamp(0)

	mut := gcp_bigtable.NewMutation()
	mut.Set(family, "data", ts, encodedBc)

	err = bigtable.tableBlocks.Apply(context.Background(), fmt.Sprintf("%s:%s", bigtable.chainId, reversedPaddedBlockNumber(block.Number)), mut)

	if err != nil {
		return err
	}
	return nil
}

func (bigtable *Bigtable) GetLastBlock() (*types.Eth1Block, error) {

	lastBlockArr, err := bigtable.GetBlocks(-1, 1)

	if err != nil {
		return nil, err
	}

	if len(lastBlockArr) == 0 {
		return nil, ErrBlockNotFound
	}

	return lastBlockArr[0], nil

}

func (bigtable *Bigtable) GetBlocks(number int64, count int) ([]*types.Eth1Block, error) {

	results := make([]*types.Eth1Block, 0, count)

	start := ""

	if number >= 0 {
		start = reversedPaddedBlockNumber(uint64(number))
	}

	// Retrieve the blocks
	err := bigtable.tableBlocks.ReadRows(context.Background(), gcp_bigtable.NewRange(fmt.Sprintf("%s:%s", bigtable.chainId, start), ""), func(r gcp_bigtable.Row) bool {
		bc := &types.Eth1Block{}
		err := proto.Unmarshal(r["default"][0].Value, bc)

		if err != nil {
			logrus.Errorf("error parsing block data for key %v: %v", r.Key(), err)
			return false
		}
		results = append(results, bc)
		return true
	}, gcp_bigtable.LimitRows(int64(count)))

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (bigtable *Bigtable) GetBlock(number uint64) (*types.Eth1Block, error) {

	row, err := bigtable.tableBlocks.ReadRow(context.Background(), fmt.Sprintf("%s:%s", bigtable.chainId, reversedPaddedBlockNumber(number)))

	if err != nil {
		return nil, err
	}

	if len(row["default"]) == 0 { // block not found
		return nil, ErrBlockNotFound
	}

	bc := &types.Eth1Block{}
	err = proto.Unmarshal(row["default"][0].Value, bc)

	if err != nil {
		return nil, err
	}

	return bc, nil
}

func reversedPaddedBlockNumber(blockNumber uint64) string {
	return fmt.Sprintf("%09d", max_block_number-blockNumber)
}
