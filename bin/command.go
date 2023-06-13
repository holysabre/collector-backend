package bin

import (
	"bytes"
	"collector-agent/util"
	"fmt"
	"os/exec"
	"strings"
)

func RunCommand(filename string, args ...string) ([]byte, error) {
	rootDir := util.GetBinDir()
	filename = rootDir + "/" + filename
	fmt.Println("cmd:", filename+" "+strings.Join(args, " "))
	cmd := exec.Command(filename, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}

	return out, nil
}

func RunCommandAndReturnBytes(filename string, args ...string) bytes.Buffer {
	rootDir := util.GetBinDir()
	filename = rootDir + "/" + filename
	cmd := exec.Command(filename, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("Run RunCommandAndReturnBytes Error: ", filename)
	}

	return out
}
