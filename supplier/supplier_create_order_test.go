package supplier

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/asadbekGo/ett-code-sdk"
)

// createOrderCall bundles everything CreateOrder needs so each test only sets what it cares
// about, and keeps the out-parameters CreateOrder writes through (the order item request, the
// queued HighPass orders, the Telegram notifications) readable after the call.
type createOrderCall struct {
	supplier      SupplierData
	order         OrderData
	additional    AdditionalData
	product       ProductData
	orderItemData map[string]interface{}
	paxInfo       map[string]PaxInfo
	// ucodeBaseURL backs the ObjectFunction that CreateOrder uses to write a refreshed token
	// back to the suppliers table.
	ucodeBaseURL string

	notifications   []string
	orderItems      []map[string]interface{}
	fastTrackOrders map[string]HighPassCrateOrderRequest
}

func (call *createOrderCall) run(t *testing.T) (CreateOrderResponse, sdk.ResponseError) {
	t.Helper()

	if call.orderItemData == nil {
		call.orderItemData = map[string]interface{}{}
	}
	if call.paxInfo == nil {
		call.paxInfo = map[string]PaxInfo{}
	}
	if call.orderItems == nil {
		call.orderItems = []map[string]interface{}{{}}
	}
	if call.fastTrackOrders == nil {
		call.fastTrackOrders = map[string]HighPassCrateOrderRequest{}
	}

	return CreateOrder(
		call.supplier,
		call.order,
		call.additional,
		call.product,
		func(text string) { call.notifications = append(call.notifications, text) },
		&sync.Mutex{},
		sdk.New(&sdk.Config{AppId: "test-app", BaseURL: call.ucodeBaseURL, FunctionName: "supplier-test"}),
		call.orderItems,
		call.orderItemData,
		call.paxInfo,
		call.fastTrackOrders,
		0,
	)
}

// liveToken and staleToken drive the "do we need to log in again?" check. Every supplier
// subtracts a safety margin from the stored expiry before comparing it to now.
func liveToken() string  { return time.Now().UTC().Add(4 * time.Hour).Format(time.RFC3339) }
func staleToken() string { return time.Now().UTC().Add(-time.Hour).Format(time.RFC3339) }

// ucodeServer stands in for the uCode object API that CreateOrder writes refreshed tokens to.
func ucodeServer(t *testing.T, status int) string {
	t.Helper()

	server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
		w.WriteHeader(status)
		w.Write([]byte(`{"status":"done"}`))
	})

	return server.URL
}

