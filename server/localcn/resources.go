package localcn

import (
	"net/http"
	"os"
	"path/filepath"

	"kairisei.local/server/internal/cnbootstrap"
	"kairisei.local/server/internal/cpk"
)

type ResourceFilesConfig struct {
	PatchRoot           string
	CPKRoot             string
	AliasPath           string
	CatalogPath         string
	GachaBanner         string
	FiveStarGachaBanner string
	HomeBanner          string
}

func OpenResourceFiles(config ResourceFilesConfig) (http.Handler, error) {
	catalog, err := os.ReadFile(config.CatalogPath)
	if err != nil {
		return nil, err
	}
	aliases, err := cpk.LoadAliases(config.CPKRoot, config.AliasPath)
	if err != nil {
		return nil, err
	}
	files, err := cpk.Load(config.CPKRoot, aliases)
	if err != nil {
		return nil, err
	}
	homeEvent := filepath.Join(filepath.Dir(config.HomeBanner), cnbootstrap.HomeEventBannerFileName())
	if _, err := os.Stat(homeEvent); err != nil {
		return nil, err
	}
	gachaBanners, err := cnbootstrap.ResolveGachaBannerPaths(config.GachaBanner, config.FiveStarGachaBanner)
	if err != nil {
		return nil, err
	}
	return cnbootstrap.NewResourceFileHandler(cnbootstrap.ResourceFileConfig{
		PatchRoot:           config.PatchRoot,
		Catalog:             catalog,
		CPKFiles:            files,
		GachaBanner:         config.GachaBanner,
		FiveStarGachaBanner: config.FiveStarGachaBanner,
		HomeBanner:          config.HomeBanner,
		HomeEventBanner:     homeEvent,
		GachaBanners:        gachaBanners,
	})
}
