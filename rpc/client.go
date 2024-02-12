package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// Client holds the configuration for the Nimiq RPC client
type Client struct {
	NodeURL string
}

// NewClient now fetches the Nimiq node URL from an environment variable
func NewClient() *Client {
	nodeURL := os.Getenv("NIMIQ_NODE_URL") // Get the Nimiq node URL from an environment variable
	if nodeURL == "" {
		nodeURL = "http://node:8648" // Default to testnet if not specified
	}
	return &Client{
		NodeURL: nodeURL,
	}
}

// query makes a generic RPC call to the Nimiq node
func (c *Client) query(method string, params interface{}) (json.RawMessage, error) {
	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(c.NodeURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if err, exists := result["error"]; exists {
		return nil, fmt.Errorf("RPC error: %s", err)
	}

	return result["result"], nil
}

// GetConsensusState retrieves the consensus state from the Nimiq node
func (c *Client) IsConsensusEstablished() (bool, error) {
	result, err := c.query("isConsensusEstablished", []interface{}{}) // Correct method with empty params
	if err != nil {
		return false, err
	}

	// Adjusting result parsing according to the provided result structure
	var consensusResult struct {
		Data bool `json:"data"`
	}
	if err := json.Unmarshal(result, &consensusResult); err != nil {
		return false, err
	}

	return consensusResult.Data, nil
}

// Add to your existing rpc/client.go

// GetEpochNumber retrieves the current epoch number from the Nimiq node
func (c *Client) GetEpochNumber() (int, error) {
	result, err := c.query("getEpochNumber", []interface{}{})
	if err != nil {
		return 0, err
	}

	var epochResult struct {
		Data int `json:"data"`
	}
	if err := json.Unmarshal(result, &epochResult); err != nil {
		return 0, err
	}

	return epochResult.Data, nil
}

// GetAddress retrieves the validator's address from the Nimiq node
func (c *Client) GetAddress() (string, error) {
	result, err := c.query("getAddress", []interface{}{})
	if err != nil {
		return "", err
	}

	var addressResult struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &addressResult); err != nil {
		return "", err
	}

	return addressResult.Data, nil
}

// GetAccountBalanceByAddress retrieves the account balance for a given address from the Nimiq node
func (c *Client) GetAccountBalanceByAddress(address string) (int64, error) {
	result, err := c.query("getAccountByAddress", []interface{}{address})
	if err != nil {
		return 0, err
	}

	var accountResult struct {
		Data struct {
			Balance int64 `json:"balance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result, &accountResult); err != nil {
		return 0, err
	}

	return accountResult.Data.Balance, nil
}

// GetTotalStakeByValidatorAddress retrieves the total stake for a validator address
func (c *Client) GetTotalStakeByValidatorAddress(address string) (int64, error) {
	result, err := c.query("getStakersByValidatorAddress", []interface{}{address})
	if err != nil {
		return 0, err
	}

	var stakeResult struct {
		Data []struct {
			Balance int64 `json:"balance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result, &stakeResult); err != nil {
		return 0, err
	}

	var totalStake int64 = 0
	for _, stake := range stakeResult.Data {
		totalStake += stake.Balance
	}

	return totalStake, nil
}

func (c *Client) GetValidatorByAddress(address string) (*ValidatorDetails, error) {
	result, err := c.query("getValidatorByAddress", []interface{}{address})
	if err != nil {
		return nil, err // RPC error or address is not a validator
	}

	var validatorResult struct {
		Data *ValidatorDetails `json:"data"`
	}
	if err := json.Unmarshal(result, &validatorResult); err != nil {
		return nil, err // Error parsing the result
	}

	return validatorResult.Data, nil
}

func (c *Client) ImportRawKey(privateKey, passphrase string) (string, error) {
	result, err := c.query("importRawKey", []interface{}{privateKey, passphrase})
	if err != nil {
		return "", err
	}

	var importResult struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &importResult); err != nil {
		return "", err
	}

	if importResult.Data == "" {
		return "", fmt.Errorf("failed to import key, no address returned")
	}

	return importResult.Data, nil
}

func (c *Client) GetCurrentBlockNumber() (int64, error) {
	result, err := c.query("getBlockNumber", []interface{}{})
	if err != nil {
		return 0, err
	}

	var blockNumberResult struct {
		Data int64 `json:"data"`
	}
	if err := json.Unmarshal(result, &blockNumberResult); err != nil {
		return 0, err
	}

	return blockNumberResult.Data, nil
}

func (c *Client) UnlockAccount(address, passphrase string, duration int) error {
	result, err := c.query("unlockAccount", []interface{}{address, passphrase, duration})
	if err != nil {
		return err
	}

	var unlockResult struct {
		Data bool `json:"data"`
	}
	if err := json.Unmarshal(result, &unlockResult); err != nil {
		return err
	}

	if !unlockResult.Data {
		return fmt.Errorf("failed to unlock account")
	}

	return nil
}

func (c *Client) SendNewValidatorTransaction(senderAddress, validatorAddress, signingSecretKey, votingSecretKey, rewardAddress, signalData string, feeInLuna int, validityStartHeight string) (string, error) {
	params := []interface{}{
		senderAddress, validatorAddress, signingSecretKey, votingSecretKey, rewardAddress, signalData, feeInLuna, validityStartHeight,
	}
	result, err := c.query("sendNewValidatorTransaction", params)
	if err != nil {
		return "", err
	}

	var txResult struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &txResult); err != nil {
		return "", err
	}

	return txResult.Data, nil
}

func (c *Client) SendReactivateValidatorTransaction(senderAddress, validatorAddress, signingSecretKey string, feeInLuna int, validityStartHeight string) (string, error) {
	params := []interface{}{
		senderAddress, validatorAddress, signingSecretKey, feeInLuna, validityStartHeight,
	}
	result, err := c.query("sendReactivateValidatorTransaction", params)
	if err != nil {
		return "", err
	}

	var txResult struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &txResult); err != nil {
		return "", err
	}

	return txResult.Data, nil
}

func (c *Client) SendRawTransaction(rawTx string) (string, error) {
	result, err := c.query("sendRawTransaction", []interface{}{rawTx})
	if err != nil {
		return "", err
	}

	var sendResult struct {
		Data string `json:"data"` // Assuming this is where the transaction hash is returned
	}
	if err := json.Unmarshal(result, &sendResult); err != nil {
		return "", err
	}

	return sendResult.Data, nil
}

// ValidatorDetails struct to hold the parsed validator information
type ValidatorDetails struct {
	Address        string `json:"address"`
	Balance        int64  `json:"balance"`
	NumStakers     int    `json:"numStakers"`
	InactivityFlag *int   `json:"inactivityFlag,omitempty"`
	Retired        bool   `json:"retired"`
	JailedFrom     *int   `json:"jailedFrom,omitempty"`
}