func TestCreateOrderPPG(t *testing.T) {
	t.Run("generates a coupon around the visit date", func(t *testing.T) {
		var couponBody string
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, got *capturedRequest) {
			couponBody = got.Body
			w.Write([]byte(`{"status":1,"result":{"coupon":"ETT999@ppg"}}`))
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "ppg", APIUrl: server.URL, Token: "tok-1", TokenExpiresAt: liveToken(), AiShortCode: "ETT"},
			order:    OrderData{FirstName: "Ada", LastName: "Lovelace", ProductDate: "2026-09-18"},
			product:  ProductData{ProductValue: 3, LocationShortCode: 42},
		}

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if response.CouponCode != "ETT999" {
			t.Errorf("got coupon %q, want ETT999", response.CouponCode)
		}

		var sent GenerateRequest
		if err := json.Unmarshal([]byte(couponBody), &sent); err != nil {
			t.Fatalf("coupon request is not JSON: %v", err)
		}
		// The coupon is valid for a day either side of the visit, to absorb timezone drift.
		if sent.StartDate != "2026-09-17" || sent.EndDate != "2026-09-19" {
			t.Errorf("got %s..%s, want the day either side of 2026-09-18", sent.StartDate, sent.EndDate)
		}
	})

	t.Run("refreshes an expired token and stores it back", func(t *testing.T) {
		var couponAuth string
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, got *capturedRequest) {
			switch got.Path {
			case "/api/fe/v1/user/login":
				w.Write([]byte(`{"token":"fresh-tok","expires":"2026-12-31T00:00:00Z"}`))
			case "/api/master/v1/coupon/generate":
				couponAuth = got.Header.Get("Authorization")
				w.Write([]byte(`{"status":1,"result":{"coupon":"ETT999@ppg"}}`))
			default:
				t.Errorf("unexpected path %q", got.Path)
			}
		})

		call := createOrderCall{
			supplier:     SupplierData{Type: "ppg", APIUrl: server.URL, Token: "stale-tok", TokenExpiresAt: staleToken()},
			order:        OrderData{ProductDate: "2026-09-18"},
			ucodeBaseURL: ucodeServer(t, http.StatusOK),
		}

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if response.CouponCode != "ETT999" {
			t.Errorf("got coupon %q, want ETT999", response.CouponCode)
		}
		// The whole point of the refresh: the coupon call must use the new token, not the stale one.
		if couponAuth != "Bearer fresh-tok" {
			t.Errorf("got Authorization %q, want the refreshed Bearer fresh-tok", couponAuth)
		}
	})

	t.Run("fails when the refreshed token cannot be stored", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"token":"fresh-tok","expires":"2026-12-31T00:00:00Z"}`))
		})

		call := createOrderCall{
			supplier:     SupplierData{Type: "ppg", APIUrl: server.URL, TokenExpiresAt: staleToken()},
			order:        OrderData{ProductDate: "2026-09-18"},
			ucodeBaseURL: ucodeServer(t, http.StatusInternalServerError),
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
		if errResponse.Description == nil {
			t.Error("the uCode failure description should be carried through")
		}
	})

	t.Run("fails when the login fails", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "ppg", APIUrl: server.URL, TokenExpiresAt: staleToken()},
			order:    OrderData{ProductDate: "2026-09-18"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
		if errResponse.TelegramErrorFile == "" {
			t.Error("the supplier notification should be attached for the Telegram alert")
		}
	})

	t.Run("fails on an unparsable stored expiry", func(t *testing.T) {
		call := createOrderCall{
			supplier: SupplierData{Type: "ppg", TokenExpiresAt: "not-a-timestamp"},
			order:    OrderData{ProductDate: "2026-09-18"},
		}

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
		if response.ErrorMessage == "" {
			t.Error("the response should carry the error message")
		}
		if errResponse.ClientErrorMessage != sdk.ErrorCodeWithMessage[422] {
			t.Errorf("got client message %q, want the canonical 422 message", errResponse.ClientErrorMessage)
		}
	})

	t.Run("alerts on an unparsable visit date only in production", func(t *testing.T) {
		call := createOrderCall{
			supplier:   SupplierData{Type: "ppg", TokenExpiresAt: liveToken()},
			order:      OrderData{ProductDate: "18.09.2026"},
			additional: AdditionalData{EnvironmentId: "prod-env", ProdEnvID: "prod-env"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 500 {
			t.Errorf("got status %d, want 500", errResponse.StatusCode)
		}
		if len(call.notifications) != 1 || !strings.Contains(call.notifications[0], "Parse Visit Date") {
			t.Errorf("got notifications %v, want one naming the visit date", call.notifications)
		}

		staging := createOrderCall{
			supplier:   SupplierData{Type: "ppg", TokenExpiresAt: liveToken()},
			order:      OrderData{ProductDate: "18.09.2026"},
			additional: AdditionalData{EnvironmentId: "staging-env", ProdEnvID: "prod-env"},
		}
		if _, errResponse := staging.run(t); errResponse.StatusCode != 500 {
			t.Errorf("got status %d, want 500", errResponse.StatusCode)
		}
		if len(staging.notifications) != 0 {
			t.Errorf("got notifications %v outside production, want none", staging.notifications)
		}
	})

	t.Run("fails when the coupon cannot be generated", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"status":0}`))
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "ppg", APIUrl: server.URL, TokenExpiresAt: liveToken()},
			order:    OrderData{ProductDate: "2026-09-18"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
		if errResponse.TelegramErrorFile == "" {
			t.Error("the supplier notification should be attached for the Telegram alert")
		}
	})
}

