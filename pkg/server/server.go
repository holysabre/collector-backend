package server

import (
	"collector-agent/db"
	"collector-agent/pkg/logger"
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
	CmdTimeout                    = 5 * time.Second
	BlacklistPrefix               = "server-blacklist:server#"
	BlacklistBaseEXMintues        = 10
	BlacklistBaseEXFloatMintues   = 5
	RetryMaxTimes                 = 3
	IPMIVersionBaseEXMintues      = 2
	IPMIVersionBaseEXFloatMintues = 1
)

type ServerCollector struct {
	Connection model_ipmi.Connection
	Server     *model_server.Server
	IPMIBinary string
}

func NewServerCollector(s *model_server.Server) *ServerCollector {
	return &ServerCollector{
		Server:     s,
		Connection: s.Connection,
		IPMIBinary: "/app/bin/ipmitool",
	}
}

func (sc *ServerCollector) Collect() error {
	if sc.checkInBlacklist() {
		errStr := fmt.Sprintf("server#%d is in blacklist, skip get power", sc.Server.ID)
		return errors.New(errStr)
	}

	sc.Server.Time = time.Now()

	tryGetPowerTimes := 1
	errStr := ""
	for {
		if tryGetPowerTimes > RetryMaxTimes {
			_, err := sc.pushToBlacklist()
			logger.LogIfErrWithMsg(err, fmt.Sprintf("server #%d push to blacklist", sc.Server.ID))
			errStr = fmt.Sprintf("server#%d try %d times to get power, skipped ", sc.Server.ID, tryGetPowerTimes)
			break
		}
		status, err := sc.getPower()
		if err != nil {
			logger.Printf("server#%d get power failed, try times %d, err: %v ", sc.Server.ID, tryGetPowerTimes, err.Error())
			time.Sleep(1 * time.Second)
		} else {
			sc.Server.PowerStatus = status
			break
		}
		tryGetPowerTimes++
	}
	if errStr != "" {
		return errors.New(errStr)
	}

	tryGetPowerReadingtimes := 1
	for {
		if tryGetPowerReadingtimes > RetryMaxTimes {
			_, err := sc.pushToBlacklist()
			logger.LogIfErrWithMsg(err, fmt.Sprintf("server #%d push to blacklist", sc.Server.ID))
			errStr = fmt.Sprintf("server#%d try %d times to get power reading, skipped ", sc.Server.ID, tryGetPowerReadingtimes)
			break
		}
		power, err := sc.getPowerReading()
		if err != nil {
			logger.Printf("server#%d get power reading failed, try times %d, err: %v ", sc.Server.ID, tryGetPowerReadingtimes, err.Error())
			time.Sleep(1 * time.Second)
		} else {
			fmt.Println("power ", power)
			sc.Server.PowerReading = power
			break
		}
		tryGetPowerReadingtimes++
	}
	if errStr != "" {
		return errors.New(errStr)
	}

	return nil
}

func (sc *ServerCollector) getPower() (string, error) {
	status := "Unknown"
	args := []string{"power", "status"}
	out, err := sc.run("ipmitool", args)
	if err != nil {
		logger.Println("getPower failed, " + string(out))
		return status, err
	}

	pattern := `Chassis Power is (on|off)`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return status, err
	}
	status = matches[1]
	logger.Printf("server #%d power status: %s ", sc.Server.ID, status)

	return status, nil
}

func (sc *ServerCollector) getPowerReading() (power int, err error) {
	needOldVersion := sc.isOldIPMIToolVersion()
	log.Println("needOldVersion:", needOldVersion)
	if needOldVersion {
		return sc.getOldPowerReading()
	}

	power, err = sc.getNewPowerReading()
	if err != nil {
		power, err = sc.getOldPowerReading()
	}

	return
}

