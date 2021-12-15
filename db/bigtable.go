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
)

func SaveEpochBigtable(data *types.EpochData) error {

	farFutureEpoch := uint64(18446744073709551615)

	ctx := context.Background()

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

	tbl := BTClient.Open(utils.Config.Bigtable.Prefix + "_epochs")

	columnFamilyName := "default"

	mut := bigtable.NewMutation()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(&types.BigtableEpoch{
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
	logger.Infof("exported epoch statistics data")

	// save the validator balances
	tbl = BTClient.Open(utils.Config.Bigtable.Prefix + "_vaidator_balance_history")
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

		if i != 0 && i%90000 == 0 {
			logger.Infof("wrting validator balances batch %v to bigtable", i)
			start := time.Now()
			if _, err := tbl.ApplyBulk(ctx, keys, muts); err != nil {
				return fmt.Errorf("error writing validator balances batch to bigtable: %v", err)
			}
			logger.Infof("writing batch took %v", time.Since(start))
			muts = make([]*bigtable.Mutation, 0, 100000)
			keys = make([]string, 0, 100000)
		}
	}
	if len(muts) > 0 {
		logger.Infof("writing final validator balances batch to bigtable")
		start := time.Now()
		if _, err := tbl.ApplyBulk(ctx, keys, muts); err != nil {
			return fmt.Errorf("error writing final validator balances batch to bigtable: %v", err)
		}
		logger.Infof("writing batch took %v", time.Since(start))
	}

	//save the validators
	tbl = BTClient.Open(utils.Config.Bigtable.Prefix + "_validators")
	muts = make([]*bigtable.Mutation, 0, 100000)
	keys = make([]string, 0, 100000)

	for i, validator := range data.Validators {
		mut := bigtable.NewMutation()
		key := fmt.Sprintf("%v#%v", validator.Index, data.Epoch)

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)

		btv := &types.BigtableValidator{
			PublicKey:                  validator.PublicKey,
			WithdrawableEpoch:          validator.WithdrawableEpoch,
			WithdrawalCredentials:      validator.WithdrawalCredentials,
			Balance:                    validator.Balance,
			EffectiveBalance:           validator.EffectiveBalance,
			Slashed:                    validator.Slashed,
			ActivationEligibilityEpoch: validator.ActivationEligibilityEpoch,
			ActivationEpoch:            validator.ActivationEpoch,
			ExitEpoch:                  validator.ExitEpoch,
			Balance1d:                  validator.Balance1d,
			Balance7d:                  validator.Balance7d,
			Balance31d:                 validator.Balance31d,
		}

		if validator.ExitEpoch <= data.Epoch && validator.Slashed {
			btv.Status = "slashed"
		} else if validator.ExitEpoch <= data.Epoch {
			btv.Status = "exited"
		} else if validator.ActivationEligibilityEpoch == farFutureEpoch {
			btv.Status = "deposited"
		} else if validator.ActivationEpoch > data.Epoch {
			btv.Status = "pending"
		} else if validator.ActivationEpoch == data.Epoch {
			btv.Status = "pending"
		}

		err := enc.Encode(btv)
		if err != nil {
			return fmt.Errorf("error encoding validator: %v", err)
		}
		mut.Set(columnFamilyName, "data", 0, buf.Bytes())
		muts = append(muts, mut)
		keys = append(keys, key)

		if i%90000 == 0 {
			logger.Infof("wrting validator balances batch %v to bigtable", i)
			start := time.Now()
			if _, err := tbl.ApplyBulk(ctx, keys, muts); err != nil {
				return fmt.Errorf("error writing validator balances batch to bigtable: %v", err)
			}
			logger.Infof("writing batch took %v", time.Since(start))
			muts = make([]*bigtable.Mutation, 0, 100000)
			keys = make([]string, 0, 100000)
		}
	}
	if len(muts) > 0 {
		logger.Infof("wrting final validator balances batch to bigtable")
		if _, err := tbl.ApplyBulk(ctx, keys, muts); err != nil {
			return fmt.Errorf("error writing final validator balances batch to bigtable: %v", err)
		}
	}

	return nil
}
