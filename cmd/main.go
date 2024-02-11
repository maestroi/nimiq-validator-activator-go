package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"nimiq-validator-activator/prometheus"
	"nimiq-validator-activator/rpc"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func init() {
	// Set log flags to include date and time in log messages
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func checkConsensus(client *rpc.Client) bool {
	const maxAttempts = 3
	successfulChecks := 0

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		consensus, err := client.IsConsensusEstablished()
		if err != nil {
			log.Printf("Attempt %d: Error checking consensus: %v\n", attempt, err)
			return false // Immediately return on error
		}

		if consensus {
			successfulChecks++
			if successfulChecks == 1 {
				log.Printf("Consensus established. Verifying stability...")
			}
		} else {
			log.Printf("Consensus not established. Restarting check...")
			return false // Exit if consensus is not established at any attempt
		}

		// Sleep only if not on the last attempt and consensus is not yet verified
		if attempt < maxAttempts {
			time.Sleep(5 * time.Second)
		}
	}

	if successfulChecks == maxAttempts {
		log.Printf("Consensus stability verified. Proceeding...")
		return true
	}

	return false
}

func updateEpochNumberGauge(client *rpc.Client) {
	epochNumber, err := client.GetEpochNumber()
	if err != nil {
		log.Println("Error fetching epoch number:", err)
		return
	}
	prometheus.NimiqEpochNumberGauge.Set(float64(epochNumber))
}

func getPrivateKey(filePath string) (string, error) {
	// Read the entire file content, assuming the key is the first line of the file
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Private Key:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Private Key:")), nil
		}
	}
	return "", fmt.Errorf("private key not found in file")
}

func getVoteKey(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, "Secret Key:") && i+2 < len(lines) {
			// Assuming the secret key is two lines down from the "Secret Key:" line
			return strings.TrimSpace(lines[i+2]), nil
		}
	}
	return "", fmt.Errorf("vote key not found in file")
}

func activateValidator(client *rpc.Client, address string) bool {
	log.Printf("Address: %s", address)

	sigKey, err := getPrivateKey("/keys/signing_key.txt")
	if err != nil {
		log.Println("Error getting signing key:", err)
		return false
	}

	voteKey, err := getVoteKey("/keys/vote_key.txt")
	if err != nil {
		log.Println("Error getting vote key:", err)
		return false
	}

	addressPrivate, err := getPrivateKey("/keys/address.txt")
	if err != nil {
		log.Println("Error getting address private key:", err)
		return false
	}

	log.Println("Importing raw key.")
	_, err = client.ImportRawKey(addressPrivate, "")
	if err != nil {
		log.Println("Failed to import raw key:", err)
		return false
	}

	// Unlock the account
	log.Println("Unlocking account.")
	if err := client.UnlockAccount(address, "", 0); err != nil {
		log.Println("Failed to unlock account:", err)
		return false
	}

	log.Println("Activating Validator")
	rawTx, err := client.SendNewValidatorTransaction(address, address, sigKey, voteKey, address, "", 500, "+0")
	if err != nil {
		log.Println("Failed to create new validator transaction:", err)
		return false
	}

	log.Println("Sending Transaction")
	txHash, err := client.SendRawTransaction(rawTx)
	if err != nil {
		log.Println("Failed to send raw transaction:", err)
		return false
	}

	log.Printf("Transaction sent successfully. Hash: %s", txHash)

	prometheus.ValidatorActivatedGauge.WithLabelValues(address).Set(1)
	prometheus.ValidatorActivatedCounterGauge.WithLabelValues(address).Inc()
	return true
}

func reActivateValidator(client *rpc.Client, address string) bool {
	log.Printf("Address: %s", address)

	sigKey, err := getPrivateKey("/keys/signing_key.txt")
	if err != nil {
		log.Println("Error getting signing key:", err)
		return false
	}

	addressPrivate, err := getPrivateKey("/keys/address.txt")
	if err != nil {
		log.Println("Error getting address private key:", err)
		return false
	}

	log.Println("Importing raw key.")
	_, err = client.ImportRawKey(addressPrivate, "")
	if err != nil {
		log.Println("Failed to import raw key:", err)
		return false
	}

	// Unlock the account
	log.Println("Unlocking account.")
	if err := client.UnlockAccount(address, "", 0); err != nil {
		log.Println("Failed to unlock account:", err)
		return false
	}

	log.Println("Activating Validator")
	txHash, err := client.SendReactivateValidatorTransaction(address, address, sigKey, 500, "+0")
	if err != nil {
		log.Println("Failed to reactivate", err)
		return false
	}

	log.Printf("Transaction sent successfully. Hash: %s", txHash)

	prometheus.ValidatorReActivatedCounterGauge.WithLabelValues(address).Inc()
	return true
}