func TestCreateOrderDreamfolks(t *testing.T) {
	t.Run("returns the voucher and both logos", func(t *testing.T) {
		var voucherBody string
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, got *capturedRequest) {
			voucherBody = got.Body
			w.Write([]byte(`{"status":true,"data":{"voucher_code":"DF-777","logo_url":"logo-1","logo_url_2":"logo-2"}}`))
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "dreamfolks", APIUrl: server.URL, AiShortCode: "7", Username: "key", Password: "secret"},
			order: OrderData{
				FirstName: "Ada", LastName: "Lovelace", TotalPax: 2,
				ProductDate: "2026-09-18", AgentTransactionId: "txn-1", AgentOrderItemId: "item-1",
			},
			product: ProductData{DFCode: "outlet-1", TimezoneOffset: "5:00"},
		}

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if response.CouponCode != "DF-777" || response.LogoUrl != "logo-1" || response.LogoUrl2 != "logo-2" {
			t.Errorf("got %+v, want voucher DF-777 with both logos", response)
		}

		var sent GenerateCouponDFAPIRequest
		if err := json.Unmarshal([]byte(voucherBody), &sent); err != nil {
			t.Fatalf("voucher request is not JSON: %v", err)
		}
		// Dreamfolks takes a single name field and the agent's own identifiers.
		if sent.FirstName != "AdaLovelace" {
			t.Errorf("got first_name %q, want the names joined", sent.FirstName)
		}
		if sent.RequestIdentifier != "txn-1" || sent.BookingReferenceNo != "item-1" || sent.TotalVisit != 2 {
			t.Errorf("got %+v, want txn-1 / item-1 / 2 visits", sent)
		}
		// A future date is passed through untouched.
		if sent.ValidFrom != "2026-09-18" {
			t.Errorf("got valid_from %q, want the ordered date unchanged", sent.ValidFrom)
		}
	})

	t.Run("pushes a same-day order ten minutes out", func(t *testing.T) {
		var voucherBody string
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, got *capturedRequest) {
			voucherBody = got.Body
			w.Write([]byte(`{"status":true,"data":{"voucher_code":"DF-777"}}`))
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "dreamfolks", APIUrl: server.URL, AiShortCode: "7"},
			order:    OrderData{ProductDate: time.Now().Format(time.DateOnly)},
			product:  ProductData{DFCode: "outlet-1", TimezoneOffset: "0:00"},
		}

		if _, errResponse := call.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}

		var sent GenerateCouponDFAPIRequest
		if err := json.Unmarshal([]byte(voucherBody), &sent); err != nil {
			t.Fatalf("voucher request is not JSON: %v", err)
		}
		// Today's order gains a clock time so the supplier does not read it as midnight, already past.
		validFrom, err := time.Parse(time.DateTime, sent.ValidFrom)
		if err != nil {
			t.Fatalf("valid_from %q should carry a time for a same-day order: %v", sent.ValidFrom, err)
		}
		if drift := validFrom.Sub(wallClock(t, time.Now().Add(10*time.Minute))); drift > time.Minute || drift < -time.Minute {
			t.Errorf("got valid_from %v, want about ten minutes out (off by %v)", validFrom, drift)
		}
	})

	t.Run("fails on a non-numeric programme id", func(t *testing.T) {
		call := createOrderCall{
			supplier: SupplierData{Type: "dreamfolks", AiShortCode: "not-a-number"},
			order:    OrderData{ProductDate: "2026-09-18"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
	})

	t.Run("fails when the voucher is refused", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"status":false,"message":"no availability"}`))
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "dreamfolks", APIUrl: server.URL, AiShortCode: "7"},
			order:    OrderData{ProductDate: "2026-09-18"},
			product:  ProductData{DFCode: "outlet-1", TimezoneOffset: "5:00"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
		if !strings.Contains(errResponse.TelegramErrorFile, "no availability") {
			t.Errorf("got %q, want the supplier's message attached", errResponse.TelegramErrorFile)
		}
	})
}

func TestCreateOrderAllAirportsFlow(t *testing.T) {
	// allAirportsServer answers the three calls the flow makes in order.
	allAirportsServer := func(t *testing.T, fail string) string {
		t.Helper()

		server, _ := newSupplierServer(t, func(w http.ResponseWriter, got *capturedRequest) {
			if strings.HasPrefix(got.Path, "/connect/token") {
				w.Write([]byte(`{"access_token":"el-tok","expires_in":3600}`))
				return
			}
			switch {
			case got.Path == "/api/v0/orders":
				if fail == "create" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Write([]byte(`{"id":501}`))
			case strings.HasSuffix(got.Path, "/pay"):
				if fail == "pay" {
					w.WriteHeader(http.StatusPaymentRequired)
					return
				}
				w.Write([]byte(`{"pnr":"PNR-777"}`))
			case strings.HasPrefix(got.Path, "/api/v0/resources/"):
				if fail == "resource" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Write([]byte(`{"organization":{"id":"org-1"}}`))
			default:
				t.Errorf("unexpected path %q", got.Path)
			}
		})

		return server.URL
	}

	t.Run("orders, pays and records the flight info", func(t *testing.T) {
		call := createOrderCall{
			supplier: SupplierData{Type: "all_airports", APIUrl: allAirportsServer(t, ""), TokenExpiresAt: liveToken(), Token: "aa-tok"},
			order:    OrderData{FirstName: "Ada", LastName: "Lovelace", ProductDate: "2026-09-18", PaxType: []string{"adult"}},
			product:  ProductData{AACode: "resource-1", DestinationCity: "TAS"},
		}

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if response.CouponCode != "PNR-777" {
			t.Errorf("got coupon %q, want the pnr PNR-777", response.CouponCode)
		}

		// The flight info is stitched together from the order id and the resource's organization,
		// and handed back to the caller through the order item request.
		var flightInfo map[string]interface{}
		raw, ok := call.orderItems[0]["flight_info"].(string)
		if !ok {
			t.Fatalf("got flight_info %v, want a JSON string", call.orderItems[0]["flight_info"])
		}
		if err := json.Unmarshal([]byte(raw), &flightInfo); err != nil {
			t.Fatalf("flight_info is not JSON: %v", err)
		}
		if flightInfo["orderId"] != float64(501) || flightInfo["organizationId"] != "org-1" {
			t.Errorf("got %v, want order 501 under organization org-1", flightInfo)
		}
		if flightInfo["type"] != "Departure" || flightInfo["date"] != "2026-09-18" {
			t.Errorf("got %v, want a departure on 2026-09-18", flightInfo)
		}
	})

	t.Run("alerts and fails without a pax type", func(t *testing.T) {
		call := createOrderCall{
			supplier:   SupplierData{Type: "all_airports", TokenExpiresAt: liveToken()},
			order:      OrderData{ProductDate: "2026-09-18"},
			additional: AdditionalData{EnvironmentId: "prod-env", ProdEnvID: "prod-env"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 500 {
			t.Errorf("got status %d, want 500", errResponse.StatusCode)
		}
		if len(call.notifications) != 1 || !strings.Contains(call.notifications[0], "Pax Type") {
			t.Errorf("got notifications %v, want one naming the pax type", call.notifications)
		}
	})

	t.Run("alerts and fails on an unparsable stored expiry", func(t *testing.T) {
		call := createOrderCall{
			supplier:   SupplierData{Type: "all_airports", TokenExpiresAt: "not-a-timestamp"},
			order:      OrderData{ProductDate: "2026-09-18", PaxType: []string{"adult"}},
			additional: AdditionalData{EnvironmentId: "prod-env", ProdEnvID: "prod-env"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 500 {
			t.Errorf("got status %d, want 500", errResponse.StatusCode)
		}
		if len(call.notifications) != 1 || !strings.Contains(call.notifications[0], "Token Expire Time") {
			t.Errorf("got notifications %v, want one naming the token expiry", call.notifications)
		}
	})

	t.Run("refreshes an expired token before ordering", func(t *testing.T) {
		serverURL := allAirportsServer(t, "")
		call := createOrderCall{
			supplier:     SupplierData{Type: "all_airports", APIUrl: serverURL, AuthUrl: serverURL, TokenExpiresAt: staleToken()},
			order:        OrderData{ProductDate: "2026-09-18", PaxType: []string{"adult"}},
			product:      ProductData{AACode: "resource-1", DestinationCity: "TAS"},
			ucodeBaseURL: ucodeServer(t, http.StatusOK),
		}

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if response.CouponCode != "PNR-777" {
			t.Errorf("got coupon %q, want PNR-777", response.CouponCode)
		}
	})

	t.Run("alerts and fails when the refreshed token cannot be stored", func(t *testing.T) {
		serverURL := allAirportsServer(t, "")
		call := createOrderCall{
			supplier:     SupplierData{Type: "all_airports", APIUrl: serverURL, AuthUrl: serverURL, TokenExpiresAt: staleToken()},
			order:        OrderData{ProductDate: "2026-09-18", PaxType: []string{"adult"}},
			additional:   AdditionalData{EnvironmentId: "prod-env", ProdEnvID: "prod-env"},
			ucodeBaseURL: ucodeServer(t, http.StatusInternalServerError),
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 500 {
			t.Errorf("got status %d, want 500", errResponse.StatusCode)
		}
		if len(call.notifications) != 1 || !strings.Contains(call.notifications[0], "Token Update") {
			t.Errorf("got notifications %v, want one naming the token update", call.notifications)
		}
	})

	t.Run("fails when the login fails", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusForbidden)
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "all_airports", APIUrl: server.URL, AuthUrl: server.URL, TokenExpiresAt: staleToken()},
			order:    OrderData{ProductDate: "2026-09-18", PaxType: []string{"adult"}},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
	})

	for _, step := range []string{"create", "pay", "resource"} {
		t.Run("fails when the "+step+" step fails", func(t *testing.T) {
			call := createOrderCall{
				supplier: SupplierData{Type: "all_airports", APIUrl: allAirportsServer(t, step), TokenExpiresAt: liveToken()},
				order:    OrderData{ProductDate: "2026-09-18", PaxType: []string{"adult"}},
				product:  ProductData{AACode: "resource-1", DestinationCity: "TAS"},
			}

			_, errResponse := call.run(t)
			if errResponse.StatusCode != 422 {
				t.Errorf("got status %d, want 422", errResponse.StatusCode)
			}
			if errResponse.TelegramErrorFile == "" {
				t.Error("the supplier notification should be attached for the Telegram alert")
			}
			if call.orderItems[0]["flight_info"] != nil {
				t.Error("no flight info should be recorded for a failed order")
			}
		})
	}
}

func TestCreateOrderHighPass(t *testing.T) {
	// highPassCall builds the common HighPass setup: the queue is keyed by product and date,
	// and the passenger details come from hpCodePaxInfo under that same key.
	const (
		hpCode      = "service-1"
		productDate = "2026-09-18"
	)
	key := hpCode + "|" + productDate

	newCall := func(paxInfo PaxInfo) *createOrderCall {
		return &createOrderCall{
			supplier: SupplierData{
				Type: "highpass", APIUrl: "https://highpass.example", TokenExpiresAt: liveToken(),
				Token: "hp-tok", Password: "private-1", AiShortCode: "public-1", Email: "ops@ett.uz",
			},
			order:   OrderData{FirstName: "Ada", LastName: "Lovelace", ProductDate: productDate},
			product: ProductData{HpCode: hpCode},
			paxInfo: map[string]PaxInfo{key: paxInfo},
		}
	}

	queued := func(t *testing.T, call *createOrderCall) HighPassOrderItem {
		t.Helper()

		order, ok := call.fastTrackOrders[key]
		if !ok {
			t.Fatalf("no order queued under %q, got %v", key, call.fastTrackOrders)
		}
		if len(order.Order.Orders) != 1 {
			t.Fatalf("got %d order items, want exactly 1", len(order.Order.Orders))
		}

		return order.Order.Orders[0]
	}

	t.Run("queues the order instead of calling the supplier", func(t *testing.T) {
		call := newCall(PaxInfo{Adults: 2, Children: 1, Offset: "5:00", FlightNumber: "HY601", Names: []string{"Grace Hopper", "Alan Turing"}})

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		// HighPass orders are batched later, so nothing comes back on this call.
		if response.CouponCode != "" {
			t.Errorf("got coupon %q, want none until the batch is sent", response.CouponCode)
		}
		if call.orderItems[0]["highPassKey"] != key {
			t.Errorf("got highPassKey %v, want %q", call.orderItems[0]["highPassKey"], key)
		}

		item := queued(t, call)
		if item.ServiceID != hpCode || item.FirstName != "Ada" || item.LastName != "Lovelace" {
			t.Errorf("got %+v, want the passenger on service-1", item)
		}
		if item.AdultCount != 2 || item.ChildCount != 1 {
			t.Errorf("got %d adults and %d children, want 2 and 1", item.AdultCount, item.ChildCount)
		}
		if item.FlightNumber != "HY601" {
			t.Errorf("got flight %q, want HY601", item.FlightNumber)
		}
		if item.OtherPassengersContactDetails != "Grace Hopper, Alan Turing" {
			t.Errorf("got companions %q, want the names joined by commas", item.OtherPassengersContactDetails)
		}
		if !item.IsDateTimeOfPassengersArrivalToAirport {
			t.Error("the service date is the passengers' arrival time and must be flagged as such")
		}
		if item.Culture != "en-US" {
			t.Errorf("got culture %q, want en-US", item.Culture)
		}

		envelope := call.fastTrackOrders[key]
		if envelope.Order.PublicAPIKey != "public-1" || envelope.PrivateKey != "private-1" || envelope.Token != "hp-tok" {
			t.Errorf("got %+v, want the supplier's keys carried into the queued order", envelope)
		}
	})

	t.Run("falls back to a placeholder flight number", func(t *testing.T) {
		call := newCall(PaxInfo{Adults: 1, Offset: "5:00"})

		if _, errResponse := call.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if item := queued(t, call); item.FlightNumber != "BA396" {
			t.Errorf("got flight %q, want the BA396 placeholder", item.FlightNumber)
		}
	})

	t.Run("sends the traveler's phone, or the previous number when the order has none", func(t *testing.T) {
		withPhone := newCall(PaxInfo{Adults: 1, Offset: "5:00", Phone: "+447700900123"})
		if _, errResponse := withPhone.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if item := queued(t, withPhone); item.Phone != "+447700900123" {
			t.Errorf("got phone %q, want the traveler's", item.Phone)
		}

		withoutPhone := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		if _, errResponse := withoutPhone.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if item := queued(t, withoutPhone); item.Phone != "+998945553322" {
			t.Errorf("got phone %q, want the previous number", item.Phone)
		}
	})

	t.Run("spells out the baggage and companion counts in the comment", func(t *testing.T) {
		call := newCall(PaxInfo{Adults: 1, Offset: "5:00", BaggageCount: 2, CompanionsCount: 3, Comment: "VIP; "})

		if _, errResponse := call.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}

		item := queued(t, call)
		if !strings.HasPrefix(item.Comment, "VIP; ") {
			t.Errorf("got comment %q, want the original comment kept in front", item.Comment)
		}
		if !strings.Contains(item.Comment, "2 baggage(s)") || !strings.Contains(item.Comment, "3 companion(s)") {
			t.Errorf("got comment %q, want both counts spelled out", item.Comment)
		}
	})

	t.Run("takes the service date from the flight when there is one", func(t *testing.T) {
		flightInfo := `{"flightNumber":"HY601","departureTime":"2026-09-18 08:30:00","arrivalTime":"2026-09-18 12:45:00"}`

		departure := newCall(PaxInfo{Adults: 1, Offset: "5:00", TerminalType: "departure"})
		departure.orderItemData = map[string]interface{}{"flight_info": flightInfo}
		if _, errResponse := departure.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if item := queued(t, departure); item.ServiceDate != "2026-09-18 08:30:00" {
			t.Errorf("got service date %q, want the departure time", item.ServiceDate)
		}

		for _, terminalType := range []string{"arrival", "transit"} {
			arriving := newCall(PaxInfo{Adults: 1, Offset: "5:00", TerminalType: terminalType})
			arriving.orderItemData = map[string]interface{}{"flight_info": flightInfo}
			if _, errResponse := arriving.run(t); errResponse.StatusCode != 0 {
				t.Fatalf("unexpected error response: %+v", errResponse)
			}
			if item := queued(t, arriving); item.ServiceDate != "2026-09-18 12:45:00" {
				t.Errorf("%s: got service date %q, want the arrival time", terminalType, item.ServiceDate)
			}
		}
	})

	t.Run("falls back to the working windows when there is no flight", func(t *testing.T) {
		call := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		call.product.ServiceHours = []ServiceHour{{Open: "04:00", Close: "21:00"}}

		if _, errResponse := call.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if item := queued(t, call); item.ServiceDate != "2026-09-18 04:10:00" {
			t.Errorf("got service date %q, want the opening time plus the buffer", item.ServiceDate)
		}
	})

	t.Run("keeps the bare date when the windows already allow it", func(t *testing.T) {
		call := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		call.product.ServiceHours = []ServiceHour{{Open: "00:00", Close: "23:59"}}

		if _, errResponse := call.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if item := queued(t, call); item.ServiceDate != productDate {
			t.Errorf("got service date %q, want the bare date %q", item.ServiceDate, productDate)
		}
	})

	t.Run("pushes a same-day order without windows ten minutes out", func(t *testing.T) {
		today := time.Now().Format(time.DateOnly)
		todaysKey := hpCode + "|" + today

		call := newCall(PaxInfo{Adults: 1, Offset: "0:00"})
		call.order.ProductDate = today
		call.paxInfo = map[string]PaxInfo{todaysKey: {Adults: 1, Offset: "0:00"}}

		if _, errResponse := call.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}

		order, ok := call.fastTrackOrders[todaysKey]
		if !ok {
			t.Fatalf("no order queued under %q", todaysKey)
		}
		serviceDate, err := time.Parse(time.DateTime, order.Order.Orders[0].ServiceDate)
		if err != nil {
			t.Fatalf("service date %q should carry a time for a same-day order: %v", order.Order.Orders[0].ServiceDate, err)
		}
		if drift := serviceDate.Sub(wallClock(t, time.Now().Add(10*time.Minute))); drift > time.Minute || drift < -time.Minute {
			t.Errorf("got %v, want about ten minutes out (off by %v)", serviceDate, drift)
		}
	})

	t.Run("queues one order per product and date, not per passenger", func(t *testing.T) {
		first := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		if _, errResponse := first.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}

		// A second passenger on the same product and date shares the queue the first one filled.
		second := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		second.order.FirstName = "Grace"
		second.order.LastName = "Hopper"
		second.fastTrackOrders = first.fastTrackOrders
		if _, errResponse := second.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}

		if len(second.fastTrackOrders) != 1 {
			t.Errorf("got %d queued orders, want them collapsed into one", len(second.fastTrackOrders))
		}
		if item := queued(t, second); item.FirstName != "Ada" {
			t.Errorf("got %q, want the first passenger to stay the lead booker", item.FirstName)
		}
	})

	t.Run("fails on unparsable flight info", func(t *testing.T) {
		call := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		call.orderItemData = map[string]interface{}{"flight_info": "{not json"}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
		if len(call.fastTrackOrders) != 0 {
			t.Error("nothing should be queued for an order that failed")
		}
	})

	t.Run("fails on an unparsable timezone offset with working windows", func(t *testing.T) {
		call := newCall(PaxInfo{Adults: 1, Offset: "not-an-offset"})
		call.product.ServiceHours = []ServiceHour{{Open: "04:00", Close: "21:00"}}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
	})

	t.Run("fails on an unparsable stored expiry", func(t *testing.T) {
		call := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		call.supplier.TokenExpiresAt = "not-a-timestamp"

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
	})

	t.Run("refreshes an expired token before queueing", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"access_token":"fresh-hp-tok","expires_in":3600}`))
		})

		call := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		call.supplier.APIUrl = server.URL
		call.supplier.TokenExpiresAt = staleToken()
		call.ucodeBaseURL = ucodeServer(t, http.StatusOK)

		if _, errResponse := call.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if token := call.fastTrackOrders[key].Token; token != "fresh-hp-tok" {
			t.Errorf("got token %q, want the refreshed fresh-hp-tok", token)
		}
	})

	t.Run("fails when the refreshed token cannot be stored", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"access_token":"fresh-hp-tok","expires_in":3600}`))
		})

		call := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		call.supplier.APIUrl = server.URL
		call.supplier.TokenExpiresAt = staleToken()
		call.ucodeBaseURL = ucodeServer(t, http.StatusInternalServerError)

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
	})

	t.Run("fails when the login fails", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		call := newCall(PaxInfo{Adults: 1, Offset: "5:00"})
		call.supplier.APIUrl = server.URL
		call.supplier.TokenExpiresAt = staleToken()

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
	})
}

func TestCreateOrderISG(t *testing.T) {
	t.Run("marks a non-production order as a test and returns the code", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"Error":false,"Data":{"Code":"ISG-777"}}`))
		})

		call := createOrderCall{
			supplier:   SupplierData{Type: "isg", APIUrl: server.URL, Password: "isg-key"},
			order:      OrderData{FirstName: "Ada", LastName: "Lovelace", TotalPax: 3},
			product:    ProductData{ISGCode: "isg-product-1"},
			additional: AdditionalData{EnvironmentId: "staging-env", ProdEnvID: "prod-env"},
		}

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if response.CouponCode != "ISG-777" {
			t.Errorf("got coupon %q, want ISG-777", response.CouponCode)
		}
		if got.Query.Get("isTest") != "true" {
			t.Errorf("got isTest=%q outside production, want true", got.Query.Get("isTest"))
		}
		if got.Query.Get("maxusecount") != "3" {
			t.Errorf("got maxusecount=%q, want the pax count 3", got.Query.Get("maxusecount"))
		}
	})

	t.Run("marks a production order as live", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"Error":false,"Data":{"Code":"ISG-777"}}`))
		})

		call := createOrderCall{
			supplier:   SupplierData{Type: "isg", APIUrl: server.URL, Password: "isg-key"},
			order:      OrderData{TotalPax: 1},
			product:    ProductData{ISGCode: "isg-product-1"},
			additional: AdditionalData{EnvironmentId: "prod-env", ProdEnvID: "prod-env"},
		}

		if _, errResponse := call.run(t); errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if got.Query.Get("isTest") != "false" {
			t.Errorf("got isTest=%q in production, want false", got.Query.Get("isTest"))
		}
	})

	t.Run("fails when the service cannot be created", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"Error":true,"ErrorMessage":"product not found"}`))
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "isg", APIUrl: server.URL},
			product:  ProductData{ISGCode: "isg-product-1"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
		if !strings.Contains(errResponse.TelegramErrorFile, "product not found") {
			t.Errorf("got %q, want the supplier's message attached", errResponse.TelegramErrorFile)
		}
	})
}

