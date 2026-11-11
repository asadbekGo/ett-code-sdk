package ettcodesdk

import (
	"encoding/json"
	"strings"
)

// Parts of a key that name a credential. A key containing one of them has its value replaced
// before a request leaves the function in a log. "token" also covers access_token, id_token and
// the like; "key" alone is the AES key some functions read from data.
var secretKeyParts = []string{"password", "secret", "token", "api_key", "apikey", "authorization", "credential", "cvv", "card_number"}

const redactedValue = "***"

// RedactSecrets returns a JSON request with every credential masked, at any depth: env.secret_key,
// data.password, password_copy, client_secret, api_key and the like. A body that is not JSON is not
// sent at all, since nothing in it could be told apart.
func RedactSecrets(raw []byte) []byte {
	var body interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return []byte(`{"redacted":"the request is not JSON and is not logged"}`)
	}

	redacted, err := json.MarshalIndent(redactValue(body), "", "  ")
	if err != nil {
		return []byte(`{"redacted":"the request could not be encoded"}`)
	}

	return redacted
}

func redactValue(value interface{}) interface{} {
	switch value := value.(type) {
	case map[string]interface{}:
		for key, item := range value {
			if isSecretKey(key) && item != nil && item != "" {
				value[key] = redactedValue
				continue
			}
			value[key] = redactValue(item)
		}
	case []interface{}:
		for i, item := range value {
			value[i] = redactValue(item)
		}
	}

	return value
}

func isSecretKey(key string) bool {
	key = strings.ToLower(key)
	if key == "key" {
		return true
	}

	for _, part := range secretKeyParts {
		if strings.Contains(key, part) {
			return true
		}
	}

	return false
}
