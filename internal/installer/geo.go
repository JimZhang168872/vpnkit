package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"vpnkit/internal/netx"
)

// GeoFile names one of mihomo's startup data files plus where to fetch it.
// The Name must match what mihomo looks for in its config dir — see
// config.mihomoGeoxURL for the canonical mapping.
type GeoFile struct {
	Name string
	URL  string
}

// DefaultGeoFiles is the set of GeoIP / GeoSite assets mihomo expects in its
// config dir at startup. Kept in sync with config.mihomoGeoxURL — when one
// changes the other must follow.
var DefaultGeoFiles = []GeoFile{
	{Name: "country.mmdb", URL: "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb"},
	{Name: "geoip.metadb", URL: "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb"},
	{Name: "geosite.dat", URL: "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat"},
	{Name: "GeoLite2-ASN.mmdb", URL: "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/GeoLite2-ASN.mmdb"},
}

// EnsureGeo populates dir with every entry in files, downloading via
// netx.SmartClient (which honors a live HTTPS_PROXY when reachable). Files
// already present and non-empty are left alone. Passing files=nil uses
// DefaultGeoFiles. Returns the names actually downloaded plus an aggregated
// error from any download that failed; individual failures do not abort the
// rest, so a partial result is normal.
//
// Why this exists: empirically mihomo's built-in GeoIP download (a) hardcodes
// a 90s deadline and (b) does not respect HTTP(S)_PROXY env, so first-launch
// on China-network hosts deadlocks the service. Pre-seeding the files lets
// mihomo skip the download entirely.
func EnsureGeo(dir string, files []GeoFile) ([]string, error) {
	if files == nil {
		files = DefaultGeoFiles
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	client := netx.SmartClient(0)
	var (
		fetched []string
		errs    []error
	)
	for _, f := range files {
		dst := filepath.Join(dir, f.Name)
		// info.Size() > 0 is a sufficient existence check because
		// downloadGeoFile only Renames the temp into place after Sync+Close,
		// so any file present at dst is a complete, prior-successful download.
		// Zero-byte placeholders (e.g. from a manual `touch`) trigger
		// re-download — see TestEnsureGeoSizeZeroTriggersRedownload.
		if info, err := os.Stat(dst); err == nil && info.Size() > 0 {
			continue
		}
		if err := downloadGeoFile(client, f.URL, dst); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.Name, err))
			continue
		}
		fetched = append(fetched, f.Name)
	}
	if len(errs) > 0 {
		return fetched, errors.Join(errs...)
	}
	return fetched, nil
}

// downloadGeoFile fetches a single asset and writes it atomically to dst.
// The 3-minute deadline matches the worst-case proxy-mediated transfer of
// the largest current asset (geoip.metadb ~9 MB) over a slow link.
func downloadGeoFile(client *http.Client, url, dst string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// On any error path before Rename, remove the temp. After Rename, the
	// source no longer exists and Remove is a no-op.
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}
