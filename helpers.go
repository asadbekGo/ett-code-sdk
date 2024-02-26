package ettcodesdk

import (
	"bytes"
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

func DoRequest(url string, method string, body interface{}, appId string) ([]byte, error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		// Timeout: time.Duration(5 * time.Second),
	}

	request, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	request.Header.Add("authorization", "API-KEY")
	request.Header.Add("X-API-KEY", appId)

	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respByte, err := io.ReadAll(resp.Body)
	if cast.ToInt(resp.Status) > 300 {
		return nil, errors.New(string(respByte))
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
	rand.Seed(time.Now().UnixNano())
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
