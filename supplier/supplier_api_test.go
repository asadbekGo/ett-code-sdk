package supplier

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// capturedRequest records what a supplier function actually put on the wire, so the tests can
// assert on the path, headers and body the supplier will see rather than only on the return value.
type capturedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Body   string
}

// newSupplierServer stands in for a supplier API. The handler gets the captured request so a
// test can answer differently per endpoint, and the capture is readable after the call returns.
func newSupplierServer(t *testing.T, handler func(w http.ResponseWriter, got *capturedRequest)) (*httptest.Server, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}

		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.Query = r.URL.Query()
		captured.Header = r.Header.Clone()
		captured.Body = string(body)

		handler(w, captured)
	}))
	t.Cleanup(server.Close)

	return server, captured
}

// deadServer hands back the URL of a server that is already shut down, so the caller's
// client.Do fails with a connection error.
func deadServer(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	return url
}

// unparsableURL makes http.NewRequest itself fail, covering the request-construction branches.
const unparsableURL = "://not-a-url"

func TestLoginPPG(t *testing.T) {
	t.Run("posts the credentials and returns the token", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"id":"user-1","token":"tok-1","expires":"2026-09-18T10:00:00Z"}`))
		})

		login, notification, err := LoginPPG(LoginRequest{LoginName: "agent", Password: "secret", URL: server.URL})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if login.Token != "tok-1" || login.Expires != "2026-09-18T10:00:00Z" {
			t.Errorf("got %+v, want token tok-1 expiring 2026-09-18T10:00:00Z", login)
		}
		if got.Method != http.MethodPost || got.Path != "/api/fe/v1/user/login" {
			t.Errorf("got %s %s, want POST /api/fe/v1/user/login", got.Method, got.Path)
		}
		if contentType := got.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("got Content-Type %q, want application/json", contentType)
		}

		var sent LoginRequest
		if err := json.Unmarshal([]byte(got.Body), &sent); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if sent.LoginName != "agent" || sent.Password != "secret" {
			t.Errorf("got %+v, want loginName agent with password secret", sent)
		}
		// The URL is a transport detail and must not leak into the credentials payload.
		if sent.URL != "" {
			t.Errorf("got url %q in the login body, want it left out", sent.URL)
		}
	})

	t.Run("reports a non-200 status", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"bad credentials"}`))
		})

		_, notification, err := LoginPPG(LoginRequest{URL: server.URL})
		if err == nil {
			t.Fatal("expected an error for a 401 response")
		}
		if !strings.Contains(notification, "401") || !strings.Contains(notification, "bad credentials") {
			t.Errorf("notification %q should carry the status and the response body", notification)
		}
	})

	t.Run("reports an undecodable body", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`not json`))
		})

		if _, _, err := LoginPPG(LoginRequest{URL: server.URL}); err == nil {
			t.Fatal("expected an error for a non-JSON body")
		}
	})

	t.Run("reports an unreachable supplier", func(t *testing.T) {
		if _, _, err := LoginPPG(LoginRequest{URL: deadServer(t)}); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
	})

	t.Run("reports an unparsable url", func(t *testing.T) {
		if _, _, err := LoginPPG(LoginRequest{URL: unparsableURL}); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestGenerateCouponPPG(t *testing.T) {
	validInput := func(serverURL string) CouponInput {
		return CouponInput{
			URL: serverURL, Token: "tok-1",
			FirstName: "Ada", LastName: "Lovelace",
			StartDate: "2026-09-18", EndDate: "2026-09-20",
			AishortCode: "ETT", ProductValue: 3, LocationShortCode: 42,
		}
	}

	t.Run("sends the offer and strips the @ppg suffix from the coupon", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"status":1,"result":{"coupon":"ETT12345@ppg"}}`))
		})

		coupon, notification, err := GenerateCouponPPG(validInput(server.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if coupon != "ETT12345" {
			t.Errorf("got coupon %q, want ETT12345", coupon)
		}
		if got.Path != "/api/master/v1/coupon/generate" {
			t.Errorf("got path %q, want /api/master/v1/coupon/generate", got.Path)
		}
		if auth := got.Header.Get("Authorization"); auth != "Bearer tok-1" {
			t.Errorf("got Authorization %q, want Bearer tok-1", auth)
		}

		var sent GenerateRequest
		if err := json.Unmarshal([]byte(got.Body), &sent); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if sent.Offer != "PPGETT3HR" {
			t.Errorf("got offer %q, want PPGETT3HR", sent.Offer)
		}
		if sent.Prefix != "ETT" {
			t.Errorf("got prefix %q, want ETT", sent.Prefix)
		}
		if len(sent.LocationShortCode) != 1 || sent.LocationShortCode[0] != 42 {
			t.Errorf("got locationShortCode %v, want [42]", sent.LocationShortCode)
		}
	})

	t.Run("formats a fractional product value into the offer", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"status":1,"result":{"coupon":"ETT1"}}`))
		})

		input := validInput(server.URL)
		input.ProductValue = 1.5
		if _, _, err := GenerateCouponPPG(input); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var sent GenerateRequest
		if err := json.Unmarshal([]byte(got.Body), &sent); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if sent.Offer != "PPGETT1.5HR" {
			t.Errorf("got offer %q, want PPGETT1.5HR", sent.Offer)
		}
	})

	t.Run("rejects a missing offer by description", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"status":1,"description":"Could not find the offer in AI"}`))
		})

		_, notification, err := GenerateCouponPPG(validInput(server.URL))
		if err == nil {
			t.Fatal("expected an error when the offer is unknown to the supplier")
		}
		if !strings.Contains(notification, "Product value is not found") {
			t.Errorf("notification %q should name the missing product value", notification)
		}
	})

	t.Run("rejects a coupon status other than 1", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"status":0,"result":{"coupon":"ETT12345"}}`))
		})

		_, notification, err := GenerateCouponPPG(validInput(server.URL))
		if err == nil {
			t.Fatal("expected an error for coupon status 0")
		}
		if !strings.Contains(notification, "Invalid Coupon status:0") {
			t.Errorf("notification %q should name the invalid status", notification)
		}
	})

	t.Run("rejects a non-200 status", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		if _, _, err := GenerateCouponPPG(validInput(server.URL)); err == nil {
			t.Fatal("expected an error for a 500 response")
		}
	})

	t.Run("rejects an undecodable body", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{`))
		})

		if _, _, err := GenerateCouponPPG(validInput(server.URL)); err == nil {
			t.Fatal("expected an error for a truncated body")
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		if _, _, err := GenerateCouponPPG(validInput(deadServer(t))); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := GenerateCouponPPG(validInput(unparsableURL)); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestGenerateCouponDF(t *testing.T) {
	validRequest := func(serverURL string) GenerateCouponDFRequest {
		return GenerateCouponDFRequest{
			URL: serverURL, Username: "key-1", Password: "secret-1",
			ProgramId: 7, FirstName: "AdaLovelace", RequestIdentifier: "txn-1",
			OutletId: "outlet-1", ValidFrom: "2026-09-18", BookingReferenceNo: "item-1", TotalVisit: 2,
		}
	}

	t.Run("posts to the unversioned endpoint and returns the voucher", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"status":true,"data":{"voucher_code":"DF-1","logo_url":"logo-1","logo_url_2":"logo-2"}}`))
		})

		coupon, notification, err := GenerateCouponDF(validRequest(server.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if coupon.Data.VoucherCode != "DF-1" || coupon.Data.LogoUrl != "logo-1" || coupon.Data.LogoUrl2 != "logo-2" {
			t.Errorf("got %+v, want voucher DF-1 with both logos", coupon.Data)
		}
		if got.Path != "/api/get-voucher-outlet" {
			t.Errorf("got path %q, want /api/get-voucher-outlet", got.Path)
		}
		if got.Header.Get("key") != "key-1" || got.Header.Get("secret") != "secret-1" {
			t.Errorf("credentials must travel in the key/secret headers, got %v", got.Header)
		}

		var sent GenerateCouponDFAPIRequest
		if err := json.Unmarshal([]byte(got.Body), &sent); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if sent.ServiceId != "21" {
			t.Errorf("got service_id %q, want the fixed 21", sent.ServiceId)
		}
		if sent.ProgramId != 7 || sent.TotalVisit != 2 || sent.OutletId != "outlet-1" {
			t.Errorf("got %+v, want program 7, 2 visits, outlet-1", sent)
		}
	})

	t.Run("puts the api version into the path when one is set", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"status":true,"data":{"voucher_code":"DF-1"}}`))
		})

		request := validRequest(server.URL)
		request.ApiVersion = "v2"
		if _, _, err := GenerateCouponDF(request); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != "/api/v2/get-voucher-outlet" {
			t.Errorf("got path %q, want /api/v2/get-voucher-outlet", got.Path)
		}
	})

	t.Run("rejects a false status", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"status":false,"message":"no availability"}`))
		})

		_, notification, err := GenerateCouponDF(validRequest(server.URL))
		if err == nil {
			t.Fatal("expected an error when the supplier reports status false")
		}
		if !strings.Contains(notification, "no availability") {
			t.Errorf("notification %q should carry the supplier's message", notification)
		}
	})

	t.Run("rejects a non-200 status and an undecodable body", func(t *testing.T) {
		failing, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusBadGateway)
		})
		if _, _, err := GenerateCouponDF(validRequest(failing.URL)); err == nil {
			t.Fatal("expected an error for a 502 response")
		}

		garbled, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{`))
		})
		if _, _, err := GenerateCouponDF(validRequest(garbled.URL)); err == nil {
			t.Fatal("expected an error for a truncated body")
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		if _, _, err := GenerateCouponDF(validRequest(deadServer(t))); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := GenerateCouponDF(validRequest(unparsableURL)); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestLoginEveryLounge(t *testing.T) {
	t.Run("posts the client credentials form and converts the ttl into an expiry", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"access_token":"el-tok","expires_in":3600}`))
		})

		before := time.Now()
		login, notification, err := LoginEveryLounge(LoginRequest{AuthURL: server.URL, Username: "client-1", Password: "secret-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if login.Token != "el-tok" {
			t.Errorf("got token %q, want el-tok", login.Token)
		}

		expires, err := time.Parse(time.RFC3339, login.Expires)
		if err != nil {
			t.Fatalf("expiry %q is not RFC3339: %v", login.Expires, err)
		}
		if wantAtLeast := before.Add(time.Hour).Truncate(time.Second); expires.Before(wantAtLeast) {
			t.Errorf("got expiry %v, want an hour out from %v", expires, before)
		}

		if got.Path != "/connect/token" {
			t.Errorf("got path %q, want /connect/token", got.Path)
		}
		if contentType := got.Header.Get("Content-Type"); contentType != "application/x-www-form-urlencoded" {
			t.Errorf("got Content-Type %q, want application/x-www-form-urlencoded", contentType)
		}

		form, err := url.ParseQuery(got.Body)
		if err != nil {
			t.Fatalf("request body is not a form: %v", err)
		}
		if form.Get("grant_type") != "client_credentials" || form.Get("client_id") != "client-1" ||
			form.Get("client_secret") != "secret-1" || form.Get("scope") != "ResourceServerApi" {
			t.Errorf("got form %v, want the ResourceServerApi client-credentials grant", form)
		}
	})

	t.Run("rejects a non-200 status", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusForbidden)
		})

		_, notification, err := LoginEveryLounge(LoginRequest{AuthURL: server.URL})
		if err == nil {
			t.Fatal("expected an error for a 403 response")
		}
		if !strings.Contains(notification, "403") {
			t.Errorf("notification %q should name the status code", notification)
		}
	})

	t.Run("reports an undecodable body", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`not json`))
		})

		if _, _, err := LoginEveryLounge(LoginRequest{AuthURL: server.URL}); err == nil {
			t.Fatal("expected an error for a non-JSON body")
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		if _, _, err := LoginEveryLounge(LoginRequest{AuthURL: deadServer(t)}); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := LoginEveryLounge(LoginRequest{AuthURL: unparsableURL}); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestCreateOrderAllAirports(t *testing.T) {
	validRequest := func(serverURL string) AAGenerateCouponRequest {
		return AAGenerateCouponRequest{
			URL: serverURL, Token: "aa-tok", SupplierAiShortCode: "contract-1",
			FirstName: "Ada", LastName: "Lovelace", DestinationCity: "TAS",
			VisitDate: "2026-09-18", AACode: "resource-1",
			ContactName: "Support", ContactEmail: "support@ett.uz", ContactPhone: "+998901234567",
		}
	}

	t.Run("posts the order and returns the decoded response", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"id":501,"status":"Created"}`))
		})

		response, notification, err := CreateOrderAllAirports(validRequest(server.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if response["status"] != "Created" {
			t.Errorf("got %v, want the decoded supplier response", response)
		}
		if got.Method != http.MethodPost || got.Path != "/api/v0/orders" {
			t.Errorf("got %s %s, want POST /api/v0/orders", got.Method, got.Path)
		}
		if auth := got.Header.Get("Authorization"); auth != "Bearer aa-tok" {
			t.Errorf("got Authorization %q, want Bearer aa-tok", auth)
		}

		var sent AAGenerateRequest
		if err := json.Unmarshal([]byte(got.Body), &sent); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if sent.Contract.ID != "contract-1" || sent.Type != "Standard" {
			t.Errorf("got contract %+v of type %q, want contract-1 / Standard", sent.Contract, sent.Type)
		}
		if len(sent.Passengers) != 1 || sent.Passengers[0].GivenName != "Ada" || sent.Passengers[0].FamilyName != "Lovelace" {
			t.Errorf("got passengers %+v, want a single Ada Lovelace", sent.Passengers)
		}
		// The supplier requires a date of birth; the code sends a synthetic 20-years-ago one.
		birth, err := time.Parse("2006-01-02T15:04:05", sent.Passengers[0].DateOfBirth)
		if err != nil {
			t.Fatalf("date of birth %q is not in the supplier's format: %v", sent.Passengers[0].DateOfBirth, err)
		}
		if age := time.Since(birth); age < 19*365*24*time.Hour {
			t.Errorf("got a date of birth %v ago, want roughly 20 years", age)
		}
		if len(sent.Resources) != 1 || sent.Resources[0].Resources.ID != "resource-1" {
			t.Fatalf("got resources %+v, want the single resource-1", sent.Resources)
		}
		flights := sent.Resources[0].Flights
		if len(flights) != 1 || flights[0].City.ID != "TAS" || flights[0].Type != "Departure" ||
			flights[0].Date != "2026-09-18" || flights[0].Number != "-" {
			t.Errorf("got flights %+v, want one placeholder departure from TAS on 2026-09-18", flights)
		}
		if sent.ContactEmail != "support@ett.uz" || sent.ContactPhone != "+998901234567" {
			t.Errorf("got contact %s / %s, want the supplier contact details", sent.ContactEmail, sent.ContactPhone)
		}
	})

	t.Run("rejects a 4xx or 5xx status", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"no availability"}`))
		})

		_, notification, err := CreateOrderAllAirports(validRequest(server.URL))
		if err == nil {
			t.Fatal("expected an error for a 400 response")
		}
		if !strings.Contains(notification, "400") || !strings.Contains(notification, "no availability") {
			t.Errorf("notification %q should carry the status and the response body", notification)
		}
	})

	t.Run("rejects an undecodable body", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`[`))
		})

		if _, _, err := CreateOrderAllAirports(validRequest(server.URL)); err == nil {
			t.Fatal("expected an error for a truncated body")
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		if _, _, err := CreateOrderAllAirports(validRequest(deadServer(t))); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := CreateOrderAllAirports(validRequest(unparsableURL)); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestPayAllAirports(t *testing.T) {
	t.Run("patches the order's pay endpoint offline by card", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"pnr":"PNR-1"}`))
		})

		response, notification, err := PayAllAirports(AAGenerateCouponRequest{URL: server.URL, Token: "aa-tok", OrderID: 501})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if response["pnr"] != "PNR-1" {
			t.Errorf("got %v, want the decoded pnr", response)
		}
		if got.Method != http.MethodPatch || got.Path != "/api/v0/orders/501/pay" {
			t.Errorf("got %s %s, want PATCH /api/v0/orders/501/pay", got.Method, got.Path)
		}

		var payload map[string]string
		if err := json.Unmarshal([]byte(got.Body), &payload); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if payload["mode"] != "Offline" || payload["type"] != "Card" {
			t.Errorf("got payment %v, want an offline card payment", payload)
		}
	})

	t.Run("refuses to call the supplier without an order id", func(t *testing.T) {
		called := false
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			called = true
		})

		_, notification, err := PayAllAirports(AAGenerateCouponRequest{URL: server.URL, Token: "aa-tok"})
		if err == nil {
			t.Fatal("expected an error when the order id is missing")
		}
		if !strings.Contains(notification, "OrderID are required") {
			t.Errorf("notification %q should name the missing order id", notification)
		}
		if called {
			t.Error("the supplier must not be called without an order id")
		}
	})

	t.Run("rejects a 4xx status and an undecodable body", func(t *testing.T) {
		failing, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusPaymentRequired)
		})
		if _, _, err := PayAllAirports(AAGenerateCouponRequest{URL: failing.URL, OrderID: 501}); err == nil {
			t.Fatal("expected an error for a 402 response")
		}

		garbled, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{`))
		})
		if _, _, err := PayAllAirports(AAGenerateCouponRequest{URL: garbled.URL, OrderID: 501}); err == nil {
			t.Fatal("expected an error for a truncated body")
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		if _, _, err := PayAllAirports(AAGenerateCouponRequest{URL: deadServer(t), OrderID: 501}); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := PayAllAirports(AAGenerateCouponRequest{URL: unparsableURL, OrderID: 501}); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestGetResourceID(t *testing.T) {
	t.Run("reads the resource by its aa code", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"id":"resource-1","organization":{"id":"org-1"}}`))
		})

		response, notification, err := GetResourceID(AAGenerateCouponRequest{URL: server.URL, Token: "aa-tok", AACode: "resource-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		organization, ok := response["organization"].(map[string]interface{})
		if !ok || organization["id"] != "org-1" {
			t.Errorf("got %v, want the nested organization id org-1", response)
		}
		if got.Method != http.MethodGet || got.Path != "/api/v0/resources/resource-1" {
			t.Errorf("got %s %s, want GET /api/v0/resources/resource-1", got.Method, got.Path)
		}
		if auth := got.Header.Get("Authorization"); auth != "Bearer aa-tok" {
			t.Errorf("got Authorization %q, want Bearer aa-tok", auth)
		}
	})

	t.Run("refuses to call the supplier without an aa code", func(t *testing.T) {
		called := false
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			called = true
		})

		_, notification, err := GetResourceID(AAGenerateCouponRequest{URL: server.URL, Token: "aa-tok"})
		if err == nil {
			t.Fatal("expected an error when the aa code is missing")
		}
		if !strings.Contains(notification, "AA Code are required") {
			t.Errorf("notification %q should name the missing aa code", notification)
		}
		if called {
			t.Error("the supplier must not be called without an aa code")
		}
	})

	t.Run("rejects a 4xx status and an undecodable body", func(t *testing.T) {
		failing, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusNotFound)
		})
		if _, _, err := GetResourceID(AAGenerateCouponRequest{URL: failing.URL, AACode: "resource-1"}); err == nil {
			t.Fatal("expected an error for a 404 response")
		}

		garbled, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{`))
		})
		if _, _, err := GetResourceID(AAGenerateCouponRequest{URL: garbled.URL, AACode: "resource-1"}); err == nil {
			t.Fatal("expected an error for a truncated body")
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		if _, _, err := GetResourceID(AAGenerateCouponRequest{URL: deadServer(t), AACode: "resource-1"}); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := GetResourceID(AAGenerateCouponRequest{URL: unparsableURL, AACode: "resource-1"}); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestLoginHighPass(t *testing.T) {
	t.Run("posts the api key as a third-party client and converts the ttl into an expiry", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"access_token":"hp-tok","expires_in":7200,"token_type":"Bearer"}`))
		})

		before := time.Now()
		login, _, err := LoginHighPass(LoginRequest{URL: server.URL, AiShortCode: "api-key-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if login.Token != "hp-tok" {
			t.Errorf("got token %q, want hp-tok", login.Token)
		}

		expires, err := time.Parse(time.RFC3339, login.Expires)
		if err != nil {
			t.Fatalf("expiry %q is not RFC3339: %v", login.Expires, err)
		}
		if wantAtLeast := before.Add(2 * time.Hour).Truncate(time.Second); expires.Before(wantAtLeast) {
			t.Errorf("got expiry %v, want two hours out from %v", expires, before)
		}

		if got.Path != "/api/v1/token" {
			t.Errorf("got path %q, want /api/v1/token", got.Path)
		}
		form, err := url.ParseQuery(got.Body)
		if err != nil {
			t.Fatalf("request body is not a form: %v", err)
		}
		if form.Get("grant_type") != "client_credentials" || form.Get("client_type") != "ThirdParty" || form.Get("api_key") != "api-key-1" {
			t.Errorf("got form %v, want a ThirdParty client-credentials grant for api-key-1", form)
		}
	})

	t.Run("rejects a non-200 status", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		if _, _, err := LoginHighPass(LoginRequest{URL: server.URL, AiShortCode: "api-key-1"}); err == nil {
			t.Fatal("expected an error for a 401 response")
		}
	})

	t.Run("reports an undecodable body", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`not json`))
		})

		if _, _, err := LoginHighPass(LoginRequest{URL: server.URL}); err == nil {
			t.Fatal("expected an error for a non-JSON body")
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		if _, _, err := LoginHighPass(LoginRequest{URL: deadServer(t)}); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := LoginHighPass(LoginRequest{URL: unparsableURL}); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestSha1ToBase64(t *testing.T) {
	// The reference SHA-1 of "abc", base64 encoded — pins the hash and the encoding together.
	if got, want := sha1ToBase64("abc"), "qZk+NkcGgWq6PiVxeFDCbJzQ2J0="; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if sha1ToBase64("a") == sha1ToBase64("b") {
		t.Error("different inputs must not hash to the same value")
	}
}

func TestCreateHighPass(t *testing.T) {
	validRequest := func(serverURL string) HighPassCrateOrderRequest {
		return HighPassCrateOrderRequest{
			URL: serverURL, Token: "hp-tok", PrivateKey: "private-1",
			Order: HighPassOrder{
				PublicAPIKey: "public-1",
				Orders: []HighPassOrderItem{{
					ServiceID: "service-1", ServiceDate: "2026-09-18 10:10:00",
					FirstName: "Ada", LastName: "Lovelace", AdultCount: 1, FlightNumber: "BA396",
				}},
			},
		}
	}

	t.Run("signs the base64 order and maps the first order back", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"orders":[{"bookingCode":"HP-1","highPassOrderId":"hp-order-1","isBarcode":true,"qrData":["qr-1","qr-2"]}]}`))
		})

		coupon, notification, err := CreateHighPass(validRequest(server.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if coupon.CouponCode != "HP-1" || coupon.OrderId != "hp-order-1" || !coupon.IsBarcode {
			t.Errorf("got %+v, want booking HP-1 / order hp-order-1 / barcode", coupon)
		}
		// Only the first QR entry is carried over.
		if coupon.QrData != "qr-1" {
			t.Errorf("got qr data %q, want the first entry qr-1", coupon.QrData)
		}
		if got.Path != "/api/v1/orders" {
			t.Errorf("got path %q, want /api/v1/orders", got.Path)
		}
		if auth := got.Header.Get("Authorization"); auth != "Bearer hp-tok" {
			t.Errorf("got Authorization %q, want Bearer hp-tok", auth)
		}

		var envelope HighPassHashedOrderRuqust
		if err := json.Unmarshal([]byte(got.Body), &envelope); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if want := sha1ToBase64("private-1" + envelope.Data + "private-1"); envelope.Signature != want {
			t.Errorf("got signature %q, want the private key wrapped around the payload: %q", envelope.Signature, want)
		}

		decoded, err := base64.StdEncoding.DecodeString(envelope.Data)
		if err != nil {
			t.Fatalf("data is not base64: %v", err)
		}
		var order HighPassOrder
		if err := json.Unmarshal(decoded, &order); err != nil {
			t.Fatalf("decoded data is not an order: %v", err)
		}
		if order.PublicAPIKey != "public-1" || len(order.Orders) != 1 || order.Orders[0].ServiceID != "service-1" {
			t.Errorf("got %+v, want the order signed intact", order)
		}
	})

	t.Run("leaves the qr data empty when the supplier sends none", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"orders":[{"bookingCode":"HP-1"}]}`))
		})

		coupon, _, err := CreateHighPass(validRequest(server.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if coupon.QrData != "" {
			t.Errorf("got qr data %q, want it empty", coupon.QrData)
		}
	})

	t.Run("rejects an empty order list", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"orders":[]}`))
		})

		_, notification, err := CreateHighPass(validRequest(server.URL))
		if err == nil {
			t.Fatal("expected an error when the supplier returns no orders")
		}
		if !strings.Contains(notification, "no orders found") {
			t.Errorf("notification %q should say no orders came back", notification)
		}
	})

	t.Run("surfaces a per-order error message", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"orders":[{"errorMessage":"service is not available in selected period"}]}`))
		})

		_, notification, err := CreateHighPass(validRequest(server.URL))
		if err == nil {
			t.Fatal("expected an error when the order carries an error message")
		}
		if !strings.Contains(notification, "service is not available in selected period") {
			t.Errorf("notification %q should quote the supplier's message", notification)
		}
	})

	t.Run("rejects a non-200 status and an undecodable body", func(t *testing.T) {
		failing, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		if _, _, err := CreateHighPass(validRequest(failing.URL)); err == nil {
			t.Fatal("expected an error for a 500 response")
		}

		garbled, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{`))
		})
		if _, _, err := CreateHighPass(validRequest(garbled.URL)); err == nil {
			t.Fatal("expected an error for a truncated body")
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		if _, _, err := CreateHighPass(validRequest(deadServer(t))); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := CreateHighPass(validRequest(unparsableURL)); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestCreateISGService(t *testing.T) {
	validRequest := func(serverURL string) ISGServiceRequest {
		return ISGServiceRequest{
			URL: serverURL, AuthKey: "isg-key", FirstName: "Ada", LastName: "Lovelace",
			ProductID: "isg-product-1", IsTest: true, MaxUseCount: 3,
		}
	}

	t.Run("puts the whole request in the query string", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"Error":false,"Data":{"Code":"ISG-1","PassUrl":"https://isg/pass"}}`))
		})

		result, notification, err := CreateISGService(validRequest(server.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if result.Data.Code != "ISG-1" {
			t.Errorf("got code %q, want ISG-1", result.Data.Code)
		}
		if got.Method != http.MethodPost || got.Path != "/premiumservices/create" {
			t.Errorf("got %s %s, want POST /premiumservices/create", got.Method, got.Path)
		}
		if got.Query.Get("authKey") != "isg-key" || got.Query.Get("firstname") != "Ada" ||
			got.Query.Get("lastname") != "Lovelace" || got.Query.Get("productid") != "isg-product-1" {
			t.Errorf("got query %v, want the passenger and product details", got.Query)
		}
		if got.Query.Get("isTest") != "true" || got.Query.Get("maxusecount") != "3" {
			t.Errorf("got isTest=%q maxusecount=%q, want true and 3", got.Query.Get("isTest"), got.Query.Get("maxusecount"))
		}
	})

	t.Run("leaves out a zero max use count and marks a live order", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"Error":false,"Data":{"Code":"ISG-1"}}`))
		})

		request := validRequest(server.URL)
		request.MaxUseCount = 0
		request.IsTest = false
		if _, _, err := CreateISGService(request); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Query.Has("maxusecount") {
			t.Errorf("got maxusecount=%q, want it left out entirely", got.Query.Get("maxusecount"))
		}
		if got.Query.Get("isTest") != "false" {
			t.Errorf("got isTest=%q, want false for a live order", got.Query.Get("isTest"))
		}
	})

	t.Run("surfaces a supplier-side error alongside the decoded result", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"Error":true,"ErrorMessage":"product not found","ErrorCode":404}`))
		})

		result, notification, err := CreateISGService(validRequest(server.URL))
		if err == nil {
			t.Fatal("expected an error when the supplier reports Error true")
		}
		if !strings.Contains(notification, "product not found") {
			t.Errorf("notification %q should quote the supplier's message", notification)
		}
		// The decoded body still comes back, so the caller can inspect the error code.
		if result.ErrorCode != 404 {
			t.Errorf("got error code %d, want the decoded 404", result.ErrorCode)
		}
	})

	t.Run("rejects a non-200 status and an undecodable body", func(t *testing.T) {
		failing, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})
		if _, _, err := CreateISGService(validRequest(failing.URL)); err == nil {
			t.Fatal("expected an error for a 503 response")
		}

		garbled, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{`))
		})
		if _, _, err := CreateISGService(validRequest(garbled.URL)); err == nil {
			t.Fatal("expected an error for a truncated body")
		}
	})

	t.Run("reports transport failures", func(t *testing.T) {
		if _, _, err := CreateISGService(validRequest(deadServer(t))); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := CreateISGService(validRequest(unparsableURL)); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestCreateWorldMobileOrder(t *testing.T) {
	t.Run("orders an esim with a quantity", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"data":{"ref_number":"ref-1","sim_cards":[{"token":"sim-tok-1"}],"sim_card":{"iccid":"8912","short_iccid":"12","lpa":"LPA:1","qrcode_base64":"QRBASE64"}}}`))
		})

		result, notification, err := CreateWorldMobileOrder(WorldMobileRequest{
			URL: server.URL, AuthKey: "wm-key", Sku: "sku-1", Quantity: 2, BookingNumber: "item-1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if len(result.Data.SimCards) != 1 || result.Data.SimCards[0].Token != "sim-tok-1" {
			t.Errorf("got sim cards %+v, want the single sim-tok-1", result.Data.SimCards)
		}
		if got.Method != http.MethodPost || got.Path != "/api/orders/esim" {
			t.Errorf("got %s %s, want POST /api/orders/esim", got.Method, got.Path)
		}
		if auth := got.Header.Get("Authorization"); auth != "Bearer wm-key" {
			t.Errorf("got Authorization %q, want Bearer wm-key", auth)
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(got.Body), &payload); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if payload["sku"] != "sku-1" || payload["quantity"] != float64(2) || payload["booking_number"] != "item-1" {
			t.Errorf("got payload %v, want sku-1 x2 for booking item-1", payload)
		}
		if _, ok := payload["iccid"]; ok {
			t.Error("a fresh esim order must not carry an iccid")
		}
	})

	t.Run("tops up an existing sim by iccid instead of quantity", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"data":{"sim_card":{"iccid":"8912","token":"topup-tok"}}}`))
		})

		result, _, err := CreateWorldMobileOrder(WorldMobileRequest{
			URL: server.URL, AuthKey: "wm-key", Sku: "sku-1", Quantity: 2,
			BookingNumber: "item-1", Iccid: "8912", Topup: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Data.SimCard.Token != "topup-tok" {
			t.Errorf("got token %q, want topup-tok", result.Data.SimCard.Token)
		}
		if got.Path != "/api/orders/topup" {
			t.Errorf("got path %q, want /api/orders/topup", got.Path)
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(got.Body), &payload); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if payload["iccid"] != "8912" {
			t.Errorf("got iccid %v, want the sim being topped up", payload["iccid"])
		}
		// A top-up targets one existing sim, so a quantity would be meaningless.
		if _, ok := payload["quantity"]; ok {
			t.Error("a top-up must not carry a quantity")
		}
	})

	t.Run("rejects an esim order that comes back without sim cards", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"data":{"ref_number":"ref-1","sim_cards":[]}}`))
		})

		_, notification, err := CreateWorldMobileOrder(WorldMobileRequest{URL: server.URL, Sku: "sku-1", Quantity: 1})
		if err == nil {
			t.Fatal("expected an error when no sim cards come back")
		}
		if !strings.Contains(notification, "no sim cards found") {
			t.Errorf("notification %q should say no sim cards came back", notification)
		}
	})

	t.Run("accepts a top-up without sim cards", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"data":{"sim_card":{"token":"topup-tok"}}}`))
		})

		if _, _, err := CreateWorldMobileOrder(WorldMobileRequest{URL: server.URL, Iccid: "8912", Topup: true}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("surfaces an error field with its message", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"error":"out_of_stock","message":"no sims left for this sku"}`))
		})

		_, notification, err := CreateWorldMobileOrder(WorldMobileRequest{URL: server.URL, Sku: "sku-1", Quantity: 1})
		if err == nil {
			t.Fatal("expected an error when the supplier sets the error field")
		}
		if !strings.Contains(notification, "out_of_stock") || !strings.Contains(notification, "no sims left for this sku") {
			t.Errorf("notification %q should carry both the error and the message", notification)
		}
	})

	t.Run("rejects a non-200 status carrying no error field", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{}`))
		})

		_, notification, err := CreateWorldMobileOrder(WorldMobileRequest{URL: server.URL, Sku: "sku-1", Quantity: 1})
		if err == nil {
			t.Fatal("expected an error for a 500 response")
		}
		if !strings.Contains(notification, "500") {
			t.Errorf("notification %q should name the status code", notification)
		}
	})

	t.Run("rejects an undecodable body and transport failures", func(t *testing.T) {
		garbled, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{`))
		})
		if _, _, err := CreateWorldMobileOrder(WorldMobileRequest{URL: garbled.URL, Sku: "sku-1"}); err == nil {
			t.Fatal("expected an error for a truncated body")
		}
		if _, _, err := CreateWorldMobileOrder(WorldMobileRequest{URL: deadServer(t), Sku: "sku-1"}); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := CreateWorldMobileOrder(WorldMobileRequest{URL: unparsableURL, Sku: "sku-1"}); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

func TestActivateWorldMobileOrder(t *testing.T) {
	t.Run("patches the sim card and returns its activated details", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"sim_card":{"iccid":"8912","short_iccid":"12","lpa":"LPA:1","qrcode_base64":"QRBASE64"}}`))
		})

		result, notification, err := ActivateWorldMobileOrder(ActivateWorldMobileRequest{
			URL: server.URL, AuthKey: "wm-key", SimCardId: "sim-tok-1", Country: "UZ", Email: "ops@ett.uz",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if notification != "" {
			t.Errorf("unexpected notification %q", notification)
		}
		if result.SimCard.Iccid != "8912" || result.SimCard.ShortIccid != "12" ||
			result.SimCard.Lpa != "LPA:1" || result.SimCard.QrcodeBase64 != "QRBASE64" {
			t.Errorf("got %+v, want the full activated sim details", result.SimCard)
		}
		if got.Method != http.MethodPatch || got.Path != "/api/sim_cards/sim-tok-1" {
			t.Errorf("got %s %s, want PATCH /api/sim_cards/sim-tok-1", got.Method, got.Path)
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(got.Body), &payload); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		if payload["country"] != "UZ" || payload["email"] != "ops@ett.uz" {
			t.Errorf("got payload %v, want the country and email", payload)
		}
	})

	t.Run("leaves out an empty country and email", func(t *testing.T) {
		server, got := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"sim_card":{"iccid":"8912"}}`))
		})

		if _, _, err := ActivateWorldMobileOrder(ActivateWorldMobileRequest{URL: server.URL, SimCardId: "sim-tok-1"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Body != "{}" {
			t.Errorf("got payload %q, want an empty object", got.Body)
		}
	})

	t.Run("rejects an activation that returns no iccid", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"sim_card":{"token":"sim-tok-1"}}`))
		})

		_, notification, err := ActivateWorldMobileOrder(ActivateWorldMobileRequest{URL: server.URL, SimCardId: "sim-tok-1"})
		if err == nil {
			t.Fatal("expected an error when the activated sim has no iccid")
		}
		if !strings.Contains(notification, "no iccid found") {
			t.Errorf("notification %q should say the iccid is missing", notification)
		}
	})

	t.Run("surfaces an error field with its message", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{"error":"already_activated","message":"this sim is already in use"}`))
		})

		_, notification, err := ActivateWorldMobileOrder(ActivateWorldMobileRequest{URL: server.URL, SimCardId: "sim-tok-1"})
		if err == nil {
			t.Fatal("expected an error when the supplier sets the error field")
		}
		if !strings.Contains(notification, "already_activated") || !strings.Contains(notification, "this sim is already in use") {
			t.Errorf("notification %q should carry both the error and the message", notification)
		}
	})

	t.Run("rejects a non-200 status carrying no error field", func(t *testing.T) {
		server, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{}`))
		})

		if _, _, err := ActivateWorldMobileOrder(ActivateWorldMobileRequest{URL: server.URL, SimCardId: "sim-tok-1"}); err == nil {
			t.Fatal("expected an error for a 500 response")
		}
	})

	t.Run("rejects an undecodable body and transport failures", func(t *testing.T) {
		garbled, _ := newSupplierServer(t, func(w http.ResponseWriter, _ *capturedRequest) {
			w.Write([]byte(`{`))
		})
		if _, _, err := ActivateWorldMobileOrder(ActivateWorldMobileRequest{URL: garbled.URL, SimCardId: "sim-1"}); err == nil {
			t.Fatal("expected an error for a truncated body")
		}
		if _, _, err := ActivateWorldMobileOrder(ActivateWorldMobileRequest{URL: deadServer(t), SimCardId: "sim-1"}); err == nil {
			t.Fatal("expected an error when the supplier refuses the connection")
		}
		if _, _, err := ActivateWorldMobileOrder(ActivateWorldMobileRequest{URL: unparsableURL, SimCardId: "sim-1"}); err == nil {
			t.Fatal("expected an error for an unparsable url")
		}
	})
}

