package ettcodesdk

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactSecretsMasksCredentialsAtAnyDepth(t *testing.T) {
	var request = `{
		"data": {
			"app_id": "P-app",
			"object_data": {
				"name": "Plaza Premium",
				"password": "Supplier!1",
				"password_v2": "Supplier!2",
				"attributes": [{"host": "sftp.example.com", "client_secret": "cs-1", "api_key": "ak-1"}],
				"access_token": "at-1",
				"sim_card": "8999",
				"empty_password": ""
			}
		},
		"env": {"branch_name": "test", "project_id": "p-1", "secret_key": "aes-key"}
	}`

	var got map[string]interface{}
	if err := json.Unmarshal(RedactSecrets([]byte(request)), &got); err != nil {
		t.Fatal(err)
	}

	text, _ := json.Marshal(got)
	for _, secret := range []string{"Supplier!1", "Supplier!2", "cs-1", "ak-1", "at-1", "aes-key"} {
		if strings.Contains(string(text), secret) {
			t.Errorf("%q is still in %s", secret, text)
		}
	}

	// What helps reading a failure stays.
	for _, kept := range []string{"P-app", "Plaza Premium", "sftp.example.com", "8999", `"branch_name":"test"`, `"empty_password":""`} {
		if !strings.Contains(string(text), kept) {
			t.Errorf("%q is missing from %s", kept, text)
		}
	}
}

func TestRedactSecretsDropsABodyThatIsNotJSON(t *testing.T) {
	if got := string(RedactSecrets([]byte("password=Supplier!1"))); strings.Contains(got, "Supplier!1") {
		t.Errorf("got %s", got)
	}
}
