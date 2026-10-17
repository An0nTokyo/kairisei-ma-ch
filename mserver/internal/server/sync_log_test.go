package server

import (
	"errors"
	"strings"
	"testing"

	"kairisei.local/server/accountsync"
)

func TestTransferLogLines(t *testing.T) {
	var lines []string
	write := func(line string) { lines = append(lines, line) }

	logList(write, 0, nil)
	logList(write, 2, nil)
	logList(write, 0, errors.New("account sync failed"))
	logTransfer(write, "借出", 1000001, nil, nil)
	logTransfer(write, "交还", 1000001, errors.New("refused"), nil)
	logTransfer(write, "交还", 1000001, nil, &accountsync.ChoiceRequiredError{ServerRevision: 4, PhoneRevision: 9})
	logTransfer(write, "保留手机进度", 1000001, nil, nil)
	logTransfer(write, "保留电脑进度", 1000002, errors.New("refused"), nil)

	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"获取账号列表\n账号列表 0 个",
		"获取账号列表\n账号列表 2 个",
		"账号列表失败: account sync failed",
		"借出 1000001\n完成 1000001",
		"交还 1000001\n失败 1000001: refused",
		"交还 1000001\n需要选择进度 1000001: 电脑 4，手机 9",
		"保留手机进度 1000001\n完成 1000001",
		"保留电脑进度 1000002\n失败 1000002: refused",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if lines[4] != "账号列表失败: account sync failed" {
		t.Fatalf("failure line = %q", lines[4])
	}
}
