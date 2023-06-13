package msg

type Msg struct {
	Type     string `json:"type"`
	Time     int64  `json:"time"`
	TryTimes int8   `json:"try_times"`
	Data     string `json:"data"`
}
