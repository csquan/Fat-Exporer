package main

import (
	"context"
	"eth2-exporter/db"
	"eth2-exporter/rpc"
	"eth2-exporter/types"
	"eth2-exporter/utils"
	"eth2-exporter/version"
	"flag"
	"fmt"

	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/sirupsen/logrus"
)

func main() {
	configPath := flag.String("config", "", "Path to the config file, if empty string defaults will be used")
	flag.Parse()

	cfg := &types.Config{}
	err := utils.ReadConfig(cfg, *configPath)
	if err != nil {
		logrus.Fatalf("error reading config file: %v", err)
	}
	utils.Config = cfg
	logrus.WithField("config", *configPath).WithField("version", version.Version).WithField("chainName", utils.Config.Chain.Config.ConfigName).Printf("starting")

	ctx, done := context.WithTimeout(context.Background(), time.Second*30)
	defer done()
	db.MustInitBigtableAdmin(ctx, cfg.Bigtable.Project, cfg.Bigtable.Instance)

	err = db.BigAdminClient.SetupBigtableCache()
	if err != nil {
		logrus.Fatalf("error setting up bigtable cache err: %v", err)
	}

	bt, err := db.InitBigtable(cfg.Bigtable.Project, cfg.Bigtable.Instance, fmt.Sprintf("%d", utils.Config.Chain.Config.DepositChainID))
	if err != nil {
		logrus.Fatalf("error initializing bigtable %v", err)
	}

	rpc.CurrentErigonClient, err = rpc.NewErigonClient(utils.Config.Eth1ErigonEndpoint)
	if err != nil {
		logrus.Fatalf("error initializing erigon client: %v", err)
	}

	for {
		lastBlockFromNode, err := rpc.CurrentErigonClient.GetLatestEth1BlockNumber()
		if err != nil {
			logrus.Fatalf("error retrieving latest eth block number: %v", err)
		}

		blockStartTs := time.Now()
		bc, timings, err := rpc.CurrentErigonClient.GetBlock(int64(lastBlockFromNode))
		if err != nil {
			logrus.Error(err)
		}

		dbStart := time.Now()
		err = bt.SaveBlock(bc)
		if err != nil {
			logrus.Error(err)
		}

		logrus.Infof("retrieved & saved block %v (0x%x) in %v (header: %v, receipts: %v, traces: %v, db: %v)", bc.Number, bc.Hash, time.Since(blockStartTs), timings.Headers, timings.Receipts, timings.Traces, time.Since(dbStart))
	}

	utils.WaitForCtrlC()

	logrus.Println("exiting...")
}
