package consensuscontext

import (
	"bytes"
	"context"
	"github.com/orbs-network/orbs-network-go/crypto/digest"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/pkg/errors"
)

type rxValidator func(ctx context.Context, vcrx *rxValidatorContext) error

type GetStateHashAdapter interface {
	GetStateHash(ctx context.Context, input *services.GetStateHashInput) (*services.GetStateHashOutput, error)
}

type ProcessTransactionSetAdapter interface {
	ProcessTransactionSet(ctx context.Context, input *services.ProcessTransactionSetInput) (*services.ProcessTransactionSetOutput, error)
}

type CalculateReceiptsMerkleRootAdapter interface {
	CalculateReceiptsMerkleRoot(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error)
}

type CalculateStateDiffMerkleRootAdapter interface {
	CalculateStateDiffMerkleRoot(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error)
}

type rxValidatorContext struct {
	protocolVersion                     primitives.ProtocolVersion
	virtualChainId                      primitives.VirtualChainId
	input                               *services.ValidateResultsBlockInput
	getStateHashAdapter                 GetStateHashAdapter
	processTransactionSetAdapter        ProcessTransactionSetAdapter
	calculateReceiptsMerkleRootAdapter  CalculateReceiptsMerkleRootAdapter
	calculateStateDiffMerkleRootAdapter CalculateStateDiffMerkleRootAdapter
}

func validateRxProtocolVersion(ctx context.Context, vcrx *rxValidatorContext) error {
	expectedProtocolVersion := vcrx.protocolVersion
	checkedProtocolVersion := vcrx.input.ResultsBlock.Header.ProtocolVersion()
	if checkedProtocolVersion != expectedProtocolVersion {
		return errors.Wrapf(ErrMismatchedProtocolVersion, "expected %v actual %v", expectedProtocolVersion, checkedProtocolVersion)
	}
	return nil
}

func validateRxVirtualChainID(ctx context.Context, vcrx *rxValidatorContext) error {
	expectedVirtualChainId := vcrx.virtualChainId
	checkedVirtualChainId := vcrx.input.ResultsBlock.Header.VirtualChainId()
	if checkedVirtualChainId != expectedVirtualChainId {
		return errors.Wrapf(ErrMismatchedVirtualChainID, "expected %v actual %v", expectedVirtualChainId, checkedVirtualChainId)
	}
	return nil
}

func validateRxBlockHeight(ctx context.Context, vcrx *rxValidatorContext) error {
	expectedBlockHeight := vcrx.input.CurrentBlockHeight
	checkedBlockHeight := vcrx.input.ResultsBlock.Header.BlockHeight()
	if checkedBlockHeight != expectedBlockHeight {
		return errors.Wrapf(ErrMismatchedBlockHeight, "expected %v actual %v", expectedBlockHeight, checkedBlockHeight)
	}
	txBlockHeight := vcrx.input.TransactionsBlock.Header.BlockHeight()
	if checkedBlockHeight != txBlockHeight {
		return errors.Wrapf(ErrMismatchedTxRxBlockHeight, "txBlock %v rxBlock %v", txBlockHeight, checkedBlockHeight)
	}
	return nil
}

func validateRxTxBlockPtrMatchesActualTxBlock(ctx context.Context, vcrx *rxValidatorContext) error {
	txBlockHashPtr := vcrx.input.ResultsBlock.Header.TransactionsBlockHashPtr()
	expectedTxBlockHashPtr := digest.CalcTransactionsBlockHash(vcrx.input.TransactionsBlock)
	if !bytes.Equal(txBlockHashPtr, expectedTxBlockHashPtr) {
		return errors.Wrapf(ErrMismatchedTxHashPtrToActualTxBlock, "expected %v actual %v", expectedTxBlockHashPtr, txBlockHashPtr)
	}
	return nil
}

func validateIdenticalTxRxTimestamp(ctx context.Context, vcrx *rxValidatorContext) error {
	txTimestamp := vcrx.input.TransactionsBlock.Header.Timestamp()
	rxTimestamp := vcrx.input.ResultsBlock.Header.Timestamp()
	if rxTimestamp != txTimestamp {
		return errors.Wrapf(ErrMismatchedTxRxTimestamps, "txTimestamp %v rxTimestamp %v", txTimestamp, rxTimestamp)
	}
	return nil
}

