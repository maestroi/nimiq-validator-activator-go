package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"nimiq-validator-activator/prometheus"
	"nimiq-validator-activator/rpc"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	faucetURL    string
	network      string
	nimiqNodeUrl string
	servingPort  = getServingPort()
)

func init() {
	// Set log flags to include date and time in log messages
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Fetching faucet URL from environment variable with a default value
	nimiqNodeUrl = os.Getenv("NIMIQ_NODE_URL")
	if nimiqNodeUrl == "" {
		nimiqNodeUrl = "http://node:8648"
	}

	// Fetching faucet URL from environment variable with a default value
	faucetURL = os.Getenv("FAUCET_URL")
	if faucetURL == "" {
		faucetURL = "https://faucet.pos.nimiq-testnet.com/tapit"
	}

	// Fetching network type from environment variable with a default value
	network = os.Getenv("NIMIQ_NETWORK")
	if network == "" {
		network = "testnet" // Assuming 'testnet' as default, adjust as needed
	}

	log.Printf("Nimiq Node URL: %s", nimiqNodeUrl)
	log.Printf("Faucet URL: %s", faucetURL)
	log.Printf("Network: %s", network)
}

func getServingPort() string {
	servingPortStr := os.Getenv("PROMETHEUS_PORT")
	if servingPortStr == "" {
		return ":8000" // Default port if not set
	}
	if _, err := strconv.Atoi(servingPortStr); err == nil {
		return ":" + servingPortStr // Prefix with colon if conversion is successful
	}
	return ":8000" // Default to ":8000" if conversion fails
}

func checkConsensus(client *rpc.Client) bool {
	const maxAttempts = 3
	successfulChecks := 0

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		consensus, err := client.IsConsensusEstablished()
		if err != nil {
			log.Printf("Attempt %d: Error checking consensus: %v\n", attempt, err)
			log.Println("Waiting 60 seconds before retrying...")
			time.Sleep(60 * time.Second) // Sleep for 60 seconds
			return false                 // Immediately return on error
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
	content, err := os.ReadFile(filePath)
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
	content, err := os.ReadFile(filePath)
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

func fundAddress(address string) bool {
	// Preparing data as URL-encoded form data
	data := url.Values{}
	data.Set("address", address)

	// Making the HTTP POST request
	resp, err := http.PostForm(faucetURL, data)
	if err != nil {
		log.Printf("Error posting to faucet: %v", err)
		return false
	}
	defer resp.Body.Close()

	// Checking for the HTTP response status code
	if resp.StatusCode != http.StatusOK {
		log.Printf("Faucet returned non-OK status: %d %s", resp.StatusCode, resp.Status)
		return false
	}

	log.Println("Funded address successfully.")
	return true
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
	prometheus.NimiqTotalStakeGauge.WithLabelValues(address).Set(float64(details.Balance))

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

func checkActive(client *rpc.Client, address string) bool {
	validatorDetails, err := client.GetValidatorByAddress(address)
	if err != nil {
		log.Println("Error fetching validator details:", err)
		return false
	}
	// Check if the validator's address matches the input address
	// Assuming ValidatorDetails struct has an Address field
	if validatorDetails.Address != address {
		return false // Address does not match or validator not found
	}
	// Check if the balance is above the threshold and address matches
	isActive := validatorDetails.Address == address
	return isActive
}

func periodicUpdates(client *rpc.Client, address string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sufficient, currentBalance := checkSufficientBalance(client, address)
		isActive := checkActive(client, address)

		if sufficient || isActive {
			log.Printf("Sufficient balance detected: %.0f NIM. Checking validator status...", currentBalance)
			if checkAndHandleValidatorStatus(client, address) {
				log.Printf("Validator status checked and handled.")
				return // Exit the loop if the validator is activated or metrics are updated
			}
		} else {
			if network == "testnet" {
				if fundAddress(address) {
					log.Printf("Funded address successfully.")
				} else {
					log.Printf("Failed to fund address.")
				}
			}
			stakeNeeded := 100000 - currentBalance
			log.Printf("Insufficient balance. %.0f/100 000 NIM. missing %.0f Waiting %d seconds for next check...", currentBalance, stakeNeeded, 10)
		}
	}
}

func checkAndHandleValidatorStatus(client *rpc.Client, address string) bool {
	const blocksForReactivation = 8000

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

	currentBlockNumber, err := client.GetCurrentBlockNumber()
	if err != nil {
		log.Println("Error fetching current block number:", err)
		return false
	}

	if details.JailedFrom != nil {
		blocksSinceJailed := currentBlockNumber - int64(*details.JailedFrom)
		if blocksSinceJailed < blocksForReactivation {
			// Validator is considered still jailed if the difference is less than 8000 blocks
			log.Printf("Validator is still within the jailed period. Blocks since jailed: %d", blocksSinceJailed)
			prometheus.ValidatorJailedGauge.WithLabelValues(address).Set(1)
			prometheus.ValidatorJailedFromGauge.WithLabelValues(address).Set(float64(*details.JailedFrom))
		} else {
			prometheus.ValidatorJailedGauge.WithLabelValues(address).Set(0)
			// If reactivation or further action is required when a validator is no longer considered jailed, add that logic here.
		}
	}
	prometheus.ValidatorJailedGauge.WithLabelValues(address).Set(0)
	prometheus.ValidatorJailedFromGauge.WithLabelValues(address).Set(0)
	log.Printf("Validator is active and in good standing.")
	return true
}

func main() {
	const appVersion = "1.0.0"
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
		_, _ = checkSufficientBalance(client, validatorAddress)
	}

}
