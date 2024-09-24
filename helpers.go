package ettcodesdk

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	crypt_rand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cast"
)

func DoRequest(ctx context.Context, url string, method string, body interface{}, appId string, headers map[string]interface{}) ([]byte, error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}

	request, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	if appId != "" {
		request.Header.Add("Content-Type", "application/json")
		request.Header.Add("authorization", "API-KEY")
		request.Header.Add("X-API-KEY", appId)
	}

	for key, value := range headers {
		request.Header.Add(key, cast.ToString(value))
	}

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respByte, err := io.ReadAll(resp.Body)
	if cast.ToInt(resp.StatusCode) > 300 {
		if err != nil {
			return nil, errors.New(string(respByte) + err.Error())
		}
		return respByte, errors.New(string(respByte))
	}

	return respByte, err
}

func RemoveDuplicateStrings(arr []string, isLower bool) []string {
	// Use a map to track unique values
	uniqueMap := make(map[string]bool)
	var uniqueArr []string

	// Iterate over the array
	for _, val := range arr {
		// Check if the value is already in the map
		if _, exists := uniqueMap[val]; !exists {
			// If not, add it to the map and append to the unique array
			uniqueMap[val] = true

			if isLower {
				uniqueArr = append(uniqueArr, strings.ToLower(val))
			} else {
				uniqueArr = append(uniqueArr, val)
			}
		}
	}

	return uniqueArr
}

func Round(number float64, precision int) float64 {
	scale := math.Pow10(precision)
	return math.Round(number*scale) / scale
}

const (
	Lower = iota + 1
	Upper
	Number
	UpperNumber
	LowerUpper
	LowerNumber
	LowerUpperNumber
)

func GenerateRandomString(length int, cmd int) string {
	var letterBytes string

	switch cmd {
	case Lower:
		letterBytes = "abcdefghijklmnopqrstuvwxyz"
	case Upper:
		letterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	case Number:
		letterBytes = "0123456789"
	case LowerUpperNumber:
		letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	case UpperNumber:
		letterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	case LowerNumber:
		letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"
	case LowerUpper:
		letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	default:
		letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}

	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func ContainsLike(s []string, e string) bool {
	for _, a := range s {
		if strings.Contains(e, a) {
			return true
		}
	}
	return false
}

func ProjectQuery(projects []string) map[string]interface{} {
	var query = map[string]interface{}{}
	for _, value := range projects {
		if strings.Contains(value, ".") {
			var key = strings.ReplaceAll(value, ".", "_")
			query[key] = map[string]interface{}{"$first": "$" + value}
		}
	}

	return query
}

// t1 <= t2
func TimeBeforeAndEqual(t1, t2 time.Time) bool {
	if t1.Before(t2) || t1.Equal(t2) {
		return true
	}

	return false
}

// t1 >= t2
func TimeAfterAndEqual(t1, t2 time.Time) bool {
	if t1.After(t2) || t1.Equal(t2) {
		return true
	}

	return false
}

func CopyMapStringInterface(dest, src map[string]interface{}) {
	for key, value := range src {
		dest[key] = value
	}
}

// sortDescending sorts the slice of indices in descending order
func SortDescendingSliceInt(slice []int) {
	for i := 0; i < len(slice)-1; i++ {
		for j := i + 1; j < len(slice); j++ {
			if slice[i] < slice[j] {
				slice[i], slice[j] = slice[j], slice[i]
			}
		}
	}
}

// hashSHA256 hashes the input data using SHA256 algorithm
func HashSHA256(data string) string {
	hash := sha256.New()
	hash.Write([]byte(data))
	hashedData := hash.Sum(nil)
	return hex.EncodeToString(hashedData)
}

// verifySHA256 verifies if the input data matches the hashed data
func VerifySHA256(data, hashedData string) bool {
	return HashSHA256(data) == hashedData
}

func Encrypt(secretKey, plaintext string) (string, error) {
	aes, err := aes.NewCipher([]byte(secretKey))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return "", err
	}

	// We need a 12-byte nonce for GCM (modifiable if you use cipher.NewGCMWithNonceSize())
	// A nonce should always be randomly generated for every encryption.
	nonce := make([]byte, gcm.NonceSize())
	_, err = crypt_rand.Read(nonce)
	if err != nil {
		return "", err
	}

	// ciphertext here is actually nonce+ciphertext
	// So that when we decrypt, just knowing the nonce size
	// is enough to separate it from the ciphertext.
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return hex.EncodeToString(ciphertext), nil
}

func Decrypt(secretKey, ciphertext string) (string, error) {
	ciphertextBytes, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	ciphertext = string(ciphertextBytes)

	aes, err := aes.NewCipher([]byte(secretKey))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return "", err
	}

	// Since we know the ciphertext is actually nonce+ciphertext
	// And len(nonce) == NonceSize(). We can separate the two.
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, []byte(nonce), []byte(ciphertext), nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
