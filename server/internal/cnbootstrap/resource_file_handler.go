package cnbootstrap

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"kairisei.local/server/internal/cpk"
)

type ResourceFileConfig struct {
	PatchRoot           string
	Catalog             []byte
	CPKFiles            []cpk.File
	GachaBanner         string
	FiveStarGachaBanner string
	HomeBanner          string
	HomeEventBanner     string
	GachaBanners        map[string]string
}

func NewResourceFileHandler(config ResourceFileConfig) (http.Handler, error) {
	delivery, err := cnPatchDeliveryFromCatalog(config.Catalog)
	if err != nil {
		return nil, err
	}
	router := chi.NewRouter()
	patch := cnBootstrapVersionedPatchFile([]string{config.PatchRoot}, delivery)
	router.Method(http.MethodGet, "/local/resources/patch/Android/patch/*", patch)
	router.Method(http.MethodHead, "/local/resources/patch/Android/patch/*", patch)
	files := cnBootstrapCPKResource(config.CPKFiles)
	router.Method(http.MethodGet, "/local/resources/cpk/*", files)
	router.Method(http.MethodHead, "/local/resources/cpk/*", files)
	servePNG := func(path string) http.HandlerFunc {
		return func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "image/png")
			writer.Header().Set("Cache-Control", "no-store")
			http.ServeFile(writer, request, path)
		}
	}
	if config.HomeBanner != "" {
		router.Get("/local/home/banner.png", servePNG(config.HomeBanner))
	}
	if config.HomeEventBanner != "" {
		router.Get("/local/home/event-banner.png", servePNG(config.HomeEventBanner))
	}
	if config.GachaBanner != "" {
		router.Get("/local/gacha/banner.png", servePNG(config.GachaBanner))
	}
	if config.FiveStarGachaBanner != "" {
		router.Get("/local/gacha/five-star-banner.png", servePNG(config.FiveStarGachaBanner))
	}
	for key, bannerPath := range config.GachaBanners {
		if key == "five_star_ticket" {
			continue
		}
		key := key
		bannerPath := bannerPath
		router.Get("/local/gacha/"+key+".png", servePNG(bannerPath))
	}
	return router, nil
}
