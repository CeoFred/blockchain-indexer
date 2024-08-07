package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/CeoFred/gin-boilerplate/internal/smartcontract/binding/Bridge"
	"github.com/CeoFred/gin-boilerplate/internal/smartcontract/binding/ERC20"
	"github.com/CeoFred/gin-boilerplate/internal/smartcontract/binding/PoolSwap"

	"github.com/gofrs/uuid"

	"github.com/CeoFred/gin-boilerplate/internal/models"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type LogTransfer struct {
	From  common.Address `json:"from"`
	To    common.Address `json:"to"`
	Value *big.Int       `json:"value"`
}

type LogApproval struct {
	Owner   common.Address `json:"owner"`
	Spender common.Address `json:"spender"`
	Value   *big.Int       `json:"value"`
}

type LogSwap struct {
	Sender       common.Address `json:"sender"`
	Recipient    common.Address `json:"recipient"`
	Amount0      *big.Int       `json:"amount0"`
	Amount1      *big.Int       `json:"amount1"`
	SqrtPriceX96 *big.Int       `json:"sqrtPriceX96"`
	Liquidity    *big.Int       `json:"liquidity"`
	Tick         *big.Int       `json:"tick"`
}

type LogBridgeSentTokens struct {
	FromUser     common.Address `json:"from_user"`
	ToUser       common.Address `json:"to_user"`
	FromChain    string         `json:"from_chain"`
	ToChain      string         `json:"to_chain"`
	Amount       *big.Int       `json:"amount"`
	ExchangeRate *big.Int       `json:"exchange_rate"`
	Raw          types.Log      // Blockchain specific contextual infos
}
type LogBridgeFulfilledTokens struct {
	FromUser     common.Address `json:"from_user"`
	ToUser       common.Address `json:"to_user"`
	FromChain    string         `json:"from_chain"`
	ToChain      string         `json:"to_chain"`
	Amount       *big.Int       `json:"amount"`
	ExchangeRate *big.Int       `json:"exchange_rate"`
	Raw          types.Log      // Blockchain specific contextual infos
}
type BlockchainService struct {
	Client          *ethclient.Client
	ContractABI     abi.ABI
	ContractAddress common.Address
}

func NewBlockchainService(rpcURL string) (*BlockchainService, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	return &BlockchainService{
		Client: client,
	}, nil
}

func (b *BlockchainService) Close() {
	b.Client.Close()
}

func (b *BlockchainService) SetContract(address, contractABI string) error {
	parsedABI, err := abi.JSON(strings.NewReader(contractABI))
	if err != nil {
		return err
	}

	b.ContractABI = parsedABI
	b.ContractAddress = common.HexToAddress(address)

	return nil
}

func (b *BlockchainService) IsContract(contractAddress string) bool {
	address := common.HexToAddress(contractAddress)
	bytecode, err := b.Client.CodeAt(context.Background(), address, nil)
	if err != nil {
		return false
	}
	isContract := len(bytecode) > 0
	return isContract
}

func (b *BlockchainService) Subscribe(address string, logs chan types.Log) error {
	contractAddress := common.HexToAddress(address)
	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
	}

	sub, err := b.Client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal(err)
	}

	err = <-sub.Err()
	if err != nil {
		log.Printf("Error in subscription: %v", err)
	}

	log.Println("Subscription successful")

	return nil
}

func (b *BlockchainService) GetLatestBlockNumber() (*big.Int, error) {
	header, err := b.Client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	return header.Number, nil
}

func (s *BlockchainService) BlockByHash(hash common.Hash) (*types.Block, error) {

	block, err := s.Client.BlockByHash(context.Background(), hash)
	if err != nil {
		log.Fatal(err)
	}
	return block, nil
}

func (s *BlockchainService) BlockByNumber(blockNumber uint64) (*types.Block, error) {
	block, err := s.Client.BlockByNumber(context.Background(), big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}
	return block, nil
}

func (b *BlockchainService) BlockTransaction(blockNum uint) (types.Transactions, error) {
	blockNumber := big.NewInt(int64(blockNum))

	block, err := b.Client.BlockByNumber(context.Background(), blockNumber)
	if err != nil {
		return nil, err
	}

	return block.Transactions(), nil
}