func (sc *ServerCollector) isOldIPMIToolVersion() bool {
	redisReadConn := db.NewRedisReadConnection()
	redisReadClient := redisReadConn.GetClient()

	ver, err := redisReadClient.Get(context.Background(), sc.getIPMIVersionCacheKey()).Result()
	if err == nil {
		if err != redis.Nil {
			fmt.Println("get version from redis ", ver)
			if ver == "old" {
				return true
			} else {
				return false
			}
		}
	}
	logger.Printf("No Data From Redis, key: %s", sc.getIPMIVersionCacheKey())
	redisReadConn.CloseClient(redisReadClient)

	rootDir := util.GetRootDir()
	oldIPMIBinary := rootDir + "/bin/ipmitool"

	var out []byte
	out, err = sc.run(sc.IPMIBinary, []string{"fru"})
	if err != nil {
		fmt.Println("check old form fru")
		sc.IPMIBinary = oldIPMIBinary
		out, err = sc.run(sc.IPMIBinary, []string{"fru"})
		if err != nil {
			logger.Println(err.Error())
		}
	}

	needOldVersion := false
	keywords := []string{"Huawei"}
	for _, keyword := range keywords {
		isContains := strings.Contains(string(out), keyword)
		if isContains {
			sc.IPMIBinary = oldIPMIBinary
			needOldVersion = true
		}
	}

	version := "new"
	if needOldVersion {
		version = "old"
	}

	sc.cacheCorrectIPMIToolVersion(version)

	fmt.Println("get version from cli ", version)

	return needOldVersion
}

func (sc *ServerCollector) getIPMIVersionCacheKey() string {
	return fmt.Sprintf("ipmi_server:%s:version", sc.Connection.Hostname)
}

func (sc *ServerCollector) getOldPowerReading() (int, error) {
	logger.Println("getOldPowerReading")
	args := []string{"sdr", "get", "Power"}
	out, err := sc.run(sc.IPMIBinary, args)
	if err != nil {
		return 0, err
	}
	pattern := `^\s*Sensor Reading\s+:\s+(\d+)`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		power, _ := strconv.Atoi(matches[1])
		logger.Printf("server #%d power reading: %d ", sc.Server.ID, power)
		sc.cacheCorrectIPMIToolVersion("old")
		return power, nil
	}

	return 0, errors.New("cannot get old power reading")
}
func (sc *ServerCollector) getNewPowerReading() (int, error) {
	logger.Println("getNewPowerReading")
	args := []string{"dcmi", "power", "reading"}
	var out []byte
	var err error
	out, err = sc.run(sc.IPMIBinary, args)
	if err != nil {
		logger.Println(string(out))
		return 0, err
	}

	pattern := `Instantaneous power reading:\s+([\d.]+)\s+Watts`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		power, _ := strconv.Atoi(matches[1])
		logger.Printf("server #%d power reading: %d ", sc.Server.ID, power)
		sc.cacheCorrectIPMIToolVersion("new")
		return power, nil
	}

	return 0, errors.New("cannot get new power reading")
}

func (sc *ServerCollector) run(command string, appendArgs []string) ([]byte, error) {
	args := []string{"-R", "2", "-H", sc.Connection.Hostname, "-U", sc.Connection.Username, "-P", sc.Connection.Password, "-I", "lanplus"}
	args = append(args, appendArgs...)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, CmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Printf("power run failed, cmd: %v, out: %v", cmd.String(), string(out))
		return out, err
	}
	return out, nil
}

func (sc *ServerCollector) cacheCorrectIPMIToolVersion(version string) {
	redisWriteConn := db.NewRedisWriteConnection()
	redisWriteClient := redisWriteConn.GetClient()
	result := redisWriteClient.SetNX(context.Background(), sc.getIPMIVersionCacheKey(), version, sc.getRandomCacheTime(IPMIVersionBaseEXMintues, IPMIVersionBaseEXFloatMintues))
	log.Println(result.Result())

	redisWriteConn.CloseClient(redisWriteClient)
}

func (sc *ServerCollector) pushToBlacklist() (bool, error) {
	redisConn := db.NewRedisReadConnection()
	client := redisConn.GetClient()

	result := client.SetNX(context.Background(), sc.getCacheKey(), 1, sc.getRandomCacheTime(BlacklistBaseEXMintues, BlacklistBaseEXFloatMintues))

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

func (sc *ServerCollector) getRandomCacheTime(baseMinutes int, floatMiuntes int) time.Duration {
	base := baseMinutes * 60
	floatNum := floatMiuntes * 60
	min := base - floatNum
	max := base + floatNum

	randomNumber := rand.Intn(max-min+1) + min
	randomDuration := time.Duration(randomNumber) * time.Second
	// logger.Printf("random Duration: %v", randomNumber)

	return randomDuration
}
