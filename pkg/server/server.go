package server

import (
	"collector-agent/db"
	"collector-agent/pkg/logger"
	"collector-agent/util"
	"context"
	"errors"
	"fmt"
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

func (sc *ServerCollector) Collect() error {
	if sc.checkInBlacklist() {
		errStr := fmt.Sprintf("server#%d is in blacklist, skip get power\n", sc.Server.ID)
		return errors.New(errStr)
	}

	sc.Server.Time = time.Now()

	tryGetPowerTimes := 1
	for {
		if tryGetPowerTimes > RetryMaxTimes {
			_, err := sc.pushToBlacklist()
			logger.LogIfErrWithMsg(err, fmt.Sprintf("server #%d push to blacklist", sc.Server.ID))
			errStr := fmt.Sprintf("server#%d try %d times to get power, skipped \n", tryGetPowerTimes, sc.Server.ID)
			return errors.New(errStr)
		}
		status, err := sc.getPower()
		if err != nil {
			logger.Printf("server#%d get power failed, try times %d, err: %v \n", sc.Server.ID, tryGetPowerTimes, err.Error())
			tryGetPowerTimes++
			time.Sleep(1 * time.Second)
		} else {
			sc.Server.PowerStatus = status
			break
		}
	}

	tryGetPowerReadingtimes := 1
	for {
		if tryGetPowerReadingtimes > RetryMaxTimes {
			_, err := sc.pushToBlacklist()
			logger.LogIfErrWithMsg(err, fmt.Sprintf("server #%d push to blacklist", sc.Server.ID))
			errStr := fmt.Sprintf("server#%d try %d times to get power reading, skipped \n", tryGetPowerReadingtimes, sc.Server.ID)
			return errors.New(errStr)
		}
		power, err := sc.PowerReading()
		if err != nil {
			logger.Printf("server#%d get power reading failed, try times %d, err: %v \n", sc.Server.ID, tryGetPowerReadingtimes, err.Error())
			tryGetPowerReadingtimes++
			time.Sleep(1 * time.Second)
		} else {
			sc.Server.PowerReading = power
			break
		}
	}

	return nil
}

func (sc *ServerCollector) getPower() (string, error) {
	status := "Unknown"
	args := []string{"power", "status"}
	out, err := sc.run("ipmitool", args)
	if err != nil {
		logger.Println(string(out))
		return status, err
	}

	pattern := `Chassis Power is (on|off)`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return status, err
	}
	status = matches[1]
	logger.Printf("server #%d power status: %s \n", sc.Server.ID, status)

	return status, nil
}

func (sc *ServerCollector) PowerReading() (int, error) {
	args := []string{"dcmi", "power", "reading"}

	var out []byte
	var err error
	out, err = sc.run("ipmitool", args)
	if err != nil {
		logger.Println(string(out))
		return 0, err
	}

	pattern := `Instantaneous power reading:\s+([\d.]+)\s+Watts`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		power, _ := strconv.Atoi(matches[1])
		logger.Printf("server #%d power reading: %d \n", sc.Server.ID, power)
		return power, nil
	}

	if strings.ContainsAny(string(out), "DCMI request failed") {
		logger.Println("run old ipmitool")
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}

func (sc *ServerCollector) pushToBlacklist() (bool, error) {
	redisConn := db.NewRedisReadConnection()
	client := redisConn.GetClient()

	result := client.SetNX(context.Background(), sc.getCacheKey(), 1, sc.getRandomCacheTime())

	redisConn.CloseClient(client)

	return result.Result()
}

func (sc *ServerCollector) checkInBlacklist() bool {
	redisConn := db.NewRedisReadConnection()
	client := redisConn.GetClient()

	isExists, err := client.Exists(context.Background(), sc.getCacheKey()).Result()
	if err != nil {
		redisConn.CloseClient(client)
		logger.Fatal("Unable To Connect Redis")
	}
	if err == redis.Nil {
		redisConn.CloseClient(client)
		logger.Fatal("Public key not found")
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
	randomDuration := time.Duration(randomNumber) * time.Second
	logger.Printf("random Duration: %v\n", randomNumber)

	return randomDuration
}