func validateRxPrevBlockHashPtr(ctx context.Context, vcrx *rxValidatorContext) error {
	prevBlockHashPtr := vcrx.input.ResultsBlock.Header.PrevBlockHashPtr()
	expectedPrevBlockHashPtr := vcrx.input.PrevBlockHash
	if !bytes.Equal(prevBlockHashPtr, expectedPrevBlockHashPtr) {
		return errors.Wrapf(ErrMismatchedPrevBlockHash, "expected %v actual %v", expectedPrevBlockHashPtr, prevBlockHashPtr)
	}
	return nil
}

func validateRxReceiptsRootHash(ctx context.Context, vcrx *rxValidatorContext) error {
	expectedReceiptsMerkleRoot := vcrx.input.ResultsBlock.Header.ReceiptsMerkleRootHash()
	calculatedReceiptMerkleRoot, err := vcrx.calculateReceiptsMerkleRootAdapter.CalculateReceiptsMerkleRoot(vcrx.input.ResultsBlock.TransactionReceipts)
	if err != nil {
		return errors.Wrapf(ErrCalculateReceiptsMerkleRoot, "ValidateResultsBlock error calculateReceiptsMerkleRoot(), %v", err)
	}
	if !bytes.Equal(expectedReceiptsMerkleRoot, []byte(calculatedReceiptMerkleRoot)) {
		return errors.Wrapf(ErrMismatchedReceiptsRootHash, "expected %v actual %v", expectedReceiptsMerkleRoot, calculatedReceiptMerkleRoot)
	}
	return nil
}

func validateRxStateDiffHash(ctx context.Context, vcrx *rxValidatorContext) error {
	expectedStateDiffMerkleRoot := vcrx.input.ResultsBlock.Header.StateDiffHash()
	calculatedStateDiffMerkleRoot, err := vcrx.calculateStateDiffMerkleRootAdapter.CalculateStateDiffMerkleRoot(vcrx.input.ResultsBlock.ContractStateDiffs)
	if err != nil {
		return errors.Wrapf(ErrCalculateStateDiffMerkleRoot, "ValidateResultsBlock error calculateStateDiffMerkleRoot(), %v", err)
	}
	if !bytes.Equal(expectedStateDiffMerkleRoot, []byte(calculatedStateDiffMerkleRoot)) {
		return errors.Wrapf(ErrMismatchedStateDiffHash, "expected %v actual %v", expectedStateDiffMerkleRoot, calculatedStateDiffMerkleRoot)
	}
	return nil
}

func validatePreExecutionStateMerkleRoot(ctx context.Context, vcrx *rxValidatorContext) error {
	expectedPreExecutionMerkleRoot := vcrx.input.ResultsBlock.Header.PreExecutionStateMerkleRootHash()
	getStateHashOut, err := vcrx.getStateHashAdapter.GetStateHash(ctx, &services.GetStateHashInput{
		BlockHeight: vcrx.input.ResultsBlock.Header.BlockHeight() - 1,
	})
	if err != nil {
		return errors.Wrapf(ErrGetStateHash, "ValidateResultsBlock.validatePreExecutionStateMerkleRoot() error GetStateHash(), %v", err)
	}
	if !bytes.Equal(expectedPreExecutionMerkleRoot, getStateHashOut.StateMerkleRootHash) {
		return errors.Wrapf(ErrMismatchedPreExecutionStateMerkleRoot, "expected %v actual %v", expectedPreExecutionMerkleRoot, getStateHashOut.StateMerkleRootHash)
	}
	return nil
}

