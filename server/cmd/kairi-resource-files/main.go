package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"kairisei.local/server/localcn"
)

func main() {
	listen := flag.String("listen", "192.168.137.1:26023", "address for patch, CPK, and banner downloads")
	patchRoot := flag.String("patch-root", "", "CN patch root")
	cpkRoot := flag.String("cpk-root", "", "CN CPK root")
	aliases := flag.String("cpk-aliases", "", "CN CPK alias manifest")
	catalog := flag.String("catalog", "", "catalog.dat bytes served to the client")
	gachaBanner := flag.String("gacha-banner", "", "default gacha banner PNG")
	fiveStarBanner := flag.String("five-star-banner", "", "five-star gacha banner PNG")
	homeBanner := flag.String("home-banner", "", "home gacha banner PNG")
	flag.Parse()
	if *patchRoot == "" || *cpkRoot == "" || *aliases == "" || *catalog == "" ||
		*gachaBanner == "" || *fiveStarBanner == "" || *homeBanner == "" {
		log.Fatal("patch, CPK, aliases, catalog, and banner paths are required")
	}
	handler, err := localcn.OpenResourceFiles(localcn.ResourceFilesConfig{
		PatchRoot:           *patchRoot,
		CPKRoot:             *cpkRoot,
		AliasPath:           *aliases,
		CatalogPath:         *catalog,
		GachaBanner:         *gachaBanner,
		FiveStarGachaBanner: *fiveStarBanner,
		HomeBanner:          *homeBanner,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("listening", *listen)
	log.Fatal(http.ListenAndServe(*listen, handler))
}
