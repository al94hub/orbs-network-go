package config

import (
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol/consensus"
	"time"
	"github.com/orbs-network/orbs-network-go/test/crypto/keys"
)

//TODO introduce FileSystemConfig

type identity struct {
	nodePublicKey  primitives.Ed25519PublicKey
	nodePrivateKey primitives.Ed25519PrivateKey
	virtualChainId primitives.VirtualChainId
}

type consensusConfig struct {
	*identity
	federationNodes                            map[string]FederationNode
	constantConsensusLeader                    primitives.Ed25519PublicKey
	activeConsensusAlgo                        consensus.ConsensusAlgoType
	benchmarkConsensusRoundRetryIntervalMillis uint32
}

type crossServiceConfig struct {
	queryGraceTimeoutMillis uint64
	querySyncGraceBlockDist uint16
}

type blockStorageConfig struct {
	blockSyncCommitTimeoutMillis                     time.Duration
	blockTransactionReceiptQueryStartGraceSec        time.Duration
	blockTransactionReceiptQueryEndGraceSec          time.Duration
	blockTransactionReceiptQueryTransactionExpireSec time.Duration
}

type consensusContextConfig struct {
	belowMinimalBlockDelayMillis uint32
	minimumTransactionsInBlock   int
}

type stateStorageConfig struct {
	*crossServiceConfig
	stateHistoryRetentionInBlockHeights uint16
}

type transactionPoolConfig struct {
	*identity
	*crossServiceConfig
	pendingPoolSizeInBytes            uint32
	transactionExpirationWindow       time.Duration
	futureTimestampGrace              time.Duration
	pendingPoolClearExpiredInterval   time.Duration
	committedPoolClearExpiredInterval time.Duration
}

type hardCodedFederationNode struct {
	nodePublicKey primitives.Ed25519PublicKey
}

type hardcodedConfig struct {
	*identity
	*consensusConfig
	*crossServiceConfig
	*blockStorageConfig
	*stateStorageConfig
	*consensusContextConfig
	*transactionPoolConfig
}

func NewHardCodedFederationNode(nodePublicKey primitives.Ed25519PublicKey) FederationNode {
	return &hardCodedFederationNode{
		nodePublicKey: nodePublicKey,
	}
}

func NewHardCodedConfig(
	federationNodes map[string]FederationNode,
	nodePublicKey primitives.Ed25519PublicKey,
	nodePrivateKey primitives.Ed25519PrivateKey,
	constantConsensusLeader primitives.Ed25519PublicKey,
	activeConsensusAlgo consensus.ConsensusAlgoType,
	benchmarkConsensusRoundRetryIntervalMillis uint32,
	blockSyncCommitTimeoutMillis uint32,
	minimumTransactionsInBlock int,
) NodeConfig {

	return &hardcodedConfig{
		identity: &identity{
			nodePublicKey:  nodePublicKey,
			nodePrivateKey: nodePrivateKey,
			virtualChainId: 42,
		},
		consensusConfig: &consensusConfig{
			federationNodes:                            federationNodes,
			constantConsensusLeader:                    constantConsensusLeader,
			activeConsensusAlgo:                        activeConsensusAlgo,
			benchmarkConsensusRoundRetryIntervalMillis: benchmarkConsensusRoundRetryIntervalMillis,
		},
		crossServiceConfig: &crossServiceConfig{
			queryGraceTimeoutMillis: 300,
			querySyncGraceBlockDist: 3,
		},
		blockStorageConfig: &blockStorageConfig{
			blockSyncCommitTimeoutMillis:                     time.Duration(blockSyncCommitTimeoutMillis) * time.Millisecond,
			blockTransactionReceiptQueryStartGraceSec:        time.Duration(5) * time.Second,
			blockTransactionReceiptQueryEndGraceSec:          time.Duration(5) * time.Second,
			blockTransactionReceiptQueryTransactionExpireSec: time.Duration(180) * time.Second,
		},
		stateStorageConfig: &stateStorageConfig{
			stateHistoryRetentionInBlockHeights: 5,
		},
		consensusContextConfig: &consensusContextConfig{
			belowMinimalBlockDelayMillis: 300,
			minimumTransactionsInBlock:   minimumTransactionsInBlock,
		},
		transactionPoolConfig: &transactionPoolConfig{
			pendingPoolSizeInBytes:            20 * 1024 * 1024,
			futureTimestampGrace:              3 * time.Minute,
			transactionExpirationWindow:       30 * time.Minute,
			pendingPoolClearExpiredInterval:   10 * time.Second,
			committedPoolClearExpiredInterval: 30 * time.Second,
		},
	}
}

func NewConsensusConfig(
	federationNodes map[string]FederationNode,
	nodePublicKey primitives.Ed25519PublicKey,
	nodePrivateKey primitives.Ed25519PrivateKey,
	constantConsensusLeader primitives.Ed25519PublicKey,
	activeConsensusAlgo consensus.ConsensusAlgoType,
	benchmarkConsensusRoundRetryIntervalMillis uint32,
) *consensusConfig {

	return &consensusConfig{
		identity: &identity{
			nodePublicKey:  nodePublicKey,
			nodePrivateKey: nodePrivateKey,
			virtualChainId: 42,
		},
		federationNodes:                            federationNodes,
		constantConsensusLeader:                    constantConsensusLeader,
		activeConsensusAlgo:                        activeConsensusAlgo,
		benchmarkConsensusRoundRetryIntervalMillis: benchmarkConsensusRoundRetryIntervalMillis,
	}
}