// s.virtualMachine.ProcessTransactionSet
func validateExecution(ctx context.Context, vcrx *rxValidatorContext) error {
	//Validate transaction execution
	// Execute the ordered transactions set by calling VirtualMachine.ProcessTransactionSet creating receipts and state diff. Using the provided header timestamp as a reference timestamp.
	processTxsOut, err := vcrx.processTransactionSetAdapter.ProcessTransactionSet(ctx, &services.ProcessTransactionSetInput{ // TODO wrap with adapter
		CurrentBlockHeight:    vcrx.input.TransactionsBlock.Header.BlockHeight(),
		CurrentBlockTimestamp: vcrx.input.TransactionsBlock.Header.Timestamp(),
		SignedTransactions:    vcrx.input.TransactionsBlock.SignedTransactions,
	})
	if err != nil {
		return errors.Wrapf(ErrProcessTransactionSet, "ValidateResultsBlock.validateExecution() error ProcessTransactionSet")
	}
	// Compare the receipts merkle root hash to the one in the block.
	expectedReceiptsMerkleRoot := vcrx.input.ResultsBlock.Header.ReceiptsMerkleRootHash()
	calculatedReceiptMerkleRoot, err := vcrx.calculateReceiptsMerkleRootAdapter.CalculateReceiptsMerkleRoot(processTxsOut.TransactionReceipts) // TODO wrap with adapter
	if err != nil {
		return errors.Wrapf(ErrCalculateReceiptsMerkleRoot, "ValidateResultsBlock error ProcessTransactionSet calculateReceiptsMerkleRoot")
	}
	if !bytes.Equal(expectedReceiptsMerkleRoot, calculatedReceiptMerkleRoot) {
		return errors.Wrapf(ErrMismatchedReceiptsRootHash, "ValidateResultsBlock error receipt merkleRoot in header does not match processed txs receipts")
	}

	// Compare the state diff hash to the one in the block (supports only deterministic execution).
	expectedStateDiffMerkleRoot := vcrx.input.ResultsBlock.Header.RawStateDiffHash()
	calculatedStateDiffMerkleRoot, err := vcrx.calculateStateDiffMerkleRootAdapter.CalculateStateDiffMerkleRoot(processTxsOut.ContractStateDiffs) // TODO wrap with adapter
	if err != nil {
		return errors.Wrapf(ErrCalculateStateDiffMerkleRoot, "ValidateResultsBlock error ProcessTransactionSet calculateStateDiffMerkleRoot")
	}
	if !bytes.Equal(expectedStateDiffMerkleRoot, calculatedStateDiffMerkleRoot) {
		return errors.Wrapf(ErrMismatchedStateDiffHash, "expected %v actual %v", expectedStateDiffMerkleRoot, calculatedStateDiffMerkleRoot)
	}

	return nil
}

type realGetStateHashAdapter struct {
	getStateHash func(ctx context.Context, input *services.GetStateHashInput) (*services.GetStateHashOutput, error)
}

func (r *realGetStateHashAdapter) GetStateHash(ctx context.Context, input *services.GetStateHashInput) (*services.GetStateHashOutput, error) {
	return r.getStateHash(ctx, input)
}
func NewRealGetStateHashAdapter(f func(ctx context.Context, input *services.GetStateHashInput) (*services.GetStateHashOutput, error)) GetStateHashAdapter {
	return &realGetStateHashAdapter{
		getStateHash: f,
	}
}

type realProcessTransactionSetAdapter struct {
	processTransactionSet func(ctx context.Context, input *services.ProcessTransactionSetInput) (*services.ProcessTransactionSetOutput, error)
}

func (r *realProcessTransactionSetAdapter) ProcessTransactionSet(ctx context.Context, input *services.ProcessTransactionSetInput) (*services.ProcessTransactionSetOutput, error) {
	return r.processTransactionSet(ctx, input)
}
func NewRealProcessTransactionSetAdapter(f func(ctx context.Context, input *services.ProcessTransactionSetInput) (*services.ProcessTransactionSetOutput, error)) ProcessTransactionSetAdapter {
	return &realProcessTransactionSetAdapter{
		processTransactionSet: f,
	}
}

type realCalculateReceiptsMerkleRootAdapter struct {
	calculateReceiptsMerkleRoot func(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error)
}

func (r *realCalculateReceiptsMerkleRootAdapter) CalculateReceiptsMerkleRoot(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error) {
	return r.calculateReceiptsMerkleRoot(receipts)
}
func NewRealCalculateReceiptsMerkleRootAdapter(f func(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error)) CalculateReceiptsMerkleRootAdapter {
	return &realCalculateReceiptsMerkleRootAdapter{
		calculateReceiptsMerkleRoot: f,
	}
}

type realCalculateStateDiffMerkleRootAdapter struct {
	calculateStateDiffMerkleRoot func(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error)
}