func (b *BlockchainService) QueryLogs(address string, startBlock, endBlock uint) ([]types.Log, error) {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{common.HexToAddress(address)},
		FromBlock: big.NewInt(int64(startBlock)),
		ToBlock:   big.NewInt(int64(endBlock)),
	}

	logs, err := b.Client.FilterLogs(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to filter logs: %w", err)
	}

	return logs, nil
}
func (b *BlockchainService) ProcessTokenBridge(logs []types.Log, events []*models.ContractEvent) ([]*models.EventLog, map[string][]*models.UserAction) {
	eventLogs := []*models.EventLog{}

	eventStructMap := map[string]interface{}{
		"FulfilledTokens": new(LogBridgeFulfilledTokens),
		"SentTokens":      new(LogBridgeSentTokens),
	}

	contractAbi, err := abi.JSON(strings.NewReader(string(Bridge.BridgeABI)))
	if err != nil {
		log.Fatal(err)
	}

	userEvents := map[string][]*models.UserAction{}

	for _, elog := range logs {
		event_signature := elog.Topics[0].Hex()
		for _, contract_event := range events {

			if contract_event.Event.Signature == event_signature {

				t, ok := eventStructMap[contract_event.Event.Name]
				if !ok {
					log.Printf("Unknown event name: %s", contract_event.Event.Name)
					continue
				}

				err := contractAbi.UnpackIntoInterface(t, contract_event.Event.Name, elog.Data)
				if err != nil {
					log.Fatal(err)
				}

				var eventData interface{}

				switch eventType := t.(type) {
				case *LogBridgeSentTokens:
					eventType.ToUser = common.HexToAddress(elog.Topics[1].Hex())
					eventData = eventType

					fmt.Printf("Processed Sent Tokens event: FromUser=%s,ToUser=%s,FromChain=%s,ToChain=%s,Amount=%d,ExchangeRate=%d \n", eventType.FromUser.Hex(), eventType.ToUser.Hex(), eventType.FromChain, eventType.ToChain, eventType.Amount, eventType.ExchangeRate)
				case *LogBridgeFulfilledTokens:
					eventType.FromUser = common.HexToAddress(elog.Topics[1].Hex())
					eventData = eventType

					fmt.Printf("Processed Fulfilled Tokens Event: FromUser=%s,ToUser=%s,FromChain=%s,ToChain=%s,Amount=%d,ExchangeRate=%d \n", eventType.FromUser.Hex(), eventType.ToUser.Hex(), eventType.FromChain, eventType.ToChain, eventType.Amount, eventType.ExchangeRate)

				default:
					log.Printf("Unhandled event type: %T", eventType)
				}

				tx, isPending, err := b.Client.TransactionByHash(context.Background(), elog.TxHash)
				if err != nil {
					log.Printf("Failed to fetch transaction by hash: %v", err)
					continue
				}

				var fromAddress common.Address
				if !isPending {
					fromAddress, err = b.Client.TransactionSender(context.Background(), tx, elog.BlockHash, elog.TxIndex)
					if err != nil {
						log.Printf("Failed to fetch transaction sender: %v", err)
						continue
					}
				}

				ID, err := uuid.NewV7()
				if err != nil {
					log.Printf("Failed to generate event log uuid: %v", err)
					continue
				}
				blockNumber := elog.BlockNumber
				transactionHash := elog.TxHash.Hex()
				logIndex := elog.Index

				data := map[string]interface{}{
					"raw":       fmt.Sprintf("%x", elog.Data),
					"formatted": eventData,
				}

				dataJSON, err := json.Marshal(data)
				if err != nil {
					log.Printf("Failed to marshal event data: %v", err)
					continue
				}

				topicsJSON, err := json.Marshal(elog.Topics)
				if err != nil {
					log.Printf("Failed to marshal topics: %v", err)
					continue
				}

				eventlog := &models.EventLog{
					ID:              ID,
					ContractAddress: contract_event.Contract.Address,
					ContractEventID: contract_event.ID,
					EventName:       contract_event.Event.Name,
					ContractID:      contract_event.Contract.ID,
					BlockNumber:     blockNumber,
					TransactionHash: transactionHash,
					LogIndex:        logIndex,
					Data:            dataJSON,
					Topics:          topicsJSON,
				}
				eventLogs = append(eventLogs, eventlog)

				ActionID, err := uuid.NewV7()
				if err != nil {
					log.Printf("Failed to generate event log uuid: %v", err)
					continue
				}
				action := &models.UserAction{
					ID:              ActionID,
					Action:          contract_event.Event.Name,
					EventLogID:      eventlog.ID,
					TransactionHash: transactionHash,
					Point:           1,
				}
				userEvents[fromAddress.String()] = append(userEvents[fromAddress.String()], action)

			}

		}

	}
	return eventLogs, userEvents
}

