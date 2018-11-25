package codec

import (
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/gossipmessages"
	"github.com/pkg/errors"
)

func EncodeForwardedTransactions(header *gossipmessages.Header, message *gossipmessages.ForwardedTransactionsMessage) [][]byte {
	payloads := make([][]byte, 0, 2+len(message.SignedTransactions))
	payloads = append(payloads, header.Raw())
	payloads = append(payloads, message.Sender.Raw())
	for _, tx := range message.SignedTransactions {
		payloads = append(payloads, tx.Raw())
	}

	return payloads
}

func DecodeForwardedTransactions(payloads [][]byte) (*gossipmessages.ForwardedTransactionsMessage, error) {
	txs := make([]*protocol.SignedTransaction, 0, len(payloads)-1)

	senderSignature := gossipmessages.SenderSignatureReader(payloads[0])
	if !senderSignature.IsValid() {
		return nil, errors.New("SenderSignature is corrupted and cannot be decoded")
	}
	for _, payload := range payloads[1:] {
		tx := protocol.SignedTransactionReader(payload)
		if !tx.IsValid() {
			return nil, errors.New("SignedTransaction is corrupted and cannot be decoded")
		}
		txs = append(txs, tx)
	}

	return &gossipmessages.ForwardedTransactionsMessage{
		Sender:             senderSignature,
		SignedTransactions: txs,
	}, nil
}
