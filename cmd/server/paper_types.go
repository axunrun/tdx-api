package main

const (
	paperAccountActive = "active"
	paperAccountClosed = "closed"

	paperAssetStock = "stock"
	paperAssetETF   = "etf"

	paperSideBuy  = "buy"
	paperSideSell = "sell"

	paperOrderMarket  = "market"
	paperOrderLimit   = "limit"
	paperOrderAuction = "auction"

	paperOrderPending   = "pending"
	paperOrderFilled    = "filled"
	paperOrderCancelled = "cancelled"
	paperOrderRejected  = "rejected"

	paperTimeInForceDay         = "day"
	paperTimeInForceAuctionOnly = "auction_only"
)

type PaperCreateAccountRequest struct {
	Name             string                 `json:"name"`
	InitialCash      float64                `json:"initialCash"`
	InitialPositions []PaperInitialPosition `json:"initialPositions"`
	Note             string                 `json:"note,omitempty"`
}

type PaperInitialPosition struct {
	Code      string  `json:"code"`
	Name      string  `json:"name,omitempty"`
	AssetType string  `json:"assetType,omitempty"`
	Quantity  int64   `json:"quantity"`
	CostPrice float64 `json:"costPrice,omitempty"`
	BuyDate   string  `json:"buyDate,omitempty"`
}

type PaperPlaceOrderRequest struct {
	AccountID   string  `json:"accountId"`
	Code        string  `json:"code"`
	Name        string  `json:"name,omitempty"`
	AssetType   string  `json:"assetType,omitempty"`
	Side        string  `json:"side"`
	OrderType   string  `json:"orderType"`
	TimeInForce string  `json:"timeInForce,omitempty"`
	Price       float64 `json:"price,omitempty"`
	Quantity    int64   `json:"quantity"`
	Reason      string  `json:"reason,omitempty"`
}

type PaperAccount struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	Note          string  `json:"note,omitempty"`
	CreatedAt     string  `json:"createdAt"`
	ClosedAt      string  `json:"closedAt,omitempty"`
	InitialCash   float64 `json:"initialCash"`
	AvailableCash float64 `json:"availableCash"`
	FrozenCash    float64 `json:"frozenCash"`
}

type PaperPosition struct {
	AccountID        string  `json:"accountId"`
	Code             string  `json:"code"`
	Name             string  `json:"name,omitempty"`
	AssetType        string  `json:"assetType"`
	Quantity         int64   `json:"quantity"`
	SellableQuantity int64   `json:"sellableQuantity"`
	FrozenQuantity   int64   `json:"frozenQuantity"`
	AvgCost          float64 `json:"avgCost"`
	UpdatedAt        string  `json:"updatedAt"`
}

type PaperOrder struct {
	ID             string  `json:"id"`
	AccountID      string  `json:"accountId"`
	Code           string  `json:"code"`
	Name           string  `json:"name,omitempty"`
	AssetType      string  `json:"assetType"`
	Side           string  `json:"side"`
	OrderType      string  `json:"orderType"`
	Status         string  `json:"status"`
	TimeInForce    string  `json:"timeInForce"`
	RejectReason   string  `json:"rejectReason,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
	FilledAt       string  `json:"filledAt,omitempty"`
	CancelledAt    string  `json:"cancelledAt,omitempty"`
	Price          float64 `json:"price,omitempty"`
	Quantity       int64   `json:"quantity"`
	FilledQuantity int64   `json:"filledQuantity"`
}

type PaperQuote struct {
	Code  string  `json:"code"`
	Name  string  `json:"name,omitempty"`
	Price float64 `json:"price"`
}

type PaperTrade struct {
	ID          string  `json:"id"`
	OrderID     string  `json:"orderId"`
	AccountID   string  `json:"accountId"`
	Code        string  `json:"code"`
	Side        string  `json:"side"`
	TradedAt    string  `json:"tradedAt"`
	Price       float64 `json:"price"`
	Quantity    int64   `json:"quantity"`
	Amount      float64 `json:"amount"`
	Commission  float64 `json:"commission"`
	StampTax    float64 `json:"stampTax"`
	TransferFee float64 `json:"transferFee"`
}

type PaperClosedPosition struct {
	ID          string  `json:"id"`
	AccountID   string  `json:"accountId"`
	Code        string  `json:"code"`
	Name        string  `json:"name,omitempty"`
	OpenedAt    string  `json:"openedAt,omitempty"`
	ClosedAt    string  `json:"closedAt"`
	Quantity    int64   `json:"quantity"`
	OpenAmount  float64 `json:"openAmount"`
	CloseAmount float64 `json:"closeAmount"`
	RealizedPnL float64 `json:"realizedPnl"`
}

type PaperFeeInput struct {
	AssetType string
	Side      string
	Amount    float64
}

type PaperFee struct {
	Commission  float64
	StampTax    float64
	TransferFee float64
}

func (f PaperFee) Total() float64 {
	return f.Commission + f.StampTax + f.TransferFee
}
