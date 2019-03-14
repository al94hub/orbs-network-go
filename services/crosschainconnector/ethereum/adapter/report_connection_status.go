package adapter

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"time"
)

type metrics struct {
	syncStatus              *metric.Text
	lastBlockHex            *metric.Text
	lastBlock               *metric.Text
	receiptsRetrievalStatus *metric.Text
}

const STATUS_FAILED = "failed"
const STATUS_SUCCESS = "success"
const STATUS_IN_PROGRESS = "in-progress"

const DEFAULT_BLOCK_NUMBER = "0"

const ARBITRARY_TXHASH = "0xb41e0591756bd1331de35eac3e3da460c9b3503d10e7bf08b84f057f489cd189"

func (c *EthereumRpcConnection) ReportConnectionStatus(ctx context.Context, registry metric.Registry, logger log.BasicLogger) {
	metrics := &metrics{
		syncStatus:              registry.NewText("Ethereum.Node.Sync.Status", STATUS_FAILED),
		lastBlock:               registry.NewText("Ethereum.Node.LastBlock", DEFAULT_BLOCK_NUMBER),
		receiptsRetrievalStatus: registry.NewText("Ethereum.Node.TransactionReceipts.Status", STATUS_FAILED),
	}

	synchronization.NewPeriodicalTrigger(ctx, 30*time.Second, logger, func() {
		if receipt, err := c.Receipt(common.HexToHash(ARBITRARY_TXHASH)); err != nil {
			logger.Info("ethereum rpc connection status check failed", log.Error(err))
			metrics.receiptsRetrievalStatus.Update(STATUS_FAILED)
		} else if len(receipt.Logs) > 0 {
			metrics.receiptsRetrievalStatus.Update(STATUS_SUCCESS)
		} else {
			metrics.receiptsRetrievalStatus.Update(STATUS_FAILED)
		}

		if syncStatus, err := c.SyncProgress(); err != nil {
			logger.Info("ethereum rpc connection status check failed", log.Error(err))
			metrics.syncStatus.Update(STATUS_FAILED)
		} else if syncStatus == nil {
			metrics.syncStatus.Update(STATUS_SUCCESS)
		} else {
			metrics.syncStatus.Update(STATUS_IN_PROGRESS)
		}

		if header, err := c.HeaderByNumber(ctx, nil); err != nil {
			logger.Info("ethereum rpc connection status check failed", log.Error(err))
			metrics.lastBlock.Update(DEFAULT_BLOCK_NUMBER)
		} else {
			metrics.lastBlock.Update(header.Number.String())
		}
	}, nil)
}