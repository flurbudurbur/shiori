package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/flurbudurbur/Shiori/pkg/errors"

	goversion "github.com/hashicorp/go-version"
)

type Release struct {
	ID              int64     `json:"id,omitempty"`
	NodeID          string    `json:"node_id,omitempty"`
	URL             string    `json:"url,omitempty"`
	HtmlURL         string    `json:"html_url,omitempty"`
	TagName         string    `json:"tag_name,omitempty"`
	TargetCommitish string    `json:"target_commitish,omitempty"`
	Name            *string   `json:"name,omitempty"`
	Body            *string   `json:"body,omitempty"`
	Draft           bool      `json:"draft,omitempty"`
	Prerelease      bool      `json:"prerelease,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	PublishedAt     time.Time `json:"published_at"`
	Author          Author    `json:"author"`
	Assets          []Asset   `json:"assets"`
}

type Author struct {
	Login      string `json:"login"`
	Id         int64  `json:"id"`
	NodeId     string `json:"node_id"`
	AvatarUrl  string `json:"avatar_url"`
	GravatarId string `json:"gravatar_id"`
	Url        string `json:"url"`
	HtmlUrl    string `json:"html_url"`
	Type       string `json:"type"`
}
type Asset struct {
	Url                string    `json:"url"`
	Id                 int64     `json:"id"`
	NodeId             string    `json:"node_id"`
	Name               string    `json:"name"`
	Label              string    `json:"label"`
	Uploader           Author    `json:"uploader"`
	ContentType        string    `json:"content_type"`
	State              string    `json:"state"`
	Size               int64     `json:"size"`
	DownloadCount      int64     `json:"download_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	BrowserDownloadUrl string    `json:"browser_download_url"`
}

func (r *Release) IsPreOrDraft() bool {
	if r.Draft || r.Prerelease {
		return true
	}
	return false
}

type Checker struct {
	// user/repo-name or org/repo-name
	Owner          string
	Repo           string
	CurrentVersion string
}

func NewChecker(owner, repo, currentVersion string) *Checker {
	return &Checker{
		Owner:          owner,
		Repo:           repo,
		CurrentVersion: currentVersion,
	}
}

func (c *Checker) get(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", c.Owner, c.Repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", c.buildUserAgent())

	client := http.DefaultClient

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting releases for %v: %s", c.Repo, resp.Status)
	}

	var release Release
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func (c *Checker) CheckNewVersion(ctx context.Context, version string) (bool, *Release, error) {
	if isDevelop(version) {
		return false, nil, nil
	}

	release, err := c.get(ctx)
	if err != nil {
		return false, nil, err
	}

	newAvailable, _, err := c.checkNewVersion(version, release)
	if err != nil {
		return false, nil, err
	}

	if !newAvailable {
		return false, nil, nil
	}

	return true, release, nil
}

func (c *Checker) checkNewVersion(version string, release *Release) (bool, string, error) {
	currentVersion, err := goversion.NewVersion(version)
	if err != nil {
		return false, "", errors.Wrap(err, "error parsing current version")
	}

	releaseVersion, err := goversion.NewVersion(release.TagName)
	if err != nil {
		return false, "", errors.Wrap(err, "error parsing release version")
	}

	if len(currentVersion.Prerelease()) == 0 && len(releaseVersion.Prerelease()) > 0 {
		return false, "", nil
	}

	if releaseVersion.GreaterThan(currentVersion) {
		// new update available
		return true, releaseVersion.String(), nil
	}

	return false, "", nil
}

func (c *Checker) buildUserAgent() string {
	return fmt.Sprintf("SyncYomi/%s (%s %s)", c.CurrentVersion, runtime.GOOS, runtime.GOARCH)
}

func isDevelop(version string) bool {
	tags := []string{"dev", "develop", "master", "latest", ""}

	for _, tag := range tags {
		if version == tag {
			return true
		}
	}

	return false
}
