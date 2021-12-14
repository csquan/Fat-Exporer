package types

type BigtableEpoch struct {
	Epoch                   uint64
	BlocksCount             int
	ProposerSlashingsCount  int
	AttesterSlashingsCount  int
	AttestationsCount       int
	DepositCount            int
	VoluntaryExitCount      int
	ValidatorsCount         int
	ValidatorBalanceAverage uint64
	ValidatorBalanceSum     uint64
	Finalized               bool
	EligibleEther           uint64
	GlobalParticipationRate float32
	VotedEther              uint64
}

type BigtableValidatorBalance struct {
	Balance          uint64
	EffectiveBalance uint64
}

type BigtableValidator struct {
	PublicKey                  []byte
	WithdrawableEpoch          uint64
	WithdrawalCredentials      []byte
	Balance                    uint64
	EffectiveBalance           uint64
	Slashed                    bool
	ActivationEligibilityEpoch uint64
	ActivationEpoch            uint64
	ExitEpoch                  uint64
	Balance1d                  uint64
	Balance7d                  uint64
	Balance31d                 uint64
}
