package relay

import (
	"net/url"
	"strings"
)

func mediaReferer(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	for _, host := range []string{"hdslb.com", "bilivideo.com", "bilivideo.cn", "upos-hz-mirrorakam.akamaized.net"} {
		if u.Hostname() == host || strings.HasSuffix(u.Hostname(), "."+host) {
			return "https://www.bilibili.com/"
		}
	}
	return "https://www.douyin.com/"
}
