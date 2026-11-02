package app

import (
	"net/url"
	"strings"
)

func releaseAssetName(version, platform string) string {
	parts := strings.Split(platform, "-")
	if len(parts) != 2 || (parts[1] != "arm64" && parts[1] != "amd64") {
		return ""
	}
	switch parts[0] {
	case "darwin":
		return "hoard-" + version + "-" + parts[1] + ".dmg"
	case "windows":
		return "hoard-setup-" + version + "-" + parts[1] + ".exe"
	case "linux":
		return "hoard-" + version + "-linux-" + parts[1] + ".tar.gz"
	}
	return ""
}

func allowedUpdateLink(raw string) bool {
	proxy := strings.HasPrefix(raw, updateProxy)
	if proxy {
		raw = strings.TrimPrefix(raw, updateProxy)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return false
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	if proxy {
		path, ok := strings.CutPrefix(raw, RepoURL+"/releases/download/")
		if !ok {
			return false
		}
		tag, name, ok := strings.Cut(path, "/")
		if !ok || parseVersion(tag) == nil {
			return false
		}
		for _, platform := range []string{"darwin-arm64", "darwin-amd64", "windows-amd64", "windows-arm64", "linux-amd64", "linux-arm64"} {
			if name == releaseAssetName(strings.TrimPrefix(tag, "v"), platform) {
				return true
			}
		}
		return false
	}
	return raw == RepoURL || strings.HasPrefix(raw, RepoURL+"/")
}
