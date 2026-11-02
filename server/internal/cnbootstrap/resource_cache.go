package cnbootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const clientResourceCacheSchema = 1

type clientResourceCacheFile struct {
	Schema              int                         `json:"schema"`
	ImageManifestSHA256 string                      `json:"image_manifest_sha256"`
	VersionDATSHA256    string                      `json:"version_dat_sha256"`
	AssetMapSHA256      string                      `json:"asset_map_sha256"`
	PatchRoots          []string                    `json:"patch_roots"`
	ImageNamespace      string                      `json:"image_namespace"`
	Images              []clientResourceCacheImage  `json:"images"`
	Bundles             []clientResourceCacheBundle `json:"bundles"`
	AvailableBundles    []string                    `json:"available_bundles"`
	AssetNames          []string                    `json:"asset_names"`
	CatalogSHA256       string                      `json:"catalog_sha256"`
}

type clientResourceCacheImage struct {
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	ModUnixNano int64  `json:"mod_time_unix_nano"`
}

type clientResourceCacheBundle struct {
	Name        string `json:"name"`
	Present     bool   `json:"present"`
	Size        int64  `json:"size"`
	ModUnixNano int64  `json:"mod_time_unix_nano"`
	Root        string `json:"root"`
}

type cachedClientInventory struct {
	Images           cardImages
	Catalog          []byte
	AvailableBundles map[string]struct{}
	AssetNames       map[string]bool
}

func readClientResourceCache(cacheDir, imageRoot, assetMapPath string, patchRoots []string) (cachedClientInventory, bool) {
	var cached cachedClientInventory
	if cacheDir == "" || len(patchRoots) == 0 {
		return cached, false
	}
	recordPath := filepath.Join(cacheDir, "cache.json")
	catalogPath := filepath.Join(cacheDir, "catalog.bin")
	recordBytes, err := os.ReadFile(recordPath)
	if err != nil {
		return cached, false
	}
	var record clientResourceCacheFile
	if err := json.Unmarshal(recordBytes, &record); err != nil || record.Schema != clientResourceCacheSchema {
		return cached, false
	}
	fingerprint, err := hashClientResourceInputs(imageRoot, assetMapPath, patchRoots)
	if err != nil || !fingerprint.matches(record) {
		return cached, false
	}
	if !cachedImagesMatch(imageRoot, record.Images) || !cachedBundlesMatch(patchRoots, record.Bundles) {
		return cached, false
	}
	catalog, err := os.ReadFile(catalogPath)
	if err != nil {
		return cached, false
	}
	catalogDigest := sha256.Sum256(catalog)
	if hex.EncodeToString(catalogDigest[:]) != record.CatalogSHA256 {
		return cached, false
	}
	images, ok := cardImagesFromCache(imageRoot, record.ImageNamespace, record.Images)
	if !ok {
		return cached, false
	}
	cached.Images = images
	cached.Catalog = catalog
	cached.AvailableBundles = make(map[string]struct{}, len(record.AvailableBundles))
	for _, name := range record.AvailableBundles {
		cached.AvailableBundles[name] = struct{}{}
	}
	cached.AssetNames = make(map[string]bool, len(record.AssetNames))
	for _, name := range record.AssetNames {
		cached.AssetNames[name] = true
	}
	return cached, true
}

func writeClientResourceCache(cacheDir, imageRoot, assetMapPath string, patchRoots []string, images cardImages, stamps []cardImageStamp, built *cnCatalogBuild) error {
	if cacheDir == "" || built == nil {
		return nil
	}
	fingerprint, err := hashClientResourceInputs(imageRoot, assetMapPath, patchRoots)
	if err != nil {
		return err
	}
	record := clientResourceCacheFile{
		Schema:              clientResourceCacheSchema,
		ImageManifestSHA256: fingerprint.imageManifest,
		VersionDATSHA256:    fingerprint.versionDAT,
		AssetMapSHA256:      fingerprint.assetMap,
		PatchRoots:          append([]string(nil), patchRoots...),
		ImageNamespace:      images.Namespace,
		Images:              make([]clientResourceCacheImage, 0, len(stamps)),
		Bundles:             make([]clientResourceCacheBundle, 0, len(built.Bundles)),
		AvailableBundles:    make([]string, 0, len(built.AvailableBundles)),
		AssetNames:          make([]string, 0, len(built.AssetNames)),
		CatalogSHA256:       catalogDigestHex(built.Content),
	}
	for _, stamp := range stamps {
		record.Images = append(record.Images, clientResourceCacheImage{
			Path: stamp.Path, Bytes: stamp.Bytes, SHA256: stamp.SHA256,
			Size: stamp.Size, ModUnixNano: stamp.ModUnixNano,
		})
	}
	for _, stamp := range built.Bundles {
		record.Bundles = append(record.Bundles, clientResourceCacheBundle{
			Name: stamp.Name, Present: stamp.Present, Size: stamp.Size,
			ModUnixNano: stamp.ModUnixNano, Root: stamp.Root,
		})
	}
	for name := range built.AvailableBundles {
		record.AvailableBundles = append(record.AvailableBundles, name)
	}
	for name := range built.AssetNames {
		record.AssetNames = append(record.AssetNames, name)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return err
	}
	if err := replaceCacheFile(filepath.Join(cacheDir, "catalog.bin"), built.Content); err != nil {
		return err
	}
	return replaceCacheFile(filepath.Join(cacheDir, "cache.json"), encoded)
}

