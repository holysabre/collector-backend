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
	"os"
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
	CmdTimeout                    = 10 * time.Second
	BlacklistPrefix               = "server-blacklist:server#"
	BlacklistBaseEXMintues        = 10
	BlacklistBaseEXFloatMintues   = 5
	RetryMaxTimes                 = 2
	IPMIVersionBaseEXMintues      = 2
	IPMIVersionBaseEXFloatMintues = 1
)

type ServerCollector struct {
	Connection model_ipmi.Connection
	Server     *model_server.Server
	IPMIBinary string
}

func NewServerCollector(s *model_server.Server) *ServerCollector {
	logger.Printf("server %d starting", s.ID)
	return &ServerCollector{
		Server:     s,
		Connection: s.Connection,
		IPMIBinary: "/app/bin/ipmitool",
	}
}

func (sc *ServerCollector) Collect() error {
	logger.Printf("server %d start collect", sc.Server.ID)
	if sc.checkInBlacklist() {
		errStr := fmt.Sprintf("server#%d is in blacklist, skip get power", sc.Server.ID)
		return errors.New(errStr)
	}

	sc.Server.Time = time.Now()

	logger.Printf("server %d start get power", sc.Server.ID)
	status, err := sc.getPower()
	if err != nil {
		_, err := sc.pushToBlacklist()
		logger.LogIfErr(err)
		logger.Printf("server %d push to blacklist", sc.Server.ID)
		errStr := fmt.Sprintf("server#%d get power failed, skipped ", sc.Server.ID)
		return errors.New(errStr)
	} else {
		sc.Server.PowerStatus = status
	}
	logger.Printf("server %d get power finished, value: %s", sc.Server.ID, status)

	logger.Printf("server %d start get power reading", sc.Server.ID)
	power, err := sc.getPowerReading()
	if err != nil {
		_, err := sc.pushToBlacklist()
		logger.LogIfErr(err)
		logger.Printf("server #%d push to blacklist", sc.Server.ID)
		errStr := fmt.Sprintf("server#%d get power reading failed, skipped ", sc.Server.ID)
		return errors.New(errStr)
	} else {
		sc.Server.PowerReading = power
	}
	logger.Printf("server %d get power reading finished, value: %d", sc.Server.ID, power)

	logger.Printf("server %d finished", sc.Server.ID)
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

	power, err = sc.getOldPowerReading()
	if err != nil {
		power, err = sc.getNewPowerReading()
	}

	return
}

func (sc *ServerCollector) isOldIPMIToolVersion() bool {
	redisReadConn := db.NewRedisReadConnection()
	redisReadClient := redisReadConn.GetClient()

	ver, err := redisReadClient.Get(context.Background(), sc.getIPMIVersionCacheKey()).Result()
	if err == nil {
		if err != redis.Nil {
			logger.Printf("get version from redis %s", ver)
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
	out, err = sc.run("ipmitool", []string{"fru"})
	if err != nil {
		logger.Println("check old form fru")
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

	logger.Printf("get version from cli %s", version)

	return needOldVersion
}

func (sc *ServerCollector) getIPMIVersionCacheKey() string {
	return fmt.Sprintf("ipmi_server:%s:version", sc.Connection.Hostname)
}

func (sc *ServerCollector) getOldPowerReading() (int, error) {
	logger.Println("getOldPowerReading")
	keywords := []string{"Power", "Total_Power", "Sys_Total_Power"}
	var out []byte
	var err error
	for _, keyword := range keywords {
		args := []string{"sdr", "get", keyword}
		out, err = sc.run(sc.IPMIBinary, args)
		// logger.Println(string(out))
		if err == nil {
			break
		}
		logger.Println(err.Error())
	}

	pattern := `Sensor Reading\s+:\s+(\d+)`
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
		logger.Printf("server #%d power reading: %d ", sc.Server.ID, power)
		sc.cacheCorrectIPMIToolVersion("new")
		return power, nil
	}

	return 0, errors.New("cannot get new power reading")
}

func (sc *ServerCollector) run(command string, appendArgs []string) ([]byte, error) {
	os.Setenv("CMD_ENV_SKIP_SHELL_EXPAND", "true")
	username := fmt.Sprintf(`%s`, sc.Connection.Username)
	password := fmt.Sprintf(`%s`, sc.Connection.Password)

	args := []string{"-H", sc.Connection.Hostname, "-U", username, "-P", password, "-I", "lanplus"}
	args = append(args, appendArgs...)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, CmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	logger.Println(cmd.String())
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
	redisConn := db.NewRedisWriteConnection()
	client := redisConn.GetClient()

	result := client.SetNX(context.Background(), sc.getCacheKey(), 1, sc.getRandomCacheTime(BlacklistBaseEXMintues, BlacklistBaseEXFloatMintues))

	redisConn.CloseClient(client)

	logger.Printf("server#%d push into blacklist", sc.Server.CabinetID)

	return result.Result()
}

func (sc *ServerCollector) checkInBlacklist() bool {
	redisConn := db.NewRedisReadConnection()
	client := redisConn.GetClient()

	ctx := context.Background()

	key := sc.getCacheKey()
	isExists, err := client.Exists(ctx, key).Result()
	if err != nil {
		logger.Printf("Unable To Connect Redis, key: %s", key)
		redisConn.CloseClient(client)
		return false
	}
	if err == redis.Nil {
		logger.Printf("Key not found, key: %s", key)
		redisConn.CloseClient(client)
		return false
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
