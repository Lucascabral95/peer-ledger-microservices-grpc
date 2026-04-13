package main

type HealthResponse struct {
	Error   bool   `json:"error" example:"false"`
	Message string `json:"message" example:"ok"`
}

type ErrorResponse struct {
	Error   bool   `json:"error" example:"true"`
	Message string `json:"message" example:"authentication required"`
}

type UserDTO struct {
	UserID string `json:"user_id" example:"user-001"`
	Name   string `json:"name" example:"Lucas"`
	Email  string `json:"email" example:"lucas@mail.com"`
}

type AuthPayload struct {
	Token string  `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  UserDTO `json:"user"`
}

type RegisterRequestDoc struct {
	Name     string `json:"name" example:"Lucas"`
	Email    string `json:"email" example:"lucas@mail.com"`
	Password string `json:"password" example:"Password123!"`
}

type RegisterResponse struct {
	Error   bool        `json:"error" example:"false"`
	Message string      `json:"message" example:"user registered successfully"`
	Data    AuthPayload `json:"data"`
}

type WalletProvisioningFailureData struct {
	UserID string `json:"user_id" example:"user-001"`
	Stage  string `json:"stage" example:"wallet_provisioning"`
}

type WalletProvisioningFailureResponse struct {
	Error   bool                          `json:"error" example:"true"`
	Message string                        `json:"message" example:"user created but wallet provisioning failed"`
	Data    WalletProvisioningFailureData `json:"data"`
}

type LoginRequestDoc struct {
	Email    string `json:"email" example:"lucas@mail.com"`
	Password string `json:"password" example:"Password123!"`
}

type LoginResponse struct {
	Error   bool        `json:"error" example:"false"`
	Message string      `json:"message" example:"login successful"`
	Data    AuthPayload `json:"data"`
}

type GetUserData struct {
	UserID string `json:"user_id" example:"user-001"`
	Name   string `json:"name" example:"Lucas"`
	Email  string `json:"email" example:"lucas@mail.com"`
}

type GetUserResponse struct {
	Error   bool        `json:"error" example:"false"`
	Message string      `json:"message" example:"ok"`
	Data    GetUserData `json:"data"`
}

type UserExistsData struct {
	UserID string `json:"user_id" example:"user-001"`
	Exists bool   `json:"exists" example:"true"`
}

type UserExistsResponse struct {
	Error   bool           `json:"error" example:"false"`
	Message string         `json:"message" example:"ok"`
	Data    UserExistsData `json:"data"`
}

type TopUpRequestDoc struct {
	Amount float64 `json:"amount" example:"5000"`
}

type TopUpData struct {
	UserID  string  `json:"user_id" example:"user-001"`
	Balance float64 `json:"balance" example:"2500"`
	Amount  float64 `json:"amount" example:"500"`
}

type TopUpResponse struct {
	Error   bool      `json:"error" example:"false"`
	Message string    `json:"message" example:"wallet topped up successfully"`
	Data    TopUpData `json:"data"`
}

type TransferRequestDoc struct {
	ReceiverID     string  `json:"receiver_id" example:"user-002"`
	Amount         float64 `json:"amount" example:"1000.01"`
	IdempotencyKey string  `json:"idempotency_key" example:"transfer-001"`
}

type TransferData struct {
	TransactionID string  `json:"transaction_id" example:"tx-200"`
	SenderBalance float64 `json:"sender_balance" example:"98999.99"`
	SenderID      string  `json:"sender_id" example:"user-001"`
	ReceiverID    string  `json:"receiver_id" example:"user-002"`
	Amount        float64 `json:"amount" example:"1000.01"`
}

type TransferResponse struct {
	Error   bool         `json:"error" example:"false"`
	Message string       `json:"message" example:"transfer executed and recorded successfully"`
	Data    TransferData `json:"data"`
}

type FraudBlockedData struct {
	Reason   string `json:"reason" example:"cooldown active for sender-receiver pair"`
	RuleCode string `json:"rule_code" example:"COOLDOWN_PAIR"`
}

type FraudBlockedResponse struct {
	Error   bool             `json:"error" example:"true"`
	Message string           `json:"message" example:"transfer blocked by fraud service"`
	Data    FraudBlockedData `json:"data"`
}

type TransferAuditFailureData struct {
	TransactionID  string  `json:"transaction_id" example:"tx-wallet-ok"`
	SenderBalance  float64 `json:"sender_balance" example:"900"`
	Stage          string  `json:"stage" example:"transaction_recording"`
	Retryable      bool    `json:"retryable" example:"true"`
	IdempotencyKey string  `json:"idempotency_key" example:"k-record-fail"`
}

type TransferAuditFailureResponse struct {
	Error   bool                     `json:"error" example:"true"`
	Message string                   `json:"message" example:"transfer executed in wallet-service but failed to record audit transaction"`
	Data    TransferAuditFailureData `json:"data"`
}

type TransactionRecordDTO struct {
	TransactionID string  `json:"transaction_id" example:"tx-1"`
	SenderID      string  `json:"sender_id" example:"user-001"`
	ReceiverID    string  `json:"receiver_id" example:"user-002"`
	Amount        float64 `json:"amount" example:"10"`
	Status        string  `json:"status" example:"completed"`
	CreatedAt     string  `json:"created_at" example:"2026-04-01T00:00:00Z"`
}

type GetHistoryData struct {
	UserID  string                 `json:"user_id" example:"user-001"`
	Records []TransactionRecordDTO `json:"records"`
}

type GetHistoryResponse struct {
	Error   bool           `json:"error" example:"false"`
	Message string         `json:"message" example:"ok"`
	Data    GetHistoryData `json:"data"`
}
