package cnbootstrap

import (
	"path/filepath"

	"kairisei.local/server/internal/cpk"
)

// clientResources is a validated, read-only inventory shared by every account.
type clientResources struct {
	GachaBanner         string
	FiveStarGachaBanner string
	HomeBanner          string
	GachaBanners        map[string]string
	HomeEventBanner     string
	CPK                 []cpk.File
	Images              cardImages
	CPKList             []byte
	CPKState            []byte
	PatchRoots          []string
	Catalog             []byte
	Patch               []cnPatchDelivery
	AvailableBundles    map[string]struct{}
	AssetNames          map[string]bool
}

func loadClientResources(config ResourcesConfig, cacheDir string) (*clientResources, string, error) {
	absoluteGachaBanner, err := resolveBanner(config.GachaBanner, "gacha")
	if err != nil {
		return nil, "miss", err
	}
	absoluteFiveStarGachaBanner, err := resolveBanner(config.FiveStarGachaBanner, "five-star gacha")
	if err != nil {
		return nil, "miss", err
	}
	absoluteHomeBanner, err := resolveBanner(config.HomeBanner, "Home")
	if err != nil {
		return nil, "miss", err
	}
	homeEventBanner, err := resolveBanner(filepath.Join(filepath.Dir(absoluteHomeBanner), cn602HomeEventBannerFile), "Home event")
	if err != nil {
		return nil, "miss", err
	}
	gachaBannerPaths, err := resolveCN602GachaBannerPaths(absoluteGachaBanner, absoluteFiveStarGachaBanner)
	if err != nil {
		return nil, "miss", err
	}
	cpkAliases, err := cpk.LoadAliases(config.CPKRoot, config.CPKAliases)
	if err != nil {
		return nil, "miss", err
	}
	cpkDelivery, err := cpk.Load(config.CPKRoot, cpkAliases)
	if err != nil {
		return nil, "miss", err
	}
	cpkFileList := cpk.FileList(cpkDelivery)
	cpkPatchState := cpk.PatchState(cpkDelivery)
	resolvedPatchRoots, err := resolveCNPatchRoots(config.PatchRoots)
	if err != nil {
		return nil, "miss", err
	}
	if cached, ok := readClientResourceCache(cacheDir, config.ImageRoot, config.AssetMap, resolvedPatchRoots); ok {
		patchDelivery, err := cnPatchDeliveryFromCatalog(cached.Catalog)
		if err != nil {
			return nil, "miss", err
		}
		return &clientResources{
			GachaBanner:         absoluteGachaBanner,
			FiveStarGachaBanner: absoluteFiveStarGachaBanner,
			HomeBanner:          absoluteHomeBanner,
			GachaBanners:        gachaBannerPaths,
			HomeEventBanner:     homeEventBanner,
			CPK:                 cpkDelivery,
			Images:              cached.Images,
			CPKList:             cpkFileList,
			CPKState:            cpkPatchState,
			PatchRoots:          resolvedPatchRoots,
			Catalog:             cached.Catalog,
			Patch:               patchDelivery,
			AvailableBundles:    cached.AvailableBundles,
			AssetNames:          cached.AssetNames,
		}, "hit", nil
	}
	images, imageStamps, err := inspectCardImages(config.ImageRoot)
	if err != nil {
		return nil, "miss", err
	}
	built, err := buildCN602CatalogSet(resolvedPatchRoots, config.AssetMap)
	if err != nil {
		return nil, "miss", err
	}
	patchDelivery, err := cnPatchDeliveryFromCatalog(built.Content)
	if err != nil {
		return nil, "miss", err
	}
	if err := writeClientResourceCache(cacheDir, config.ImageRoot, config.AssetMap, resolvedPatchRoots, images, imageStamps, built); err != nil {
		return nil, "miss", err
	}
	return &clientResources{
		GachaBanner:         absoluteGachaBanner,
		FiveStarGachaBanner: absoluteFiveStarGachaBanner,
		HomeBanner:          absoluteHomeBanner,
		GachaBanners:        gachaBannerPaths,
		HomeEventBanner:     homeEventBanner,
		CPK:                 cpkDelivery,
		Images:              images,
		CPKList:             cpkFileList,
		CPKState:            cpkPatchState,
		PatchRoots:          resolvedPatchRoots,
		Catalog:             built.Content,
		Patch:               patchDelivery,
		AvailableBundles:    built.AvailableBundles,
		AssetNames:          built.AssetNames,
	}, "miss", nil
}