func NewBlockStorageConfig(blockSyncCommitTimeoutMillis, blockTransactionReceiptQueryStartGraceSec, blockTransactionReceiptQueryEndGraceSec, blockTransactionReceiptQueryTransactionExpireSec uint32) *blockStorageConfig {
	return &blockStorageConfig{
		blockSyncCommitTimeoutMillis:                     time.Duration(blockSyncCommitTimeoutMillis) * time.Millisecond,
		blockTransactionReceiptQueryStartGraceSec:        time.Duration(blockTransactionReceiptQueryStartGraceSec) * time.Second,
		blockTransactionReceiptQueryEndGraceSec:          time.Duration(blockTransactionReceiptQueryEndGraceSec) * time.Second,
		blockTransactionReceiptQueryTransactionExpireSec: time.Duration(blockTransactionReceiptQueryTransactionExpireSec) * time.Second,
	}
}

func NewConsensusContextConfig(belowMinimalBlockDelayMillis uint32, minimumTransactionsInBlock int) *consensusContextConfig {
	return &consensusContextConfig{
		belowMinimalBlockDelayMillis: belowMinimalBlockDelayMillis,
		minimumTransactionsInBlock:   minimumTransactionsInBlock,
	}
}

func NewTransactionPoolConfig(pendingPoolSizeInBytes uint32, transactionExpirationWindow time.Duration, nodeKeyPair *keys.Ed25519KeyPair) *transactionPoolConfig {
	return &transactionPoolConfig{
		identity: &identity{
			nodePublicKey:  nodeKeyPair.PublicKey(),
			nodePrivateKey:  nodeKeyPair.PrivateKey(),
			virtualChainId: 42,
		},
		crossServiceConfig: &crossServiceConfig{
			queryGraceTimeoutMillis: 100,
			querySyncGraceBlockDist: 5,
		},
		pendingPoolSizeInBytes:            pendingPoolSizeInBytes,
		futureTimestampGrace:              3 * time.Minute,
		transactionExpirationWindow:       transactionExpirationWindow,
		pendingPoolClearExpiredInterval:   10 * time.Second,
		committedPoolClearExpiredInterval: 30 * time.Second,
	}
}

func NewStateStorageConfig(maxStateHistory uint16, graceBlockDist uint16, graceTimeoutMillis uint64) *stateStorageConfig {
	return &stateStorageConfig{
		stateHistoryRetentionInBlockHeights: maxStateHistory,
		crossServiceConfig: &crossServiceConfig{
			queryGraceTimeoutMillis: graceTimeoutMillis,
			querySyncGraceBlockDist: graceBlockDist,
		},
	}
}

func (c *identity) NodePublicKey() primitives.Ed25519PublicKey {
	return c.nodePublicKey
}

func (c *identity) NodePrivateKey() primitives.Ed25519PrivateKey {
	return c.nodePrivateKey
}

func (c *identity) VirtualChainId() primitives.VirtualChainId {
	return c.virtualChainId
}

func (c *consensusConfig) NetworkSize(asOfBlock uint64) uint32 {
	return uint32(len(c.federationNodes))
}

func (c *consensusConfig) FederationNodes(asOfBlock uint64) map[string]FederationNode {
	return c.federationNodes
}

func (c *consensusConfig) ConstantConsensusLeader() primitives.Ed25519PublicKey {
	return c.constantConsensusLeader
}

func (c *consensusConfig) ActiveConsensusAlgo() consensus.ConsensusAlgoType {
	return c.activeConsensusAlgo
}

func (c *consensusConfig) BenchmarkConsensusRoundRetryIntervalMillis() uint32 {
	return c.benchmarkConsensusRoundRetryIntervalMillis
}

func (n *hardCodedFederationNode) NodePublicKey() primitives.Ed25519PublicKey {
	return n.nodePublicKey
}

func (c *blockStorageConfig) BlockSyncCommitTimeoutMillis() time.Duration {
	return c.blockSyncCommitTimeoutMillis
}

func (c *blockStorageConfig) BlockTransactionReceiptQueryStartGraceSec() time.Duration {
	return c.blockTransactionReceiptQueryStartGraceSec
}
func (c *blockStorageConfig) BlockTransactionReceiptQueryEndGraceSec() time.Duration {
	return c.blockTransactionReceiptQueryEndGraceSec
}
func (c *blockStorageConfig) BlockTransactionReceiptQueryTransactionExpireSec() time.Duration {
	return c.blockTransactionReceiptQueryTransactionExpireSec
}

func (c *consensusContextConfig) BelowMinimalBlockDelayMillis() uint32 {
	return c.belowMinimalBlockDelayMillis
}

func (c *consensusContextConfig) MinimumTransactionsInBlock() int {
	return c.minimumTransactionsInBlock
}

func (c *stateStorageConfig) StateHistoryRetentionInBlockHeights() uint16 {
	return c.stateHistoryRetentionInBlockHeights
}

func (c *crossServiceConfig) QuerySyncGraceBlockDist() uint16 {
	return c.querySyncGraceBlockDist
}

func (c *crossServiceConfig) QueryGraceTimeoutMillis() uint64 {
	return c.queryGraceTimeoutMillis
}

func (c *transactionPoolConfig) PendingPoolSizeInBytes() uint32 {
	return c.pendingPoolSizeInBytes
}

func (c *transactionPoolConfig) TransactionExpirationWindow() time.Duration {
	return c.transactionExpirationWindow
}

func (c *transactionPoolConfig) FutureTimestampGrace() time.Duration {
	return c.futureTimestampGrace
}

func (c *transactionPoolConfig) PendingPoolClearExpiredInterval() time.Duration {
	return c.pendingPoolClearExpiredInterval
}

func (c *transactionPoolConfig) CommittedPoolClearExpiredInterval() time.Duration {
	return c.committedPoolClearExpiredInterval
}
