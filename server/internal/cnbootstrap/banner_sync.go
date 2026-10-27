package cnbootstrap

import "kairisei.local/server/accountsync"

func bannerSourcesFromResources(resources *clientResources) []accountsync.BannerSource {
	if resources == nil {
		return nil
	}
	out := []accountsync.BannerSource{
		{URLPath: "/local/home/banner.png", File: resources.HomeBanner},
		{URLPath: "/local/home/event-banner.png", File: resources.HomeEventBanner},
		{URLPath: "/local/gacha/banner.png", File: resources.GachaBanner},
		{URLPath: "/local/gacha/five-star-banner.png", File: resources.FiveStarGachaBanner},
	}
	for key, file := range resources.GachaBanners {
		if key == "five_star_ticket" {
			continue
		}
		out = append(out, accountsync.BannerSource{URLPath: "/local/gacha/" + key + ".png", File: file})
	}
	return out
}
