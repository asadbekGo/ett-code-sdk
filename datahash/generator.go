package datahash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func HashSHA256(data string) string {
	hash := sha256.New()
	hash.Write([]byte(data))
	hashedData := hash.Sum(nil)
	return hex.EncodeToString(hashedData)
}

func VerifySHA256(data, hashedData string) bool {
	return HashSHA256(data) == hashedData
}

func ValidateDataHash(generated interface{}, client string) (bool, error) {
	datahashByte, err := json.Marshal(generated)
	if err != nil {
		return false, err
	}
	return VerifySHA256(string(datahashByte), client), nil
}

func GenerateDataHash(generated interface{}) (string, error) {
	datahashByte, err := json.Marshal(generated)
	if err != nil {
		return "", err
	}
	return HashSHA256(string(datahashByte)), nil
}
