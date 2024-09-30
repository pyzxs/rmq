package rmq

import "encoding/json"

const (
	DEFAULT_QUEUE_NAME = "default"
	MAX_CACHE_LEN      = 1000
	DEFAULT_DSN_URL    = "amqp://guest:guest@127.0.0.1:5672/"
)

type IJob interface {
	JobHandle(string)
}

// 解析包数据
func parseMessage(data []byte) (string, string) {
	mp := map[string]string{}
	err := json.Unmarshal(data, &mp)

	if err != nil {
		return "", ""
	}

	return mp["type"], mp["body"]
}