func updateValidatorMetrics(address string, details *rpc.ValidatorDetails) {
	// Update balance
	prometheus.ValidatorBalanceGauge.WithLabelValues(address).Set(float64(details.Balance))

	// Update number of stakers
	prometheus.ValidatorNumStakersGauge.WithLabelValues(address).Set(float64(details.NumStakers))

	// Update inactivity flag, default to 0 if nil
	inactivityFlag := float64(0)
	if details.InactivityFlag != nil {
		inactivityFlag = float64(*details.InactivityFlag)
	}
	prometheus.ValidatorInactivityFlagGauge.WithLabelValues(address).Set(inactivityFlag)

	// Update retired status, 1 if true, 0 otherwise
	retired := float64(0)
	if details.Retired {
		retired = 1
	}
	prometheus.ValidatorRetiredGauge.WithLabelValues(address).Set(retired)

	// Update jailed from, default to 0 if nil
	jailedFrom := float64(0)
	if details.JailedFrom != nil {
		jailedFrom = float64(*details.JailedFrom)
	}
	prometheus.ValidatorJailedFromGauge.WithLabelValues(address).Set(jailedFrom)

	// validator is active when reaches this point
	prometheus.ValidatorActivatedGauge.WithLabelValues(address).Set(1)

	log.Printf("Validator Prometheus metrics updated.")
}

func updateStake(client *rpc.Client, address string) (bool, error) {
	totalStake, err := client.GetTotalStakeByValidatorAddress(address)
	if err != nil {
		log.Println("Error fetching total stake:", err)
		return false, err
	}
	prometheus.NimiqTotalStakeGauge.WithLabelValues(address).Set(float64(totalStake))
	return true, nil
}

func checkSufficientBalance(client *rpc.Client, address string) (bool, float64) {
	balance, err := client.GetAccountBalanceByAddress(address)
	if err != nil {
		log.Println("Error fetching account balance:", err)
		return false, 0
	}
	balanceInNim := float64(balance) / 100000.0
	prometheus.ValidatorBalanceGauge.WithLabelValues(address).Set(float64(balance))
	return balanceInNim >= 100000.0, balanceInNim
}

func periodicUpdates(client *rpc.Client, address string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sufficient, currentBalance := checkSufficientBalance(client, address)
		if sufficient {
			log.Printf("Sufficient balance detected: %.0f NIM. Checking validator status...", currentBalance)
			if checkAndHandleValidatorStatus(client, address) {
				log.Printf("Validator status checked and handled.")
				return // Exit the loop if the validator is activated or metrics are updated
			}
		} else {
			stakeNeeded := 100000 - currentBalance
			log.Printf("Insufficient balance. %.0f/100 000 NIM. missing %.0f Waiting %d seconds for next check...", currentBalance, stakeNeeded, 10)
		}
	}
}

func checkAndHandleValidatorStatus(client *rpc.Client, address string) bool {
	details, err := client.GetValidatorByAddress(address)
	if err != nil {
		log.Println("Validator not active. Needs activation:", err)
		activateValidator(client, address)
		return false
	}

	// Update metrics regardless of the validator's status
	updateValidatorMetrics(address, details)

	// Check if the validator is retired or jailed and handle accordingly
	if details.Retired {
		log.Printf("Validator is retired. Needs reactivation.")
		reActivateValidator(client, address)
		return false
	}

	if details.JailedFrom != nil {
		log.Printf("Validator is jailed. this takes 8 epochs to be reactivated.")
		prometheus.ValidatorJailedGauge.WithLabelValues(address).Set(1)
		prometheus.ValidatorJailedFromGauge.WithLabelValues(address).Set(float64(*details.JailedFrom))
		return false
	}

	prometheus.ValidatorJailedGauge.WithLabelValues(address).Set(0)
	prometheus.ValidatorJailedFromGauge.WithLabelValues(address).Set(0)
	log.Printf("Validator is active and in good standing.")
	return true
}

func main() {
	const appVersion = "1.0.0"
	const servingPort = ":8000"
	client := rpc.NewClient()

	log.Printf("Starting Nimiq Validator Activator v%s on port %s\n", appVersion, servingPort)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("Prometheus metrics server running on port %s", servingPort)
		if err := http.ListenAndServe(servingPort, nil); err != nil {
			log.Fatalf("Error starting Prometheus HTTP server: %v", err)
		}
	}()

	if !checkConsensus(client) {
		log.Printf("Failed to establish consensus. Exiting...")
		return
	}

	updateEpochNumberGauge(client)

	validatorAddress, err := client.GetAddress()
	if err != nil {
		log.Println("Error fetching validator address:", err)
		return
	}
	log.Println("Validator address:", validatorAddress)
	prometheus.ValidatorActivatedGauge.WithLabelValues(validatorAddress).Set(0)
	prometheus.ValidatorActivatedCounterGauge.WithLabelValues(validatorAddress).Set(0)

	_, err = client.GetValidatorByAddress(validatorAddress)
	if err != nil {
		log.Println("Validator not active. Needs activation:", err)
		sufficientBalance, currentBalance := checkSufficientBalance(client, validatorAddress)
		if sufficientBalance {
			log.Printf("Sufficient Balance detected: %.2f NIM. Checking validator status...", currentBalance)
			checkAndHandleValidatorStatus(client, validatorAddress)
		} else {
			balanceNeeded := 100000.0 - currentBalance
			log.Printf("Initial balance insufficient: %.0f NIM needed to reach 100k NIM.", balanceNeeded)
			periodicUpdates(client, validatorAddress)
		}
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		updateEpochNumberGauge(client)
		state := checkAndHandleValidatorStatus(client, validatorAddress)
		if !state {
			log.Printf("Something went wrong. with the validator!")
		}
		_, err := updateStake(client, validatorAddress)
		if err != nil {
			log.Printf("Error updating stake: %v", err)
		}
	}

}
