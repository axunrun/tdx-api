package main

const (
	paperAccountActive = "active"
	paperAccountClosed = "closed"

	paperAssetStock = "stock"

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
