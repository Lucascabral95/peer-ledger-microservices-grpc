package prototest

import (
	"testing"

	fraudpb "github.com/peer-ledger/gen/fraud"
	transactionpb "github.com/peer-ledger/gen/transaction"
	userpb "github.com/peer-ledger/gen/user"
	walletpb "github.com/peer-ledger/gen/wallet"
	"google.golang.org/protobuf/proto"
)

func TestGeneratedProtobufMessagesMarshal(t *testing.T) {
	messages := []proto.Message{
		&userpb.RegisterRequest{Name: "Test User", Email: "test@example.com", Password: "Test.1234"},
		&userpb.RegisterResponse{UserId: "user-test", Name: "Test User", Email: "test@example.com"},
		&userpb.LoginRequest{Email: "test@example.com", Password: "Test.1234"},
		&userpb.DeleteUserRequest{UserId: "user-test"},
		&walletpb.CreateWalletRequest{UserId: "user-test"},
		&walletpb.TopUpRequest{UserId: "user-test", Amount: 100},
		&walletpb.ListTopUpsRequest{UserId: "user-test", Limit: 10},
		&walletpb.ListTopUpsResponse{Records: []*walletpb.TopUpRecord{{TopupId: "topup-test", UserId: "user-test", Amount: 100}}},
		&transactionpb.GetTransferSummaryRequest{UserId: "user-test", Timezone: "America/Argentina/Buenos_Aires"},
		&transactionpb.ListTransfersRequest{UserId: "user-test", Direction: transactionpb.TransferDirection_TRANSFER_DIRECTION_ALL, Limit: 10},
		&transactionpb.ListTransfersResponse{Records: []*transactionpb.TransactionRecord{{TransactionId: "tx-test", SenderId: "user-test", ReceiverId: "other-user", Amount: 50}}},
		&fraudpb.EvaluateRequest{SenderId: "user-test", ReceiverId: "other-user", Amount: 50, IdempotencyKey: "idem-test"},
	}

	for _, message := range messages {
		if _, err := proto.Marshal(message); err != nil {
			t.Fatalf("marshal %T: %v", message, err)
		}
	}
}
