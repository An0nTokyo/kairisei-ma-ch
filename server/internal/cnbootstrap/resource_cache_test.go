package cnbootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kairisei.local/server/internal/testfixture"
)

func TestClientResourceCacheReusesCatalogUntilBundleChanges(t *testing.T) {
	root := t.TempDir()
	bannerDir := filepath.Join(root, "banners")
	if err := os.MkdirAll(bannerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range cn602GachaBannerFiles {
		if err := os.WriteFile(filepath.Join(bannerDir, name), []byte("png"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gacha := filepath.Join(bannerDir, "gacha.png")
	fiveStar := filepath.Join(bannerDir, "five.png")
	home := filepath.Join(bannerDir, "home.png")
	for _, path := range []string{gacha, fiveStar, home} {
		if err := os.WriteFile(path, []byte("png"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bannerDir, cn602HomeEventBannerFile), []byte("event"), 0o600); err != nil {
		t.Fatal(err)
	}

	cpkRoot := filepath.Join(root, "cpk")
	if err := os.MkdirAll(cpkRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	testfixture.WriteCPKVersions(t, cpkRoot)
	if err := os.WriteFile(filepath.Join(cpkRoot, "alpha.cpk"), []byte("cpk!"), 0o600); err != nil {
		t.Fatal(err)
	}

	patchRoot := filepath.Join(root, "patch")
	menuPath := filepath.Join(patchRoot, "main_c", "scenes", "scene_Menu.dat")
	if err := os.MkdirAll(filepath.Dir(menuPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(menuPath, []byte("menu"), 0o600); err != nil {
		t.Fatal(err)
	}
	version := []byte("<version>,790\n<bundle_ver>,main_c/scenes/scene_Menu.dat,0,93936638\n")
	for index := range version {
		version[index] += cn602ScrambleKey[index%len(cn602ScrambleKey)]
	}
	if err := os.WriteFile(filepath.Join(patchRoot, "version.dat"), version, 0o600); err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(root, "data", "client-resource-cache")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cache.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	config := ResourcesConfig{
		AssetMap:            writeTestAssetMap(t, root, version),
		GachaBanner:         gacha,
		FiveStarGachaBanner: fiveStar,
		HomeBanner:          home,
		CPKRoot:             cpkRoot,
		ImageRoot:           writeTestCardImages(t, root),
		CPKAliases:          writeTestCPKAliases(t, root),
		PatchRoots:          []string{patchRoot},
	}

	first, state, err := loadClientResources(config, cacheDir)
	if err != nil || state != "miss" || len(first.Catalog) == 0 || len(first.AssetNames) == 0 {
		t.Fatalf("first load state=%s catalog=%d names=%d err=%v", state, len(first.Catalog), len(first.AssetNames), err)
	}

	pngPath := filepath.Join(config.ImageRoot, "chr51", "chr51_10000010.png")
	original, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pngPath, bytes.Repeat([]byte{'x'}, len(original)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(pngPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	second, state, err := loadClientResources(config, cacheDir)
	if err != nil || state != "hit" {
		t.Fatalf("cached load state=%s err=%v", state, err)
	}
	if !bytes.Equal(first.Catalog, second.Catalog) {
		t.Fatal("cache hit changed the catalog bytes")
	}

	if err := os.WriteFile(pngPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(pngPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	menu, err := os.OpenFile(menuPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := menu.Write([]byte("!")); err != nil {
		_ = menu.Close()
		t.Fatal(err)
	}
	if err := menu.Close(); err != nil {
		t.Fatal(err)
	}

	third, state, err := loadClientResources(config, cacheDir)
	if err != nil || state != "miss" {
		t.Fatalf("rebuilt load state=%s err=%v", state, err)
	}
	if bytes.Equal(first.Catalog, third.Catalog) {
		t.Fatal("bundle size change reused the cached catalog")
	}
}