func (r *realCalculateStateDiffMerkleRootAdapter) CalculateStateDiffMerkleRoot(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error) {
	return r.CalculateStateDiffMerkleRoot(stateDiffs)
}
func NewRealCalculateStateDiffMerkleRootAdapter(f func(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error)) CalculateStateDiffMerkleRootAdapter {
	return &realCalculateStateDiffMerkleRootAdapter{
		calculateStateDiffMerkleRoot: f,
	}
}

//processTransactionSet func(ctx context.Context, input *services.ProcessTransactionSetInput) (*services.ProcessTransactionSetOutput, error)
//CalculateReceiptsMerkleRoot(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error)
//CalculateStateDiffMerkleRoot(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error)

func (s *service) ValidateResultsBlock(ctx context.Context, input *services.ValidateResultsBlockInput) (*services.ValidateResultsBlockOutput, error) {

	vcrx := &rxValidatorContext{
		protocolVersion:                     s.config.ProtocolVersion(),
		virtualChainId:                      s.config.VirtualChainId(),
		input:                               input,
		getStateHashAdapter:                 NewRealGetStateHashAdapter(s.stateStorage.GetStateHash),
		processTransactionSetAdapter:        NewRealProcessTransactionSetAdapter(s.virtualMachine.ProcessTransactionSet),
		calculateReceiptsMerkleRootAdapter:  NewRealCalculateReceiptsMerkleRootAdapter(calculateReceiptsMerkleRoot),
		calculateStateDiffMerkleRootAdapter: NewRealCalculateStateDiffMerkleRootAdapter(calculateStateDiffMerkleRoot),
	}

	validators := []rxValidator{
		validateRxProtocolVersion,
		validateRxVirtualChainID,
		validateRxBlockHeight,
		validateRxTxBlockPtrMatchesActualTxBlock,
		validateIdenticalTxRxTimestamp,
		validateRxPrevBlockHashPtr,
		validateRxReceiptsRootHash,
		validateRxStateDiffHash,
		validatePreExecutionStateMerkleRoot,
		validateExecution,
	}

	for _, v := range validators {
		if err := v(ctx, vcrx); err != nil {
			return &services.ValidateResultsBlockOutput{}, err
		}
	}
	return &services.ValidateResultsBlockOutput{}, nil
}

