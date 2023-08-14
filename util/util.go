package util

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
)

func FailOnError(err error, msg string) {
	if err != nil {
		log.Printf("%s: %s", msg, err)
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

func LogIfErr(err error) {
	if err != nil {
		log.Printf("%s \n", err.Error())
	}
}

func GetRootDir() string {
	var dir string
	var err error
	if IsTesting() {
		dir, err = os.Getwd()
		if err != nil {
			fmt.Println(err)
		}
	} else {
		exePath, err := os.Executable()
		if err != nil {
			panic(err)
		}
		dir = filepath.Dir(exePath)
	}
	fmt.Println("root dir: ", dir)
	return dir
}

func GetBinDir() string {
	if IsTesting() {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
		}
		dir := wd + "/bin"

		return dir
	} else {
		fmt.Println("exec binary")
		exePath, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exeDir := filepath.Dir(exePath)
		templatesDir := filepath.Join(exeDir, "bin")
		fmt.Println("templatesDir:", templatesDir)
		return templatesDir
	}
}

func IsTesting() bool {
	args := os.Args
	return len(args) > 0 && strings.Contains(strings.ToLower(os.Args[0]), "go-build")
}

func RoundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