func (b *BlockchainService) ProcessPoolSwapLogs(logs []types.Log, events []*models.ContractEvent) ([]*models.EventLog, map[string][]*models.UserAction) {
	eventLogs := []*models.EventLog{}

	eventStructMap := map[string]interface{}{
		"Swap": new(LogSwap),
	}
	contractAbi, err := abi.JSON(strings.NewReader(string(PoolSwap.PoolSwapABI)))
	if err != nil {
		log.Fatal(err)
	}

	userEvents := map[string][]*models.UserAction{}

	for _, elog := range logs {
		event_signature := elog.Topics[0].Hex()

		for _, contract_event := range events {

			if contract_event.Event.Signature == event_signature {

				t, ok := eventStructMap[contract_event.Event.Name]
				if !ok {
					log.Printf("Unknown event name: %s", contract_event.Event.Name)
					continue
				}

				err := contractAbi.UnpackIntoInterface(t, contract_event.Event.Name, elog.Data)
				if err != nil {
					log.Fatal(err)
				}

				var eventData interface{}

				switch eventType := t.(type) {
				case *LogSwap:
					eventType.Sender = common.HexToAddress(elog.Topics[1].Hex())
					eventType.Recipient = common.HexToAddress(elog.Topics[2].Hex())
					eventData = eventType
					fmt.Printf("Processed Swap event: Sender=%s,Recipient=%s,Amount0=%s,Amount1=%s,Liquidity=%d \n", eventType.Sender.Hex(), eventType.Recipient.Hex(), eventType.Amount0, eventType.Amount0, eventType.Liquidity)

				default:
					log.Printf("Unhandled event type: %T", eventType)
				}

				tx, isPending, err := b.Client.TransactionByHash(context.Background(), elog.TxHash)
				if err != nil {
					log.Printf("Failed to fetch transaction by hash: %v", err)
					continue
				}

				var fromAddress common.Address
				if !isPending { //TODO: handle pending transactions
					fromAddress, err = b.Client.TransactionSender(context.Background(), tx, elog.BlockHash, elog.TxIndex)
					if err != nil {
						log.Printf("Failed to fetch transaction sender: %v", err)
						continue
					}
				}

				ID, err := uuid.NewV7()
				if err != nil {
					log.Printf("Failed to generate event log uuid: %v", err)
					continue
				}
				blockNumber := elog.BlockNumber
				transactionHash := elog.TxHash.Hex()
				logIndex := elog.Index

				data := map[string]interface{}{
					"raw":       fmt.Sprintf("%x", elog.Data),
					"formatted": eventData,
				}

				dataJSON, err := json.Marshal(data)
				if err != nil {
					log.Printf("Failed to marshal event data: %v", err)
					continue
				}

				topicsJSON, err := json.Marshal(elog.Topics)
				if err != nil {
					log.Printf("Failed to marshal topics: %v", err)
					continue
				}

				eventlog := &models.EventLog{
					ID:              ID,
					ContractAddress: contract_event.Contract.Address,
					ContractEventID: contract_event.ID,
					EventName:       contract_event.Event.Name,
					ContractID:      contract_event.Contract.ID,
					BlockNumber:     blockNumber,
					TransactionHash: transactionHash,
					LogIndex:        logIndex,
					Data:            dataJSON,
					Topics:          topicsJSON,
				}
				eventLogs = append(eventLogs, eventlog)

				ActionID, err := uuid.NewV7()
				if err != nil {
					log.Printf("Failed to generate event log uuid: %v", err)
					continue
				}
				action := &models.UserAction{
					ID:              ActionID,
					Action:          contract_event.Event.Name,
					EventLogID:      eventlog.ID,
					TransactionHash: transactionHash,
					Point:           1,
				}
				userEvents[fromAddress.String()] = append(userEvents[fromAddress.String()], action)

			}

		}

	}
	return eventLogs, userEvents

}

