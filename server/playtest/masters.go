package playtest

import (
	"os"
	"path/filepath"
	"testing"

	"kairisei.local/server/internal/testfixture"
	"kairisei.local/server/localcn"
)

func Write(t *testing.T, root, configDir string) localcn.PlayOptions {
	t.Helper()
	return localcn.PlayOptions{
		Cards:             testfixture.WriteTestCardMaster(t, root),
		Explore:           testfixture.WriteTestExploreMaster(t, root),
		Story:             testfixture.WriteTestStoryMaster(t, root),
		Navi:              testfixture.WriteTestNaviMaster(t, root),
		Items:             testfixture.WriteTestItemMaster(t, root),
		Avatar:            testfixture.WriteTestAvatarMaster(t, root),
		Stamps:            testfixture.WriteTestStampMaster(t, root),
		Honors:            testfixture.WriteTestHonorMaster(t, root),
		PlayerProgression: filepath.Join(configDir, "cn602-player-progression-runtime.json"),
		LoginBonus:        filepath.Join(configDir, "cn602-login-bonus-runtime.json"),
	}
}

func Save(t *testing.T, root, configDir string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(configDir, "cn602-save-template.json"))
	if err != nil {
		t.Fatal(err)
	}
	savePath := filepath.Join(root, "cn602-save.json")
	if err := os.WriteFile(savePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return savePath
}