func TestCreateOrderWorldMobile(t *testing.T) {
	t.Run("orders an esim and returns the activated sim's details", func(t *testing.T) {
		var activatedSimID string
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, got *capturedRequest) {
			switch {
			case got.Path == "/api/orders/esim":
				w.Write([]byte(`{"data":{"sim_cards":[{"token":"sim-tok-1"}],"sim_card":{"iccid":"ordered-iccid"}}}`))
			case strings.HasPrefix(got.Path, "/api/sim_cards/"):
				activatedSimID = strings.TrimPrefix(got.Path, "/api/sim_cards/")
				w.Write([]byte(`{"sim_card":{"iccid":"8912","short_iccid":"12","lpa":"LPA:1","qrcode_base64":"QRBASE64"}}`))
			default:
				t.Errorf("unexpected path %q", got.Path)
			}
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "world_mobile", APIUrl: server.URL, Password: "wm-key", Email: "ops@ett.uz"},
			order:    OrderData{TotalPax: 1, AgentOrderItemId: "item-1", PhoneNumber: "+998901234567"},
			product:  ProductData{WorldMobile: "sku-1", CountryCode: "UZ"},
		}

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if response.CouponCode != "sim-tok-1" {
			t.Errorf("got coupon %q, want the sim token sim-tok-1", response.CouponCode)
		}
		if activatedSimID != "sim-tok-1" {
			t.Errorf("got activation for %q, want the ordered sim token", activatedSimID)
		}
		// The activation response, not the order response, is the source of the final details.
		if response.Iccid != "8912" || response.ShortIccid != "12" ||
			response.Lpa != "LPA:1" || response.ESIMQrCodeBase64 != "QRBASE64" {
			t.Errorf("got %+v, want the activated sim's details", response)
		}
	})

	t.Run("tops up an existing sim without activating it", func(t *testing.T) {
		activated := false
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, got *capturedRequest) {
			if strings.HasPrefix(got.Path, "/api/sim_cards/") {
				activated = true
			}
			w.Write([]byte(`{"data":{"sim_card":{"token":"topup-tok","iccid":"8912","short_iccid":"12","lpa":"LPA:1","qrcode_base64":"QRBASE64"}}}`))
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "world_mobile", APIUrl: server.URL, Password: "wm-key"},
			order:    OrderData{Iccid: "8912", AgentOrderItemId: "item-1", Topup: true},
			product:  ProductData{WorldMobile: "sku-1"},
		}

		response, errResponse := call.run(t)
		if errResponse.StatusCode != 0 {
			t.Fatalf("unexpected error response: %+v", errResponse)
		}
		if response.CouponCode != "topup-tok" {
			t.Errorf("got coupon %q, want the topped-up sim's token", response.CouponCode)
		}
		if response.Iccid != "8912" || response.ESIMQrCodeBase64 != "QRBASE64" {
			t.Errorf("got %+v, want the order response's sim details", response)
		}
		// An existing sim is already live, so a second activation would be wrong.
		if activated {
			t.Error("a top-up must not re-activate the sim")
		}
	})

	t.Run("fails when the order fails", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"error":"out_of_stock"}`))
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "world_mobile", APIUrl: server.URL},
			order:    OrderData{TotalPax: 1},
			product:  ProductData{WorldMobile: "sku-1"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
		if !strings.Contains(errResponse.TelegramErrorFile, "out_of_stock") {
			t.Errorf("got %q, want the supplier's error attached", errResponse.TelegramErrorFile)
		}
	})

	t.Run("fails when the activation fails", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, got *capturedRequest) {
			if strings.HasPrefix(got.Path, "/api/sim_cards/") {
				w.Write([]byte(`{"error":"already_activated"}`))
				return
			}
			w.Write([]byte(`{"data":{"sim_cards":[{"token":"sim-tok-1"}]}}`))
		})

		call := createOrderCall{
			supplier: SupplierData{Type: "world_mobile", APIUrl: server.URL},
			order:    OrderData{TotalPax: 1},
			product:  ProductData{WorldMobile: "sku-1", CountryCode: "UZ"},
		}

		_, errResponse := call.run(t)
		if errResponse.StatusCode != 422 {
			t.Errorf("got status %d, want 422", errResponse.StatusCode)
		}
		if !strings.Contains(errResponse.TelegramErrorFile, "already_activated") {
			t.Errorf("got %q, want the supplier's error attached", errResponse.TelegramErrorFile)
		}
	})
}

func TestCreateOrderIgnoresAnUnknownSupplierType(t *testing.T) {
	call := createOrderCall{
		supplier: SupplierData{Type: "not-a-supplier"},
		order:    OrderData{ProductDate: "2026-09-18"},
	}

	response, errResponse := call.run(t)
	if errResponse.StatusCode != 0 {
		t.Errorf("got status %d, want an untouched error response", errResponse.StatusCode)
	}
	if response != (CreateOrderResponse{}) {
		t.Errorf("got %+v, want an empty response", response)
	}
}