func (b *BlockchainService) ProcessERCTokenLogs(logs []types.Log, events []*models.ContractEvent) ([]*models.EventLog, map[string][]*models.UserAction) {

	eventLogs := []*models.EventLog{}

	eventStructMap := map[string]interface{}{
		"Transfer": new(LogTransfer),
		"Approval": new(LogApproval),
	}

	contractAbi, err := abi.JSON(strings.NewReader(string(ERC20.ERC20ABI)))
	if err != nil {
		log.Fatal(err)
	}

	userEvents := map[string][]*models.UserAction{}

	for _, elog := range logs {
		event_signature := elog.Topics[0].Hex()
		for _, contract_event := range events {

			if contract_event.Event.Signature == event_signature {

				t, ok := eventStructMap[contract_event.Event.Name]
				if !ok {
					log.Printf("Unknown event name: %s", contract_event.Event.Name)
					continue
				}

				err := contractAbi.UnpackIntoInterface(t, contract_event.Event.Name, elog.Data)
				if err != nil {
					log.Fatal(err)
				}

				var eventData interface{}

				switch eventType := t.(type) {
				case *LogTransfer:
					eventType.From = common.HexToAddress(elog.Topics[1].Hex())
					eventType.To = common.HexToAddress(elog.Topics[2].Hex())

					eventData = eventType
					fmt.Printf("Processed Transfer event: From=%s, To=%s, Value=%s\n\n", eventType.From.Hex(), eventType.To.Hex(), eventType.Value.String())
				case *LogApproval:
					eventType.Owner = common.HexToAddress(elog.Topics[1].Hex())
					eventType.Spender = common.HexToAddress(elog.Topics[2].Hex())
					eventData = eventType
					fmt.Printf("Processed Approval event: Owner=%s, Spender=%s, Value=%s\n\n", eventType.Owner.Hex(), eventType.Spender.Hex(), eventType.Value.String())
				default:
					log.Printf("Unhandled event type: %T", eventType)
				}

				tx, isPending, err := b.Client.TransactionByHash(context.Background(), elog.TxHash)
				if err != nil {
					log.Printf("Failed to fetch transaction by hash: %v", err)
					continue
				}

				var fromAddress common.Address
				if !isPending { //TODO: handle pending transactions
					fromAddress, err = b.Client.TransactionSender(context.Background(), tx, elog.BlockHash, elog.TxIndex)
					if err != nil {
						log.Printf("Failed to fetch transaction sender: %v", err)
						continue
					}
				}

				// PUSH TO EVENT LOGS
				ID, err := uuid.NewV7()
				if err != nil {
					log.Printf("Failed to generate event log uuid: %v", err)
					continue
				}
				blockNumber := elog.BlockNumber
				transactionHash := elog.TxHash.Hex()
				logIndex := elog.Index

				data := map[string]interface{}{
					"raw":       fmt.Sprintf("%x", elog.Data),
					"formatted": eventData,
				}

				dataJSON, err := json.Marshal(data)
				if err != nil {
					log.Printf("Failed to marshal event data: %v", err)
					continue
				}

				topicsJSON, err := json.Marshal(elog.Topics)
				if err != nil {
					log.Printf("Failed to marshal topics: %v", err)
					continue
				}

				eventlog := &models.EventLog{
					ID:              ID,
					ContractAddress: contract_event.Contract.Address,
					ContractEventID: contract_event.ID,
					EventName:       contract_event.Event.Name,
					ContractID:      contract_event.Contract.ID,
					BlockNumber:     blockNumber,
					TransactionHash: transactionHash,
					LogIndex:        logIndex,
					Data:            dataJSON,
					Topics:          topicsJSON,
				}
				eventLogs = append(eventLogs, eventlog)

				ActionID, err := uuid.NewV7()
				if err != nil {
					log.Printf("Failed to generate event log uuid: %v", err)
					continue
				}
				action := &models.UserAction{
					ID:              ActionID,
					Action:          contract_event.Event.Name,
					EventLogID:      eventlog.ID,
					TransactionHash: transactionHash,
					Point:           1,
				}
				userEvents[fromAddress.String()] = append(userEvents[fromAddress.String()], action)

			}

		}

	}
	return eventLogs, userEvents
}

func (b *BlockchainService) GetTransactionByHash(hash string) {

}
