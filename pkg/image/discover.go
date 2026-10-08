package image

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Discover finds the newest build of an image whose upstream only publishes
// versioned filenames (no stable "latest" URL). The index page is fetched and
// searched with Match; the highest version wins.
type Discover struct {
	// Index is a directory listing or JSON document that names the available builds.
	Index string `yaml:"index"`
	// Match is a regex whose first capture group is the version, compared numerically.
	Match string `yaml:"match"`
	// URL is the download URL with {version} substituted. When empty, the full
	// regex match is resolved relative to Index.
	URL string `yaml:"url,omitempty"`
}

// Resolve returns the download URL of the newest build listed in the index.
func (d Discover) Resolve() (string, error) {
	re, err := regexp.Compile(d.Match)
	if err != nil {
		return "", fmt.Errorf("invalid discover.match: %w", err)
	}
	if re.NumSubexp() < 1 {
		return "", fmt.Errorf("discover.match needs a capture group for the version")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", d.Index, nil)
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("GET %s failed: %s", d.Index, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return "", err
	}

	return d.pick(re, string(body))
}

func (d Discover) pick(re *regexp.Regexp, body string) (string, error) {
	var bestMatch, bestVersion string
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		if bestVersion == "" || compareVersions(m[1], bestVersion) > 0 {
			bestMatch, bestVersion = m[0], m[1]
		}
	}
	if bestVersion == "" {
		return "", fmt.Errorf("no entry in %s matches %q", d.Index, d.Match)
	}

	if d.URL != "" {
		return strings.ReplaceAll(d.URL, "{version}", bestVersion), nil
	}
	base, err := url.Parse(d.Index)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(bestMatch)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

// compareVersions compares dotted numeric versions ("3.23.10" > "3.23.9").
// Non-numeric separators are treated alike; missing parts count as zero.
func compareVersions(a, b string) int {
	split := func(s string) []string {
		return strings.FieldsFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	}
	pa, pb := split(a), split(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}
