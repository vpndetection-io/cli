package lib

import "encoding/json"

// jsonUnmarshal keeps the test bodies readable by hiding the error dance.
func jsonUnmarshal(body string, v any) error {
	return json.Unmarshal([]byte(body), v)
}
