package internetarchive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tilalis/capnhook/media/interfaces"
)

// licenseFilter keeps only items that are permissible to view: Creative
// Commons licensed or dedicated to the public domain.
const licenseFilter = "(licenseurl:(*creativecommons.org*) OR licenseurl:(*publicdomain*))"

// defaultRows caps the number of search results when no limit is given.
const defaultRows = 30

// searchFields are the item fields requested from the advanced search API.
var searchFields = []string{
	"identifier",
	"title",
	"description",
	"creator",
	"item_size",
	"files_count",
	"publicdate",
}

type InternetArchive struct {
	apiUrl  string
	siteUrl string
}

func New(apiUrl string, siteUrl string) *InternetArchive {
	if apiUrl == "" {
		apiUrl = "https://archive.org"
	}
	if siteUrl == "" {
		siteUrl = "https://archive.org"
	}

	return &InternetArchive{
		apiUrl:  apiUrl,
		siteUrl: siteUrl,
	}
}

func NewDefault() *InternetArchive {
	return New("", "")
}

func (ia *InternetArchive) Find(ctx context.Context, id string) (interfaces.TorrentSearchResult, error) {
	body, err := ia.request(ctx, "metadata/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}

	var item metadataResponse
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, fmt.Errorf("unmarshal metadata response: %w", err)
	}

	// the metadata API answers requests for unknown identifiers with "{}"
	if item.Metadata.Identifier == "" {
		return nil, fmt.Errorf("torrent not found by ID: %s", id)
	}

	if !strings.EqualFold(string(item.Metadata.Mediatype), "movies") {
		return nil, fmt.Errorf("item %s is not a movie", id)
	}

	if !isPermissibleLicense(string(item.Metadata.LicenseUrl)) {
		return nil, fmt.Errorf("item %s is not under a permissible license", id)
	}

	return Torrent{
		identifier:  item.Metadata.Identifier,
		title:       string(item.Metadata.Title),
		description: string(item.Metadata.Description),
		creator:     string(item.Metadata.Creator),
		sizeBytes:   item.ItemSize,
		numFiles:    item.FilesCount,
		added:       parseArchiveTime(string(item.Metadata.PublicDate)),
		torrentUrl:  ia.torrentUrl(item.Metadata.Identifier),
	}, nil
}

func (ia *InternetArchive) Search(ctx context.Context, query string, limit int) ([]interfaces.TorrentSearchResult, error) {
	luceneQuery, err := buildSearchQuery(query)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = defaultRows
	}

	params := url.Values{}
	params.Set("q", luceneQuery)
	for _, field := range searchFields {
		params.Add("fl[]", field)
	}
	params.Set("sort[]", "downloads desc")
	params.Set("rows", strconv.Itoa(limit))
	params.Set("page", "1")
	params.Set("output", "json")

	body, err := ia.request(ctx, "advancedsearch.php?"+params.Encode())
	if err != nil {
		return nil, err
	}

	var response searchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unmarshal search response: %w", err)
	}

	docs := response.Response.Docs
	if len(docs) == 0 {
		return nil, fmt.Errorf("no torrents found by query: %s", query)
	}

	results := make([]interfaces.TorrentSearchResult, len(docs))
	for i, doc := range docs {
		results[i] = Torrent{
			identifier:  doc.Identifier,
			title:       string(doc.Title),
			description: string(doc.Description),
			creator:     string(doc.Creator),
			sizeBytes:   doc.ItemSize,
			numFiles:    doc.FilesCount,
			added:       parseArchiveTime(string(doc.PublicDate)),
			torrentUrl:  ia.torrentUrl(doc.Identifier),
		}
	}

	return results, nil
}

func (ia *InternetArchive) SiteUrl(t interfaces.TorrentSearchResult) string {
	return fmt.Sprintf("%s/details/%s", ia.siteUrl, t.ID())
}

// torrentUrl returns the URL of the item's archive torrent file, which every
// Internet Archive item exposes for download.
func (ia *InternetArchive) torrentUrl(identifier string) string {
	return fmt.Sprintf("%s/download/%s/%s_archive.torrent", ia.siteUrl, identifier, identifier)
}

// luceneSpecials are stripped from user input so it cannot alter the query
// structure or escape the movies-only and license filters.
const luceneSpecials = "+-&|!(){}[]^\"~*?:\\/"

// buildSearchQuery turns a raw user query into a Lucene query that matches
// the title of movie items under a permissible license.
func buildSearchQuery(query string) (string, error) {
	sanitized := strings.Map(func(r rune) rune {
		if strings.ContainsRune(luceneSpecials, r) {
			return ' '
		}
		return r
	}, query)

	terms := strings.Fields(sanitized)
	if len(terms) == 0 {
		return "", fmt.Errorf("empty search query: %q", query)
	}

	return fmt.Sprintf(
		"title:(%s) AND mediatype:(movies) AND %s",
		strings.Join(terms, " "),
		licenseFilter,
	), nil
}

// isPermissibleLicense reports whether licenseURL denotes a license that
// permits viewing: any Creative Commons license or a public domain mark.
func isPermissibleLicense(licenseURL string) bool {
	license := strings.ToLower(licenseURL)
	return strings.Contains(license, "creativecommons.org") || strings.Contains(license, "publicdomain")
}

// parseArchiveTime handles both timestamp formats the Internet Archive uses:
// RFC3339 in search responses and "2006-01-02 15:04:05" in metadata responses.
func parseArchiveTime(raw string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}

	return time.Time{}
}

func (ia *InternetArchive) request(ctx context.Context, endpoint string) ([]byte, error) {
	return get(ctx, fmt.Sprintf("%s/%s", ia.apiUrl, endpoint))
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", url, err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:152.0) Gecko/20100101 Firefox/152.0")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error fetching %s: %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("read response from %s: %w", url, err)
	}

	return body, nil
}
