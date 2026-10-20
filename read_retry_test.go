package ettcodesdk

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// resettingAPI resets the connection of its first resets requests — closed
// with linger 0, so the client reads an RST, as from Cloudflare — and answers
// the rest with status and an empty list.
func resettingAPI(t *testing.T, resets int32, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if requests.Add(1) <= resets {
			conn, _, err := writer.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.(*net.TCPConn).SetLinger(0)
			_ = conn.Close()
			return
		}
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(`{"data":{"data":{"count":0,"response":[]}}}`))
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func objectFunction(baseUrl string) *ObjectFunction {
	return &ObjectFunction{Cfg: &Config{BaseURL: baseUrl, AppId: "app-1"}, Logger: NewLoggerFunction("sdk-test")}
}

func TestAReadIsSentAgainWhenItsConnectionIsReset(t *testing.T) {
	server, requests := resettingAPI(t, 2, http.StatusOK)

	_, _, err := objectFunction(server.URL).GetListAggregate(&Argument{TableSlug: "service_credential", Request: Request{Data: map[string]interface{}{}}})

	if err != nil {
		t.Fatalf("got %v, want the list after two resets", err)
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("server got %d requests, want 3", got)
	}
}

func TestAReadGivesUpAfterThreeResets(t *testing.T) {
	server, requests := resettingAPI(t, 10, http.StatusOK)

	_, _, err := objectFunction(server.URL).GetSingle(&Argument{TableSlug: "product", Request: Request{Data: map[string]interface{}{"guid": "g"}}})

	if err == nil {
		t.Fatal("got no error, want the reset")
	}
	if got := requests.Load(); got != readRequestAttempts {
		t.Fatalf("server got %d requests, want %d", got, readRequestAttempts)
	}
}

func TestAReadIsNotSentAgainForAnErrorAnswer(t *testing.T) {
	server, requests := resettingAPI(t, 0, http.StatusInternalServerError)

	_, _, err := objectFunction(server.URL).GetList(&Argument{TableSlug: "product", Request: Request{Data: map[string]interface{}{}}})

	if err == nil {
		t.Fatal("got no error, want the 500")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("server got %d requests, want 1", got)
	}
}

func TestAWriteIsNotSentAgainWhenItsConnectionIsReset(t *testing.T) {
	// The write may have been carried out before the reset; a second one
	// could create a second record.
	server, requests := resettingAPI(t, 1, http.StatusOK)

	_, _, err := objectFunction(server.URL).CreateObject(&Argument{TableSlug: "order", Request: Request{Data: map[string]interface{}{}}})

	if err == nil {
		t.Fatal("got no error, want the reset")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("server got %d requests, want 1", got)
	}
}
