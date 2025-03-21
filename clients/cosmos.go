package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/Layer-Edge/light-node/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientConfig holds all configurable parameters for the clients package
type ClientConfig struct {
	GrpcURL        string
	ContractAddr   string
	// Retry configuration
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	// Connection timeout
	ConnectionTimeout time.Duration
}

// Global configuration with default values
var globalClientConfig = ClientConfig{
	GrpcURL:           "34.57.133.111:9090",                                                 // Default gRPC endpoint
	ContractAddr:      "cosmos1ufs3tlq4umljk0qfe8k5ya0x6hpavn897u2cnf9k0en9jr7qarqqt56709", // Default contract address
	MaxRetries:        -1,                   // -1 means retry indefinitely
	InitialBackoff:    30 * time.Second,      // Start with 30 second backoff
	MaxBackoff:        10 * time.Minute,     // Maximum backoff of 10 minutes
	ConnectionTimeout: 10 * time.Second,     // Connection verification timeout
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
	return cqc.connect()
}

// InitWithConfig initializes the client with a specific configuration
func (cqc *CosmosQueryClient) InitWithConfig(config ClientConfig) error {
	cqc.config = config
	return cqc.connect()
}

// verifyConnection checks if the connection is actually usable by making a test query
func (cqc *CosmosQueryClient) verifyConnection(conn *grpc.ClientConn) error {
	// Create a deadline for connection verification
	ctx, cancel := context.WithTimeout(context.Background(), cqc.config.ConnectionTimeout)
	defer cancel()

	// Wait for connection to become ready with a timeout
	state := conn.GetState()
	if state != connectivity.Ready {
		if !conn.WaitForStateChange(ctx, state) {
			return fmt.Errorf("connection timed out waiting to become ready, current state: %s", state.String())
		}
	}
	
	// Try to make a simple query to verify the connection works
	queryClient := wasmtypes.NewQueryClient(conn)
	_, err := queryClient.ContractInfo(
		ctx,
		&wasmtypes.QueryContractInfoRequest{
			Address: cqc.config.ContractAddr,
		},
	)
	
	if err != nil {
		return fmt.Errorf("connection verification failed: %v", err)
	}
	
	return nil
}

// connect attempts to establish a connection with exponential backoff retry
func (cqc *CosmosQueryClient) connect() error {
	backoff := cqc.config.InitialBackoff
	attempt := 0

	for {
		// Try to connect
		log.Printf("Attempting to connect to gRPC at %s (attempt %d)", cqc.config.GrpcURL, attempt+1)
		
		// Create connection
		conn, err := grpc.Dial(
			cqc.config.GrpcURL,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(), // Makes Dial block until a connection is established
			grpc.WithTimeout(cqc.config.ConnectionTimeout), // Timeout for initial connection
		)
		
		if err == nil {
			// Verify connection is actually usable
			err = cqc.verifyConnection(conn)
			if err == nil {
				// Connection successful and verified
				cqc.conn = conn
				cqc.queryClient = wasmtypes.NewQueryClient(conn)
				log.Printf("Successfully connected to gRPC at %s", cqc.config.GrpcURL)
				return nil
			}
			// Connection verification failed, close it and retry
			conn.Close()
			log.Printf("Connection established but verification failed: %v", err)
		}
		
		attempt++
		
		// Check if max retries reached (if not set to infinite)
		if cqc.config.MaxRetries > 0 && attempt >= cqc.config.MaxRetries {
			return fmt.Errorf("failed to connect to gRPC at %s after %d attempts: %v", 
				cqc.config.GrpcURL, attempt, err)
		}
		
		// Calculate next backoff with exponential increase, but capped at max
		backoff = time.Duration(math.Min(
			float64(backoff)*2, 
			float64(cqc.config.MaxBackoff),
		))
		
		log.Printf("Connection failed: %v. Retrying in %v...", err, backoff)
		time.Sleep(backoff)
	}
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