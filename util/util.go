package util

import (
	"log"
	"strings"
)

func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func ReverseMap(m map[string]string) map[string]string {
	n := make(map[string]string, len(m))
	for k, v := range m {
		n[v] = k
	}
	return n
}

func GetKeyByOid(m map[string]string, oid string) string {
	for k, v := range m {
		if strings.HasPrefix(oid, v) {
			return k
		}
	}
	return ""
}