type clientResourceFingerprint struct {
	imageManifest string
	versionDAT    string
	assetMap      string
	patchRoots    []string
}

func (fingerprint clientResourceFingerprint) matches(record clientResourceCacheFile) bool {
	if fingerprint.imageManifest != record.ImageManifestSHA256 ||
		fingerprint.versionDAT != record.VersionDATSHA256 ||
		fingerprint.assetMap != record.AssetMapSHA256 ||
		len(fingerprint.patchRoots) != len(record.PatchRoots) {
		return false
	}
	for index := range fingerprint.patchRoots {
		if fingerprint.patchRoots[index] != record.PatchRoots[index] {
			return false
		}
	}
	return true
}

func catalogDigestHex(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func hashClientResourceInputs(imageRoot, assetMapPath string, patchRoots []string) (clientResourceFingerprint, error) {
	var fingerprint clientResourceFingerprint
	var err error
	fingerprint.imageManifest, err = hashFile(filepath.Join(imageRoot, "manifest.json"))
	if err != nil {
		return fingerprint, err
	}
	fingerprint.versionDAT, err = hashFile(filepath.Join(patchRoots[0], "version.dat"))
	if err != nil {
		return fingerprint, err
	}
	fingerprint.assetMap, err = hashFile(assetMapPath)
	if err != nil {
		return fingerprint, err
	}
	fingerprint.patchRoots = patchRoots
	return fingerprint, nil
}

func cachedImagesMatch(imageRoot string, images []clientResourceCacheImage) bool {
	if len(images) == 0 {
		return false
	}
	for _, image := range images {
		source, err := cdnSourcePath(imageRoot, image.Path)
		if err != nil {
			return false
		}
		info, err := os.Stat(source)
		if err != nil || !info.Mode().IsRegular() || info.Size() != image.Size || info.ModTime().UnixNano() != image.ModUnixNano {
			return false
		}
	}
	return true
}

func cachedBundlesMatch(patchRoots []string, bundles []clientResourceCacheBundle) bool {
	if len(bundles) == 0 {
		return false
	}
	for _, bundle := range bundles {
		info, root, err := locateCNPatchFile(patchRoots, bundle.Name)
		if !bundle.Present {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false
		}
		if err != nil || root != bundle.Root || !info.Mode().IsRegular() || info.Size() != bundle.Size || info.ModTime().UnixNano() != bundle.ModUnixNano {
			return false
		}
	}
	return true
}

func cardImagesFromCache(imageRoot, namespace string, images []clientResourceCacheImage) (cardImages, bool) {
	result := cardImages{Namespace: namespace, Files: make(map[string]cardImage, len(images))}
	if namespace == "" {
		return result, false
	}
	for _, image := range images {
		digest, err := hex.DecodeString(image.SHA256)
		if !cardImagePath.MatchString(image.Path) || err != nil || len(digest) != sha256.Size || image.Bytes <= 0 {
			return result, false
		}
		source, err := cdnSourcePath(imageRoot, image.Path)
		if err != nil {
			return result, false
		}
		if _, exists := result.Files[image.Path]; exists {
			return result, false
		}
		result.Files[image.Path] = cardImage{
			Path: image.Path, Bytes: image.Bytes, SHA256: image.SHA256, SourcePath: source,
		}
	}
	return result, len(result.Files) > 0
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func replaceCacheFile(path string, data []byte) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
