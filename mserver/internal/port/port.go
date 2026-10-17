package port

import (
	"errors"
	"strconv"
)

func Parse(text string) (int, error) {
	if len(text) == 0 || text[0] < '1' || text[0] > '9' {
		return 0, errors.New("本机端口无效")
	}
	for _, character := range text {
		if character < '0' || character > '9' {
			return 0, errors.New("本机端口无效")
		}
	}
	value, err := strconv.Atoi(text)
	if err != nil || value > 65534 {
		return 0, errors.New("本机端口无效")
	}
	return value, nil
}
