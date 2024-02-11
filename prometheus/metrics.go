package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// NimiqEpochNumberGauge tracks the current Nimiq epoch number
	NimiqEpochNumberGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "nimiq_epoch_number",
		Help: "Current Nimiq epoch number.",
	})
	NimiqValidatorBalanceGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_balance_luna",
		Help: "Current balance of the validator in Luna.",
	}, []string{"address"}) // Label for address

	NimiqTotalStakeGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_stake_balance_luna",
		Help: "Current stake balance of the validator in Luna.",
	}, []string{"address"}) // Label for address

	ValidatorBalanceGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_balance",
		Help: "Balance of the validator in Luna.",
	}, []string{"address"})

	ValidatorNumStakersGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_num_stakers",
		Help: "Number of stakers for the validator.",
	}, []string{"address"})

	ValidatorInactivityFlagGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_inactivity_flag",
		Help: "Inactivity flag for the validator, 0 if active.",
	}, []string{"address"})

	ValidatorRetiredGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_retired",
		Help: "Whether the validator is retired, 1 for yes, 0 for no.",
	}, []string{"address"})

	ValidatorJailedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_jailed",
		Help: "Block number from which the validator is jailed, 0 if not jailed.",
	}, []string{"address"})

	ValidatorJailedFromGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_jailed_from",
		Help: "Block number from which the validator is jailed, 0 if not jailed.",
	}, []string{"address"})

	ValidatorActivatedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_activated",
		Help: "Activation status of a Nimiq validator. 1 indicates activated.",
	}, []string{"address"}) // Label by validator address

	ValidatorActivatedCounterGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_activated_counter",
		Help: "Activation status of a Nimiq validator.",
	}, []string{"address"}) // Label by validator address

	ValidatorReActivatedCounterGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nimiq_validator_reactivated_counter",
		Help: "Reactivation status of a Nimiq validator.",
	}, []string{"address"}) // Label by validator address
)

func init() {
	// Register the new gauges
	prometheus.MustRegister(
		NimiqEpochNumberGauge,
		NimiqValidatorBalanceGauge,
		NimiqTotalStakeGauge,
		ValidatorBalanceGauge,
		ValidatorNumStakersGauge,
		ValidatorInactivityFlagGauge,
		ValidatorRetiredGauge,
		ValidatorJailedGauge,
		ValidatorJailedFromGauge,
		ValidatorActivatedGauge,
		ValidatorActivatedCounterGauge,
		ValidatorReActivatedCounterGauge,
	)
}
