package degrade

import (
	"encoding/json"
	"net/http"
)

const Message = "该功能需要连接电脑上的服务器"

func Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"res_code": -1,
			"res_str":  Message,
		})
	})
}