/*

//Check Results block header
// Check protocol version, virtual chain,
// Check header's block height matches the provided one.
// Check timestamp equals the TransactionsBlock timestamp.
// Check hash pointer indeed matches the given previous block hash.
// Check the receipts merkle root matches the receipts.
// Check the hash of the state diff in the block.
// Check hash pointer to the Transactions block of the same height.
// Check merkle root of the state prior to the block execution, retrieved by calling StateStorage.GetStateHash.
// bloom filter currently not supported.
// Assume TxBlock validated prior to RxBlock
//Validate transaction execution
// Execute the ordered transactions set by calling VirtualMachine.ProcessTransactionSet creating receipts and state diff.
// Using the provided header timestamp as a reference timestamp.
// Compare the receipts merkle root hash to the one in the block.
// Compare the state diff hash to the one in the block (supports only deterministic execution).
func (s *service) ValidateResultsBlock(ctx context.Context, input *services.ValidateResultsBlockInput) (*services.ValidateResultsBlockOutput, error) {

	//Check protocol version (config), virtual chain (config), block height (provided), timestamp (TransactionsBlock)
	checkedBlockHeigt := input.ResultsBlock.Header.BlockHeight()
	expectedBlockHeigt := input.BlockHeight
	if checkedBlockHeigt != expectedBlockHeigt {
		return nil, errors.Errorf("ValidateResultsBlock mismatching blockHeight: expected %v actual %v", expectedBlockHeigt, checkedBlockHeigt)
	}
	txBlockHeight := input.TransactionsBlock.Header.BlockHeight()
	if checkedBlockHeigt != txBlockHeight {
		return nil, fmt.Errorf("ValidateResultsBlock mismatching block height: txBlock=%v rxBlock=%v", txBlockHeight, checkedBlockHeigt)
	}

	checkedProtocolVersion := input.ResultsBlock.Header.ProtocolVersion()
	expectedProtocolVersion := s.config.ProtocolVersion()
	if checkedProtocolVersion != expectedProtocolVersion {
		return nil, errors.Errorf("ValidateResultsBlock incorrect protocol version: expected %v actual %v", expectedProtocolVersion, checkedProtocolVersion)
	}

	checkedVirtualChainId := input.ResultsBlock.Header.VirtualChainId()
	expectedVirtualChainId := s.config.VirtualChainId()
	if checkedVirtualChainId != expectedVirtualChainId {
		return nil, errors.Errorf("ValidateResultsBlock incorrect virtualChainId: expected %v actual %v", expectedVirtualChainId, checkedVirtualChainId)
	}

	// Check timestamp equals the TransactionsBlock timestamp.
	txTimestamp := input.TransactionsBlock.Header.Timestamp()
	rxTimestamp := input.ResultsBlock.Header.Timestamp()
	if rxTimestamp != txTimestamp {
		return nil, fmt.Errorf("ValidateResultsBlock mismatching timestamps: txTimestamp=%v rxTimestamp=%v", txTimestamp, rxTimestamp)
	}

	//prevBlockTimestamp := input.PrevBlockTimestamp
	//jitter := primitives.TimestampNano(s.config.ConsensusContextSystemTimestampAllowedJitter())
	//currentTimestamp := primitives.TimestampNano(time.Now().UnixNano())
	//if err := verifyBlockTimestamp(rxTimestamp, prevBlockTimestamp, currentTimestamp, jitter); err != nil {
	//	return nil, errors.Errorf("ValidateResultsBlock incorrect block Timestamp", err)
	//}

	// *validate rx hash pointers*
	// Check hash pointer matches the given previous block hash.
	prevBlockHashPtr := input.ResultsBlock.Header.PrevBlockHashPtr()
	expectedPrevBlockHashPtr := input.PrevBlockHash
	if !bytes.Equal(prevBlockHashPtr, expectedPrevBlockHashPtr) {
		return nil, errors.Errorf("ValidateResultsBlock mismatching previous block pointer: expected %v actual %v", expectedPrevBlockHashPtr, prevBlockHashPtr)
	}

	// Check hash pointer to the Transactions block of the same height.
	txBlockHashPtr := input.ResultsBlock.Header.TransactionsBlockHashPtr()
	expectedTxBlockHashPtr := digest.CalcTransactionsBlockHash(input.TransactionsBlock)
	if !bytes.Equal(txBlockHashPtr, expectedTxBlockHashPtr) {
		return nil, errors.Errorf("ValidateResultsBlock mismatching transaction block pointer: expected %v actual %v", expectedTxBlockHashPtr, txBlockHashPtr)
	}

	//Check the block's receipts_root_hash: Calculate the merkle root hash of the block's receipts and verify the hash in the header.
	recieptsMerkleRoot := input.ResultsBlock.Header.RawReceiptsRootHash()
	if calculatedRecieptMerkleRoot, err := calculateReceiptsMerkleRoot(input.ResultsBlock.TransactionReceipts); err != nil {
		return nil, errors.Errorf("ValidateResultsBlock error calculateReceiptsMerkleRoot", log.Error(err))
	} else if !bytes.Equal(recieptsMerkleRoot, []byte(calculatedRecieptMerkleRoot)) {
		return nil, errors.New("ValidateResultsBlock error receipt merkleRoot in header does not match txs receipts")
	}

	//Check the block's state_diff_hash: Calculate the hash of the block's state diff and verify the hash in the header.
	stateDiffMerkleRoot := input.ResultsBlock.Header.RawStateDiffHash()
	if calculatedStateDiffMerkleRoot, err := calculateStateDiffMerkleRoot(input.ResultsBlock.ContractStateDiffs); err != nil {
		return nil, errors.Errorf("ValidateResultsBlock error calculateStateDiffMerkleRoot", log.Error(err))
	} else if !bytes.Equal(stateDiffMerkleRoot, []byte(calculatedStateDiffMerkleRoot)) {
		return nil, errors.New("ValidateResultsBlock error state diff merkleRoot in header does not match state diffs")
	}

	// Check merkle root of the state prior to the block execution, retrieved by calling StateStorage.GetStateHash.
	preExecutionMerkleRoot := input.ResultsBlock.Header.RawPreExecutionStateRootHash()
	if getStateHashOut, err := s.stateStorage.GetStateHash(ctx, &services.GetStateHashInput{
		BlockHeight: input.ResultsBlock.Header.BlockHeight() - 1,
	}); err != nil {
		return nil, errors.Errorf("ValidateResultsBlock error GetStateHash", log.Error(err))
	} else if !bytes.Equal(preExecutionMerkleRoot, []byte(getStateHashOut.StateRootHash)) {
		return nil, errors.New("ValidateResultsBlock error pre-execution state merkleRoot in header does not match retrieved stateHash")
	}

	//Validate transaction execution
	// Execute the ordered transactions set by calling VirtualMachine.ProcessTransactionSet creating receipts and state diff. Using the provided header timestamp as a reference timestamp.
	if processTxsOut, err := s.virtualMachine.ProcessTransactionSet(ctx, &services.ProcessTransactionSetInput{
		BlockHeight:        input.TransactionsBlock.Header.BlockHeight(),
		BlockTimestamp:     input.TransactionsBlock.Header.Timestamp(),
		SignedTransactions: input.TransactionsBlock.SignedTransactions,
	}); err != nil {
		return nil, errors.Errorf("ValidateResultsBlock error GetStateHash", log.Error(err))
	} else {
		// Compare the receipts merkle root hash to the one in the block.
		recieptsMerkleRoot := input.ResultsBlock.Header.RawReceiptsRootHash()
		if calculatedRecieptMerkleRoot, err := calculateReceiptsMerkleRoot(processTxsOut.TransactionReceipts); err != nil {
			return nil, errors.Errorf("ValidateResultsBlock error ProcessTransactionSet calculateReceiptsMerkleRoot", log.Error(err))
		} else if !bytes.Equal(recieptsMerkleRoot, calculatedRecieptMerkleRoot) {
			return nil, errors.New("ValidateResultsBlock error receipt merkleRoot in header does not match processed txs receipts")
		}

		// Compare the state diff hash to the one in the block (supports only deterministic execution).
		stateDiffMerkleRoot := input.ResultsBlock.Header.RawStateDiffHash()
		if calculatedStateDiffMerkleRoot, err := calculateStateDiffMerkleRoot(processTxsOut.ContractStateDiffs); err != nil {
			return nil, errors.Errorf("ValidateResultsBlock error ProcessTransactionSet calculateStateDiffMerkleRoot", log.Error(err))
		} else if !bytes.Equal(stateDiffMerkleRoot, calculatedStateDiffMerkleRoot) {
			//return nil, errors.New("ValidateResultsBlock error state diff merkleRoot in header does not match processed state diffs")
			// TODO: fix this
			s.logger.Info("ValidateResultsBlock error state diff merkleRoot in header does not match processed state diffs")
		}
	}
	return &services.ValidateResultsBlockOutput{}, nil

}

*/

