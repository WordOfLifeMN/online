package util

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"strings"
)

func ComputeHash(s string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(s))

	b64 := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", hash.Sum32())))
	return strings.Trim(b64, "=")
}
