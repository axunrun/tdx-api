package main

import "testing"

func TestMatchPaperLimitBuyFillsWhenPriceIsBelowLimit(t *testing.T) {
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
		Side:      paperSideBuy,
		OrderType: paperOrderLimit,
		Price:     10,
		Quantity:  100,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = store.MatchOpenOrders(fixedPaperQuote(9.8))
	if err != nil {
		t.Fatal(err)
	}

	filled, err := store.GetOrder(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filled.Status != paperOrderFilled || filled.FilledQuantity != 100 {
		t.Fatalf("order = %+v", filled)
	}
	trades, err := store.ListTrades(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 || trades[0].Price != 9.8 || trades[0].Quantity != 100 {
		t.Fatalf("trades = %+v", trades)
	}
	got, err := store.GetAccount(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FrozenCash != 0 {
		t.Fatalf("frozen cash = %f, want 0", got.FrozenCash)
	}
}

func TestMatchPaperLimitSellFillsWhenPriceIsAboveLimit(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name: "seller",
		InitialPositions: []PaperInitialPosition{
			{Code: "600000", Quantity: 200, CostPrice: 10},
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

	if err := store.MatchOpenOrders(fixedPaperQuote(11.2)); err != nil {
		t.Fatal(err)
	}

	filled, err := store.GetOrder(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filled.Status != paperOrderFilled || filled.FilledQuantity != 100 {
		t.Fatalf("order = %+v", filled)
	}
	positions, err := store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if positions[0].Quantity != 100 || positions[0].FrozenQuantity != 0 {
		t.Fatalf("position = %+v", positions[0])
	}
}

func TestPaperMarketBuyFillsWithQuote(t *testing.T) {
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
		Side:      paperSideBuy,
		OrderType: paperOrderMarket,
		Quantity:  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetAccount(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FrozenCash != 0 || got.AvailableCash != 20000 {
		t.Fatalf("account before fill = %+v", got)
	}

	if err := store.FillOrder(order.ID, PaperQuote{Code: "600000", Price: 9.9}); err != nil {
		t.Fatal(err)
	}

	filled, err := store.GetOrder(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filled.Status != paperOrderFilled {
		t.Fatalf("order = %+v", filled)
	}
	got, err = store.GetAccount(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantCash := 20000 - 990 - calculatePaperFee(PaperFeeInput{
		AssetType: paperAssetStock,
		Side:      paperSideBuy,
		Amount:    990,
	}).Total()
	assertFloatEqual(t, got.AvailableCash, wantCash)
}

func TestFillPaperOrderRejectsAlreadyFilledOrder(t *testing.T) {
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
		Side:      paperSideBuy,
		OrderType: paperOrderMarket,
		Quantity:  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FillOrder(order.ID, PaperQuote{Code: "600000", Price: 10}); err != nil {
		t.Fatal(err)
	}
	if err := store.FillOrder(order.ID, PaperQuote{Code: "600000", Price: 10}); err == nil {
		t.Fatal("FillOrder() second call error = nil, want error")
	}
	trades, err := store.ListTrades(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 {
		t.Fatalf("len(trades) = %d, want 1", len(trades))
	}
}

func TestSellLastPositionCreatesClosedPosition(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name: "seller",
		InitialPositions: []PaperInitialPosition{
			{Code: "600000", Name: "stock", Quantity: 100, CostPrice: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: account.ID,
		Code:      "600000",
		Side:      paperSideSell,
		OrderType: paperOrderMarket,
		Quantity:  100,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.FillOrder(order.ID, PaperQuote{Code: "600000", Price: 12}); err != nil {
		t.Fatal(err)
	}

	closed, err := store.ListClosedPositions(account.ID, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(closed) != 1 {
		t.Fatalf("len(closed) = %d, want 1", len(closed))
	}
	if closed[0].Quantity != 100 || closed[0].OpenAmount != 1000 {
		t.Fatalf("closed = %+v", closed[0])
	}
}

func TestCreatePaperSnapshotUsesQuote(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "holder",
		InitialCash: 1000,
		InitialPositions: []PaperInitialPosition{
			{Code: "600000", Quantity: 100, CostPrice: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.CreateSnapshot(account.ID, fixedPaperQuote(12)); err != nil {
		t.Fatal(err)
	}

	var totalAssets, marketValue float64
	err = store.db.QueryRow(`
		SELECT total_assets, market_value
		FROM paper_account_snapshots
		WHERE account_id = ?
	`, account.ID).Scan(&totalAssets, &marketValue)
	if err != nil {
		t.Fatal(err)
	}
	assertFloatEqual(t, marketValue, 1200)
	assertFloatEqual(t, totalAssets, 2200)
}

func fixedPaperQuote(price float64) PaperQuoteProvider {
	return func(code string) (PaperQuote, error) {
		return PaperQuote{Code: code, Price: price}, nil
	}
}