//func (s *service) ValidateResultsBlock(ctx context.Context, input *services.ValidateResultsBlockInput) (*services.ValidateResultsBlockOutput, error) {
//
//	err := ValidateResultsBlockInternal(ctx, input,
//		s.config.ProtocolVersion(), s.config.VirtualChainId(),
//		s.stateStorage.GetStateHash,
//		s.virtualMachine.ProcessTransactionSet)
//	return &services.ValidateResultsBlockOutput{}, err
//
//}

//func ValidateResultsBlockInternal(ctx context.Context, input *services.ValidateResultsBlockInput,
//	expectedProtocolVersion primitives.ProtocolVersion,
//	expectedVirtualChainId primitives.VirtualChainId,
//	getStateHash func(ctx context.Context, input *services.GetStateHashInput) (*services.GetStateHashOutput, error),
//	processTransactionSet func(ctx context.Context, input *services.ProcessTransactionSetInput) (*services.ProcessTransactionSetOutput, error),
//) error {
//	fmt.Println("ValidateResultsBlock ", ctx, input)
//
//	checkedHeader := input.ResultsBlock.Header
//	blockProtocolVersion := checkedHeader.ProtocolVersion()
//	blockVirtualChainId := checkedHeader.VirtualChainId()
//
//	if blockProtocolVersion != expectedProtocolVersion {
//		return fmt.Errorf("incorrect protocol version: expected %v but block has %v", expectedProtocolVersion, blockProtocolVersion)
//	}
//
//	if blockVirtualChainId != expectedVirtualChainId {
//		return fmt.Errorf("incorrect virtual chain ID: expected %v but block has %v", expectedVirtualChainId, blockVirtualChainId)
//	}
//
//	if input.BlockHeight != checkedHeader.BlockHeight() {
//		return fmt.Errorf("mismatching blockHeight: input %v checkedHeader %v", input.BlockHeight, checkedHeader.BlockHeight())
//	}
//
//	prevBlockHashPtr := input.ResultsBlock.Header.PrevBlockHashPtr()
//	if !bytes.Equal(input.PrevBlockHash, prevBlockHashPtr) {
//		return errors.New("incorrect previous results block hash")
//	}
//
//	if checkedHeader.Timestamp() != input.TransactionsBlock.Header.Timestamp() {
//		return fmt.Errorf("mismatching timestamps: txBlock=%v rxBlock=%v", checkedHeader.Timestamp(), input.TransactionsBlock.Header.Timestamp())
//	}
//
//	// Check the receipts merkle root matches the receipts.
//	receipts := input.ResultsBlock.TransactionReceipts
//	calculatedReceiptsRoot, err := calculateReceiptsMerkleRoot(receipts)
//	if err != nil {
//		fmt.Errorf("error in calculatedReceiptsRoot  blockheight=%v", input.BlockHeight)
//		return err
//	}
//	if !bytes.Equal(checkedHeader.ReceiptsRootHash(), calculatedReceiptsRoot) {
//		fmt.Println("ValidateResultsBlock122 ", calculatedReceiptsRoot, checkedHeader)
//		return errors.New("incorrect receipts root hash")
//	}
//
//	// Check the hash of the state diff in the block.
//	// TODO Statediff not impl - pending https://tree.taiga.io/project/orbs-network/us/535
//
//	// Check hash pointer to the Transactions block of the same height.
//	if checkedHeader.BlockHeight() != input.TransactionsBlock.Header.BlockHeight() {
//		return fmt.Errorf("mismatching block height: txBlock=%v rxBlock=%v", checkedHeader.BlockHeight(), input.TransactionsBlock.Header.BlockHeight())
//	}
//
//	// Check merkle root of the state prior to the block execution, retrieved by calling `StateStorage.GetStateHash`. blockHeight-1
//	calculatedPreExecutionStateRootHash, err := getStateHash(ctx, &services.GetStateHashInput{
//		BlockHeight: checkedHeader.BlockHeight() - 1,
//	})
//	if err != nil {
//		return err
//	}
//
//	if !bytes.Equal(checkedHeader.PreExecutionStateRootHash(), calculatedPreExecutionStateRootHash.StateRootHash) {
//		return fmt.Errorf("mismatching PreExecutionStateRootHash: expected %v but results block hash %v",
//			calculatedPreExecutionStateRootHash, checkedHeader.PreExecutionStateRootHash())
//	}
//
//	// Check transaction id bloom filter (see block format for structure).
//	// TODO Pending spec https://github.com/orbs-network/orbs-spec/issues/118
//
//	// Check transaction timestamp bloom filter (see block format for structure).
//	// TODO Pending spec https://github.com/orbs-network/orbs-spec/issues/118
//
//	// Validate transaction execution
//
//	// Execute the ordered transactions set by calling VirtualMachine.ProcessTransactionSet
//	// (creating receipts and state diff). Using the provided header timestamp as a reference timestamp.
//	_, err = processTransactionSet(ctx, &services.ProcessTransactionSetInput{
//		BlockHeight:        checkedHeader.BlockHeight(),
//		SignedTransactions: input.TransactionsBlock.SignedTransactions,
//	})
//	if err != nil {
//		return err
//	}
//
//	// Compare the receipts merkle root hash to the one in the block
//
//	// Compare the state diff hash to the one in the block (supports only deterministic execution).
//
//	// TODO https://tree.taiga.io/project/orbs-network/us/535 How to calculate receipts merkle hash root and state diff hash
//	// See https://github.com/orbs-network/orbs-spec/issues/111
//	//blockMerkleRootHash := checkedHeader.ReceiptsRootHash()
//
//	return nil
//
//}