// truncatedBodyServer promises more bytes than it sends and then drops the connection, so the
// caller's io.ReadAll fails part-way through the body. A supplier that dies mid-response is the
// one failure a well-formed error response cannot stand in for, and reading it as an empty body
// would look like a successful call with nothing in it.
func truncatedBodyServer(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("the test server cannot hijack connections")
			return
		}

		connection, buffered, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijacking the connection: %v", err)
			return
		}
		defer connection.Close()

		buffered.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 2048\r\n\r\n{\"partial\":")
		buffered.Flush()
	}))
	t.Cleanup(server.Close)

	return server.URL
}

func TestSupplierCallsRejectATruncatedResponseBody(t *testing.T) {
	calls := map[string]func(serverURL string) error{
		"LoginPPG": func(serverURL string) error {
			_, _, err := LoginPPG(LoginRequest{URL: serverURL})
			return err
		},
		"GenerateCouponPPG": func(serverURL string) error {
			_, _, err := GenerateCouponPPG(CouponInput{URL: serverURL})
			return err
		},
		"GenerateCouponDF": func(serverURL string) error {
			_, _, err := GenerateCouponDF(GenerateCouponDFRequest{URL: serverURL})
			return err
		},
		"LoginEveryLounge": func(serverURL string) error {
			_, _, err := LoginEveryLounge(LoginRequest{AuthURL: serverURL})
			return err
		},
		"CreateOrderAllAirports": func(serverURL string) error {
			_, _, err := CreateOrderAllAirports(AAGenerateCouponRequest{URL: serverURL})
			return err
		},
		"PayAllAirports": func(serverURL string) error {
			_, _, err := PayAllAirports(AAGenerateCouponRequest{URL: serverURL, OrderID: 501})
			return err
		},
		"GetResourceID": func(serverURL string) error {
			_, _, err := GetResourceID(AAGenerateCouponRequest{URL: serverURL, AACode: "resource-1"})
			return err
		},
		"LoginHighPass": func(serverURL string) error {
			_, _, err := LoginHighPass(LoginRequest{URL: serverURL})
			return err
		},
		"CreateHighPass": func(serverURL string) error {
			_, _, err := CreateHighPass(HighPassCrateOrderRequest{URL: serverURL})
			return err
		},
		"CreateISGService": func(serverURL string) error {
			_, _, err := CreateISGService(ISGServiceRequest{URL: serverURL})
			return err
		},
		"CreateWorldMobileOrder": func(serverURL string) error {
			_, _, err := CreateWorldMobileOrder(WorldMobileRequest{URL: serverURL, Sku: "sku-1"})
			return err
		},
		"ActivateWorldMobileOrder": func(serverURL string) error {
			_, _, err := ActivateWorldMobileOrder(ActivateWorldMobileRequest{URL: serverURL, SimCardId: "sim-1"})
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(truncatedBodyServer(t)); err == nil {
				t.Error("expected an error when the supplier cuts the response short")
			}
		})
	}
}
