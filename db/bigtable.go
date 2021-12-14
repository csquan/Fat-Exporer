package db

import (
	"bytes"
	"context"
	"encoding/gob"
	"eth2-exporter/types"
	"eth2-exporter/utils"
	"fmt"
	"math/big"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

func SaveEpochBigtable(data *types.EpochData) error {

	ctx := context.Background()

	client, err := bigtable.NewClient(ctx, "ethermine-198109", "ethermine-stats", option.WithCredentialsJSON([]byte(utils.Config.Bigtable.Key)))

	if err != nil {
		return err
	}

	if !data.EpochParticipationStats.Finalized {
		return fmt.Errorf("cannot export non-finalized epoch %v to bigtable", data.Epoch)
	}

	// Save the epoch data
	logger.Infof("exporting epoch statistics data")
	proposerSlashingsCount := 0
	attesterSlashingsCount := 0
	attestationsCount := 0
	depositCount := 0
	voluntaryExitCount := 0

	for _, slot := range data.Blocks {
		for _, b := range slot {
			proposerSlashingsCount += len(b.ProposerSlashings)
			attesterSlashingsCount += len(b.AttesterSlashings)
			attestationsCount += len(b.Attestations)
			depositCount += len(b.Deposits)
			voluntaryExitCount += len(b.VoluntaryExits)
		}
	}

	validatorBalanceSum := new(big.Int)
	validatorsCount := 0
	for _, v := range data.Validators {
		if v.ExitEpoch > data.Epoch && v.ActivationEpoch <= data.Epoch {
			validatorsCount++
			validatorBalanceSum = new(big.Int).Add(validatorBalanceSum, new(big.Int).SetUint64(v.Balance))
		}
	}

	validatorBalanceAverage := new(big.Int).Div(validatorBalanceSum, new(big.Int).SetInt64(int64(validatorsCount)))

	tbl := client.Open(utils.Config.Bigtable.Prefix + "_epochs")

	columnFamilyName := "default"

	mut := bigtable.NewMutation()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err = enc.Encode(&types.BigtableEpoch{
		Epoch:                   data.Epoch,
		BlocksCount:             len(data.Blocks),
		ProposerSlashingsCount:  proposerSlashingsCount,
		AttesterSlashingsCount:  attesterSlashingsCount,
		AttestationsCount:       attestationsCount,
		DepositCount:            depositCount,
		VoluntaryExitCount:      voluntaryExitCount,
		ValidatorsCount:         validatorsCount,
		ValidatorBalanceAverage: validatorBalanceAverage.Uint64(),
		ValidatorBalanceSum:     validatorBalanceSum.Uint64(),
		Finalized:               data.EpochParticipationStats.Finalized,
		EligibleEther:           data.EpochParticipationStats.EligibleEther,
		GlobalParticipationRate: data.EpochParticipationStats.GlobalParticipationRate,
		VotedEther:              data.EpochParticipationStats.VotedEther,
	})

	if err != nil {
		return fmt.Errorf("error encoding epoch data: %v", err)
	}

	mut.Set(columnFamilyName, "data", 0, buf.Bytes())

	err = tbl.Apply(ctx, fmt.Sprintf("%v", data.Epoch), mut)
	if err != nil {
		return fmt.Errorf("error writing epoch data: %v", err)
	}

	// save the validator balances
	tbl = client.Open(utils.Config.Bigtable.Prefix + "_vaidator_balance_history")
	muts := make([]*bigtable.Mutation, 0, 100000)
	keys := make([]string, 0, 100000)

	for i, validator := range data.Validators {
		mut := bigtable.NewMutation()
		key := fmt.Sprintf("%v#%v", validator.Index, data.Epoch)

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		err := enc.Encode(&types.BigtableValidatorBalance{
			Balance:          validator.Balance,
			EffectiveBalance: validator.EffectiveBalance,
		})

		if err != nil {
			return fmt.Errorf("error encoding validator balance: %v", err)
		}
		mut.Set(columnFamilyName, "data", 0, buf.Bytes())
		muts = append(muts, mut)
		keys = append(keys, key)

		if i%90000 == 0 {
			logrus.Infof("wrting validator balances batch %v to bigtable", i)
			start := time.Now()
			if _, err := tbl.ApplyBulk(ctx, keys, muts); err != nil {
				return fmt.Errorf("error writing validator balances batch to bigtable: %v", err)
			}
			logrus.Infof("writing batch took %v", time.Since(start))
			muts = make([]*bigtable.Mutation, 0, 100000)
			keys = make([]string, 0, 100000)
		}
	}
	if len(muts) > 0 {
		logrus.Infof("wrting final validator balances batch to bigtable")
		if _, err := tbl.ApplyBulk(ctx, keys, muts); err != nil {
			return fmt.Errorf("error writing final validator balances batch to bigtable: %v", err)
		}
	}

	// save the validators
	// tbl = client.Open(utils.Config.Bigtable.Prefix + "_validators")
	// muts = make([]*bigtable.Mutation, 0, 100000)
	// keys = make([]string, 0, 100000)

	// for i, validator := range data.Validators {
	// 	mut := bigtable.NewMutation()
	// 	key := fmt.Sprintf("%v#%v", validator.Index, data.Epoch)

	// 	var buf bytes.Buffer
	// 	enc := gob.NewEncoder(&buf)
	// 	err := enc.Encode(&types.BigtableValidator{
	// 		Balance:          validator.Balance,
	// 		EffectiveBalance: validator.EffectiveBalance,
	// 	})

	// 	if err != nil {
	// 		return fmt.Errorf("error encoding validator balance: %v", err)
	// 	}
	// 	mut.Set(columnFamilyName, "data", 0, buf.Bytes())
	// 	muts = append(muts, mut)
	// 	keys = append(keys, key)

	// 	if i%90000 == 0 {
	// 		logrus.Infof("wrting validator balances batch %v to bigtable", i)
	// 		start := time.Now()
	// 		if _, err := tbl.ApplyBulk(ctx, keys, muts); err != nil {
	// 			return fmt.Errorf("error writing validator balances batch to bigtable: %v", err)
	// 		}
	// 		logrus.Infof("writing batch took %v", time.Since(start))
	// 		muts = make([]*bigtable.Mutation, 0, 100000)
	// 		keys = make([]string, 0, 100000)
	// 	}
	// }
	// if len(muts) > 0 {
	// 	logrus.Infof("wrting final validator balances batch to bigtable")
	// 	if _, err := tbl.ApplyBulk(ctx, keys, muts); err != nil {
	// 		return fmt.Errorf("error writing final validator balances batch to bigtable: %v", err)
	// 	}
	// }

	return nil
}
