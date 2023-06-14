package ipmi

type Connection struct {
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Interface string `json:"interface"`
}
