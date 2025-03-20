package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/Layer-Edge/light-node/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientConfig holds all configurable parameters for the clients package
type ClientConfig struct {
	GrpcURL      string
	ContractAddr string
}

// Global configuration with default values
var globalClientConfig = ClientConfig{
	GrpcURL:      "34.57.133.111:9090",                                                 // Default gRPC endpoint
	ContractAddr: "cosmos1ufs3tlq4umljk0qfe8k5ya0x6hpavn897u2cnf9k0en9jr7qarqqt56709", // Default contract address
}

// InitClientConfig initializes the client configuration with environment variables or defaults
func InitClientConfig() {
	globalClientConfig.GrpcURL = utils.GetEnv("GRPC_URL", "0.0.0.0:9090")
	globalClientConfig.ContractAddr = utils.GetEnv("CONTRACT_ADDR", "cosmos1ufs3tlq4umljk0qfe8k5ya0x6hpavn897u2cnf9k0en9jr7qarqqt56709")

	log.Printf("Initialized client configuration: GRPC_URL=%s, CONTRACT_ADDR=%s",
		globalClientConfig.GrpcURL, globalClientConfig.ContractAddr)
}

// SetClientConfig allows overriding the configuration programmatically
func SetClientConfig(config ClientConfig) {
	globalClientConfig = config
	log.Printf("Updated client configuration: GRPC_URL=%s, CONTRACT_ADDR=%s",
		globalClientConfig.GrpcURL, globalClientConfig.ContractAddr)
}

// GetClientConfig returns a copy of the current configuration
func GetClientConfig() ClientConfig {
	return globalClientConfig
}

type MerkleTree struct {
	Root     string   `json:"root"`
	Leaves   []string `json:"leaves"`
	Metadata string   `json:"metadata"`
}

type QueryGetTree struct {
	GetMerkleTree struct {
		ID string `json:"id"`
	} `json:"get_merkle_tree"`
}

type QueryListTreeIDs struct {
	ListMerkleTreeIds struct {
	} `json:"list_merkle_tree_ids"`
}

type CosmosQueryClient struct {
	conn        *grpc.ClientConn
	queryClient wasmtypes.QueryClient
	config      ClientConfig
}

func (cqc *CosmosQueryClient) Init() error {
	// Use the global configuration
	cqc.config = globalClientConfig

	// Connect to gRPC client
	conn, err := grpc.Dial(cqc.config.GrpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC at %s: %v", cqc.config.GrpcURL, err)
	}

	cqc.conn = conn
	cqc.queryClient = wasmtypes.NewQueryClient(conn)
	return nil
}

// InitWithConfig initializes the client with a specific configuration
func (cqc *CosmosQueryClient) InitWithConfig(config ClientConfig) error {
	cqc.config = config

	// Connect to gRPC client
	conn, err := grpc.Dial(cqc.config.GrpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC at %s: %v", cqc.config.GrpcURL, err)
	}

	cqc.conn = conn
	cqc.queryClient = wasmtypes.NewQueryClient(conn)
	return nil
}

func (cqc *CosmosQueryClient) Close() {
	if cqc.conn != nil {
		cqc.conn.Close()
	}
}

func (cqc *CosmosQueryClient) GetMerkleTreeData(id string) (*MerkleTree, error) {
	query := QueryGetTree{}
	query.GetMerkleTree.ID = id

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %v", err)
	}

	res, err := cqc.queryClient.SmartContractState(
		context.Background(),
		&wasmtypes.QuerySmartContractStateRequest{
			Address:   cqc.config.ContractAddr,
			QueryData: queryBytes,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query contract: %v", err)
	}

	// Parse response JSON into struct
	var tree MerkleTree
	err = json.Unmarshal(res.Data, &tree)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal tree data: %v", err)
	}

	return &tree, nil
}

func (cqc *CosmosQueryClient) ListMerkleTreeIds() ([]string, error) {
	query := QueryListTreeIDs{}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %v", err)
	}

	res, err := cqc.queryClient.SmartContractState(
		context.Background(),
		&wasmtypes.QuerySmartContractStateRequest{
			Address:   cqc.config.ContractAddr,
			QueryData: queryBytes,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query contract: %v", err)
	}

	// Parse response JSON into struct
	var treeIds []string
	err = json.Unmarshal(res.Data, &treeIds)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal tree data: %v", err)
	}
	return treeIds, nil
}
