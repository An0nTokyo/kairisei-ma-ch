package solo

import "kairisei.local/server/accountsync"

func FilePath(dataDir string) string {
	return accountsync.DefenseFilePath(dataDir)
}
