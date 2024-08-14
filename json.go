package utils

import (
	"encoding/json"
)

func PrettyJSON(x any) string {
	j, _ := json.MarshalIndent(x, "", "    ")
	return string(j)
}
