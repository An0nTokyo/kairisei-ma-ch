package server

import (
	"fmt"

	"kairisei.local/server/accountsync"
)

func logList(write func(string), count int, err error) {
	if err != nil {
		write("账号列表失败: " + err.Error())
		return
	}
	write("获取账号列表")
	write(fmt.Sprintf("账号列表 %d 个", count))
}

func logTransfer(write func(string), action string, userID int, err error, choice *accountsync.ChoiceRequiredError) {
	write(fmt.Sprintf("%s %d", action, userID))
	if choice != nil {
		write(fmt.Sprintf("需要选择进度 %d: 电脑 %d，手机 %d", userID, choice.ServerRevision, choice.PhoneRevision))
		return
	}
	if err != nil {
		write(fmt.Sprintf("失败 %d: %s", userID, err.Error()))
		return
	}
	write(fmt.Sprintf("完成 %d", userID))
}
