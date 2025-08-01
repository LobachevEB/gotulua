package boolfunc

import (
	"fmt"
	"strings"
)

const (
	trueInDB  = "1"
	falseInDB = "0"
)

const (
	ToInternalFormat = iota
	ToUserFormat
)

func FormatBool(inp string, direction int) (string, error) {
	switch direction {
	case ToInternalFormat:
		up := strings.ToUpper(inp)
		if up == "1" || up == "0" {
			return up, nil
		}
		if strings.ToLower(inp) == "true" || strings.ToLower(inp) == "1" {
			return trueInDB, nil
		}
		return falseInDB, nil
	case ToUserFormat:
		if inp == trueInDB {
			return "true", nil
		}
		return "false", nil
	}
	return "", fmt.Errorf("invalid direction")
}
