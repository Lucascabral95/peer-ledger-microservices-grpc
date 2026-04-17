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
	Token        string  `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string  `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType    string  `json:"token_type" example:"Bearer"`
	ExpiresIn    int64   `json:"expires_in" example:"86400"`
	User         UserDTO `json:"user"`
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

type RefreshTokenRequestDoc struct {
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

type LoginResponse struct {
	Error   bool        `json:"error" example:"false"`
	Message string      `json:"message" example:"login successful"`
	Data    AuthPayload `json:"data"`
}

type RefreshTokenResponse struct {
	Error   bool        `json:"error" example:"false"`
	Message string      `json:"message" example:"token refreshed successfully"`
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

type DashboardWalletSummary struct {
	Balance float64 `json:"balance" example:"125000.5"`
}

type DashboardTransferSummary struct {
	SentTotal          float64 `json:"sent_total" example:"18000"`
	ReceivedTotal      float64 `json:"received_total" example:"24250"`
	SentCountTotal     int64   `json:"sent_count_total" example:"12"`
	ReceivedCountTotal int64   `json:"received_count_total" example:"16"`
}

type DashboardTopUpSummary struct {
	CountTotal  int64   `json:"count_total" example:"5"`
	AmountTotal float64 `json:"amount_total" example:"30000"`
	CountToday  int64   `json:"count_today" example:"1"`
	AmountToday float64 `json:"amount_today" example:"5000"`
}

type DashboardActivityToday struct {
	TransferSentCount     int64 `json:"transfer_sent_count" example:"1"`
	TransferReceivedCount int64 `json:"transfer_received_count" example:"2"`
	TopUpCount            int64 `json:"topup_count" example:"1"`
	TotalEvents           int64 `json:"total_events" example:"4"`
}

type ActivityItem struct {
	ID                 string   `json:"id" example:"tx-123"`
	Kind               string   `json:"kind" example:"transfer_sent"`
	Status             string   `json:"status" example:"completed"`
	Amount             float64  `json:"amount" example:"1500"`
	CreatedAt          string   `json:"created_at" example:"2026-04-15T13:10:00Z"`
	CounterpartyUserID string   `json:"counterparty_user_id,omitempty" example:"user-002"`
	BalanceAfter       *float64 `json:"balance_after,omitempty" swaggertype:"number" example:"125000.5"`
}

type TopUpItem struct {
	ID           string  `json:"id" example:"topup-001"`
	Kind         string  `json:"kind" example:"topup"`
	Status       string  `json:"status" example:"completed"`
	Amount       float64 `json:"amount" example:"5000"`
	BalanceAfter float64 `json:"balance_after" example:"125000.5"`
	CreatedAt    string  `json:"created_at" example:"2026-04-15T11:00:00Z"`
}

type PaginationMeta struct {
	Page     int  `json:"page" example:"1"`
	PageSize int  `json:"page_size" example:"20"`
	HasNext  bool `json:"has_next" example:"false"`
}

type TopUpFilters struct {
	From *string `json:"from" swaggertype:"string" example:"2026-04-01T00:00:00Z"`
	To   *string `json:"to" swaggertype:"string" example:"2026-04-30T23:59:59Z"`
}

type ActivityFilters struct {
	Kind string  `json:"kind" example:"all"`
	From *string `json:"from" swaggertype:"string" example:"2026-04-01T00:00:00Z"`
	To   *string `json:"to" swaggertype:"string" example:"2026-04-30T23:59:59Z"`
}

type MeProfileResponse struct {
	Error   bool        `json:"error" example:"false"`
	Message string      `json:"message" example:"ok"`
	Data    GetUserData `json:"data"`
}

type MeDashboardData struct {
	Timezone        string                   `json:"timezone" example:"America/Argentina/Buenos_Aires"`
	User            UserDTO                  `json:"user"`
	Wallet          DashboardWalletSummary   `json:"wallet"`
	Transfers       DashboardTransferSummary `json:"transfers"`
	Topups          DashboardTopUpSummary    `json:"topups"`
	ActivityToday   DashboardActivityToday   `json:"activity_today"`
	RecentTransfers []ActivityItem           `json:"recent_transfers"`
	RecentTopUps    []TopUpItem              `json:"recent_topups"`
}

type MeDashboardResponse struct {
	Error   bool            `json:"error" example:"false"`
	Message string          `json:"message" example:"ok"`
	Data    MeDashboardData `json:"data"`
}

type MeWalletData struct {
	Timezone string                `json:"timezone" example:"America/Argentina/Buenos_Aires"`
	UserID   string                `json:"user_id" example:"user-001"`
	Balance  float64               `json:"balance" example:"125000.5"`
	Topups   DashboardTopUpSummary `json:"topups"`
}

type MeWalletResponse struct {
	Error   bool         `json:"error" example:"false"`
	Message string       `json:"message" example:"ok"`
	Data    MeWalletData `json:"data"`
}

type MeTopUpsData struct {
	Timezone   string         `json:"timezone" example:"America/Argentina/Buenos_Aires"`
	Items      []TopUpItem    `json:"items"`
	Pagination PaginationMeta `json:"pagination"`
	Filters    TopUpFilters   `json:"filters"`
}

type MeTopUpsResponse struct {
	Error   bool         `json:"error" example:"false"`
	Message string       `json:"message" example:"ok"`
	Data    MeTopUpsData `json:"data"`
}

type MeActivityData struct {
	Timezone   string          `json:"timezone" example:"America/Argentina/Buenos_Aires"`
	Items      []ActivityItem  `json:"items"`
	Pagination PaginationMeta  `json:"pagination"`
	Filters    ActivityFilters `json:"filters"`
}

type MeActivityResponse struct {
	Error   bool           `json:"error" example:"false"`
	Message string         `json:"message" example:"ok"`
	Data    MeActivityData `json:"data"`
}
