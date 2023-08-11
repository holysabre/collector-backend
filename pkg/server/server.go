package server

import (
	"collector-agent/db"
	"collector-agent/util"
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	model_ipmi "collector-agent/models/ipmi"
	model_server "collector-agent/models/server"

	"github.com/go-redis/redis/v8"
)

const (
	CmdTimeout                  = 5 * time.Second
	BlacklistPrefix             = "server-blacklist:server#"
	BlacklistBaseEXMintues      = 10
	BlacklistBaseEXFloatMintues = 5
	RetryMaxTimes               = 5
)

type ServerCollector struct {
	Connection model_ipmi.Connection
	Server     *model_server.Server
}

func NewServerCollector(s *model_server.Server) *ServerCollector {
	return &ServerCollector{
		Server:     s,
		Connection: s.Connection,
	}
}

func (sc *ServerCollector) Collect() {
	// fmt.Printf("collect #%d start \n", sc.Server.ID)

	if sc.checkInBlacklist() {
		log.Printf("server#%d is in blacklist, skip get power\n", sc.Server.ID)
		return
	}

	try_times := 0
	for {
		if try_times > RetryMaxTimes {
			sc.pushToBlacklist()
			break
		}
		status, err := sc.getPower()
		if err != nil {
			fmt.Printf("server #%d get power failed, err: %v \n", sc.Server.ID, err.Error())
			try_times++
		} else {
			sc.Server.PowerStatus = status
		}
	}

	if sc.checkInBlacklist() {
		log.Printf("server#%d is in blacklist, skip get power reading \n", sc.Server.ID)
		return
	}

	try_times = 0
	for {
		if try_times > RetryMaxTimes {
			sc.pushToBlacklist()
			break
		}
		power, err := sc.PowerReading()
		if err != nil {
			fmt.Printf("server #%d get power reading failed, err: %v \n", sc.Server.ID, err.Error())
			try_times++
		} else {
			sc.Server.PowerReading = power
		}
	}

	// fmt.Println("collect done")
}

func (sc *ServerCollector) getPower() (string, error) {
	status := "Unknown"
	args := []string{"power", "status"}
	out, err := sc.run("ipmitool", args)
	if err != nil {
		return status, err
	}
	// fmt.Println(string(out))

	pattern := `Chassis Power is (on|off)`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return status, err
	}
	status = matches[1]
	// fmt.Printf("server #%d power status: %s \n", sc.Server.ID, status)

	sc.Server.Time = time.Now()

	return status, nil
}

func (sc *ServerCollector) PowerReading() (int, error) {
	args := []string{"dcmi", "power", "reading"}

	var out []byte
	var err error
	out, err = sc.run("ipmitool", args)
	if err != nil {
		return 0, err
	}

	pattern := `Instantaneous power reading:\s+([\d.]+)\s+Watts`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		power, _ := strconv.Atoi(matches[1])
		// fmt.Printf("server #%d power reading: %d \n", sc.Server.ID, power)
		return power, nil
	}

	if strings.ContainsAny(string(out), "DCMI request failed") {
		fmt.Println("run old ipmitool")
		oldArgs := []string{"sdr", "get", "Power"}
		rootDir := util.GetBinDir()
		oldCommand := rootDir + "/ipmitool"
		out, err = sc.run(oldCommand, oldArgs)
		if err != nil {
			return 0, err
		}
		pattern = `^\s*Sensor Reading\s+:\s+(\d+)`
		reg = regexp.MustCompile(pattern)
		matches = reg.FindStringSubmatch(string(out))
		if len(matches) > 1 {
			power, _ := strconv.Atoi(matches[1])
			return power, nil
		}
	}

	return 0, errors.New("cannot get power")
}

func (sc *ServerCollector) run(command string, appendArgs []string) ([]byte, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, CmdTimeout)
	defer cancel()

	args := []string{"-R", "2", "-H", sc.Connection.Hostname, "-U", sc.Connection.Username, "-P", sc.Connection.Password, "-I", "lanplus"}
	args = append(args, appendArgs...)
	cmd := exec.CommandContext(ctx, command, args...)
	// fmt.Println("cmd: ", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}

func (sc *ServerCollector) pushToBlacklist() {
	redisConn := db.NewRedisReadConnection()
	client := redisConn.GetClient()

	err := client.SetNX(context.Background(), sc.getCacheKey(), 1, sc.getRandomCacheTime())

	if err != nil {
		log.Println(err.String())
	}
	redisConn.CloseClient(client)
}

func (sc *ServerCollector) checkInBlacklist() bool {
	redisConn := db.NewRedisReadConnection()
	client := redisConn.GetClient()

	isExists, err := client.Exists(context.Background(), sc.getCacheKey()).Result()
	if err == redis.Nil {
		redisConn.CloseClient(client)
		log.Fatal("Public key not found")
	} else if err != nil {
		redisConn.CloseClient(client)
		log.Fatal(err)
	}

	redisConn.CloseClient(client)

	return isExists > 0
}

func (sc *ServerCollector) getCacheKey() string {
	return BlacklistPrefix + strconv.Itoa(int(sc.Server.ID))
}

func (sc *ServerCollector) getRandomCacheTime() time.Duration {

	base := BlacklistBaseEXMintues * 60
	floatNum := BlacklistBaseEXFloatMintues * 60
	min := base - floatNum
	max := base + floatNum

	randomNumber := rand.Intn(max-min+1) + min

	fmt.Printf("randomNumber: %v\n", randomNumber)

	randomDuration := time.Duration(randomNumber) * time.Second

	fmt.Printf("randomDuration: %v\n", randomNumber)

	return randomDuration
}
