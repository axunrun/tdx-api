package main

import (
	"math"
	"testing"
)

func TestPaperFeeForStockSell(t *testing.T) {
	fee := calculatePaperFee(PaperFeeInput{
		AssetType: paperAssetStock,
		Side:      paperSideSell,
		Amount:    10000,
	})

	assertFloatEqual(t, fee.Commission, 1)
	assertFloatEqual(t, fee.StampTax, 5)
	assertFloatEqual(t, fee.TransferFee, 0.1)
	assertFloatEqual(t, fee.Total(), 6.1)
}

func TestPaperFeeForETFSkipsStampTax(t *testing.T) {
	fee := calculatePaperFee(PaperFeeInput{
		AssetType: paperAssetETF,
		Side:      paperSideSell,
		Amount:    10000,
	})

	assertFloatEqual(t, fee.Commission, 1)
	assertFloatEqual(t, fee.StampTax, 0)
	assertFloatEqual(t, fee.TransferFee, 0)
	assertFloatEqual(t, fee.Total(), 1)
}

func TestPlacePaperLimitBuyFreezesCash(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "buyer",
		InitialCash: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}

	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: account.ID,
		Code:      "600000",
		Name:      "stock",
		Side:      paperSideBuy,
		OrderType: paperOrderLimit,
		Price:     10,
		Quantity:  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != paperOrderPending || order.AssetType != paperAssetStock ||
		order.TimeInForce != paperTimeInForceDay {
		t.Fatalf("order = %+v", order)
	}

	got, err := store.GetAccount(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantFrozen := 1000 + calculatePaperFee(PaperFeeInput{
		AssetType: paperAssetStock,
		Side:      paperSideBuy,
		Amount:    1000,
	}).Total()
	assertFloatEqual(t, got.FrozenCash, wantFrozen)
	assertFloatEqual(t, got.AvailableCash, 20000-wantFrozen)
	assertPaperRowCount(t, store.db, "paper_orders", 1)
	assertPaperRowCount(t, store.db, "paper_agent_actions", 2)
}

func TestPlacePaperLimitSellFreezesPosition(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name: "seller",
		InitialPositions: []PaperInitialPosition{
			{Code: "600000", Name: "stock", Quantity: 200, CostPrice: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: account.ID,
		Code:      "600000",
		Side:      paperSideSell,
		OrderType: paperOrderLimit,
		Price:     11,
		Quantity:  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != paperOrderPending {
		t.Fatalf("order = %+v", order)
	}

	positions, err := store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 1 {
		t.Fatalf("len(positions) = %d, want 1", len(positions))
	}
	if positions[0].SellableQuantity != 100 || positions[0].FrozenQuantity != 100 {
		t.Fatalf("position = %+v", positions[0])
	}
}

func TestPlacePaperOrderRejectsInvalidQuantity(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "buyer",
		InitialCash: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: account.ID,
		Code:      "600000",
		Side:      paperSideBuy,
		OrderType: paperOrderLimit,
		Price:     10,
		Quantity:  99,
	}); err == nil {
		t.Fatal("PlaceOrder() error = nil, want error")
	}
	assertPaperRowCount(t, store.db, "paper_orders", 0)
}

func TestPlacePaperOrderRejectsInsufficientPosition(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name: "seller",
		InitialPositions: []PaperInitialPosition{
			{Code: "600000", Quantity: 100, CostPrice: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: account.ID,
		Code:      "600000",
		Side:      paperSideSell,
		OrderType: paperOrderLimit,
		Price:     10,
		Quantity:  200,
	}); err == nil {
		t.Fatal("PlaceOrder() error = nil, want error")
	}

	positions, err := store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if positions[0].SellableQuantity != 100 || positions[0].FrozenQuantity != 0 {
		t.Fatalf("position = %+v", positions[0])
	}
	assertPaperRowCount(t, store.db, "paper_orders", 0)
}

func assertFloatEqual(t *testing.T, got, want float64) {
	t.Helper()

	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("got %f, want %f", got, want)
	}
}
