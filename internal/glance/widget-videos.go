package glance

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const videosWidgetPlaylistPrefix = "playlist:"

var (
	videosWidgetTemplate             = mustParseTemplate("videos.html", "widget-base.html", "video-card-contents.html")
	videosWidgetGridTemplate         = mustParseTemplate("videos-grid.html", "widget-base.html", "video-card-contents.html")
	videosWidgetVerticalListTemplate = mustParseTemplate("videos-vertical-list.html", "widget-base.html")
)

type videosWidget struct {
	widgetBase                   `yaml:",inline"`
	Videos                       videoList `yaml:"-"`
	cachedVideoLists             sync.Map  `yaml:"-"`
	VideoUrlTemplate             string    `yaml:"video-url-template"`
	Style                        string    `yaml:"style"`
	CollapseAfter                int       `yaml:"collapse-after"`
	CollapseAfterRows            int       `yaml:"collapse-after-rows"`
	Channels                     []string  `yaml:"channels"`
	Playlists                    []string  `yaml:"playlists"`
	Limit                        int       `yaml:"limit"`
	IncludeShorts                bool      `yaml:"include-shorts"`
	UseDearrowTitles             bool      `yaml:"use-dearrow-titles"`
	UseDearrowThumbnails         bool      `yaml:"use-dearrow-thumbnails"`
	DearrowTitlesInstanceUrl     string    `yaml:"dearrow-titles-instance-url"`
	DearrowThumbnailsInstanceUrl string    `yaml:"dearrow-thumbnails-instance-url"`
	SortBy                       string    `yaml:"sort-by"`
}

func (w *videosWidget) initialize() error {
	w.withTitle("Videos").withCacheDuration(time.Hour)

	if w.Limit <= 0 {
		w.Limit = 25
	}

	if w.CollapseAfterRows == 0 || w.CollapseAfterRows < -1 {
		w.CollapseAfterRows = 4
	}

	if w.CollapseAfter == 0 || w.CollapseAfter < -1 {
		w.CollapseAfter = 7
	}

	// A bit cheeky, but from a user's perspective it makes more sense when channels and
	// playlists are separate things rather than specifying a list of channels and some of
	// them awkwardly have a "playlist:" prefix
	if len(w.Playlists) > 0 {
		initialLen := len(w.Channels)
		w.Channels = append(w.Channels, make([]string, len(w.Playlists))...)

		for i := range w.Playlists {
			w.Channels[initialLen+i] = videosWidgetPlaylistPrefix + w.Playlists[i]
		}
	}

	return nil
}

func (w *videosWidget) update(ctx context.Context) {
	videos, err := w.fetchYoutubeChannelUploads(w.Channels, w.VideoUrlTemplate, w.IncludeShorts, w.UseDearrowTitles, w.UseDearrowThumbnails, w.DearrowTitlesInstanceUrl, w.DearrowThumbnailsInstanceUrl, w.SortBy)

	if !w.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(videos) > w.Limit {
		videos = videos[:w.Limit]
	}

	w.Videos = videos
}

func (w *videosWidget) Render() template.HTML {
	var template *template.Template

	switch w.Style {
	case "grid-cards":
		template = videosWidgetGridTemplate
	case "vertical-list":
		template = videosWidgetVerticalListTemplate
	default:
		template = videosWidgetTemplate
	}

	return w.renderTemplate(w, template)
}

type youtubeFeedResponseXml struct {
	Channel     string `xml:"author>name"`
	ChannelLink string `xml:"author>uri"`
	Videos      []struct {
		Title     string `xml:"title"`
		Published string `xml:"published"`
		Link      struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`

		Group struct {
			Thumbnail struct {
				Url string `xml:"url,attr"`
			} `xml:"http://search.yahoo.com/mrss/ thumbnail"`
		} `xml:"http://search.yahoo.com/mrss/ group"`
	} `xml:"entry"`
}

type dearrowBrandingResponseJson struct {
	Titles []struct {
		Title    string `json:"title"`
		Original bool   `json:"original"`
		Votes    int    `json:"votes"`
		Locked   bool   `json:"locked"`
		UUID     string `json:"UUID"`
		UserID   string `json:"userID,omitempty"`
	} `json:"titles"`
	Thumbnails []struct {
		Timestamp float32 `json:"timestamp"`
		Original  bool    `json:"original"`
		Votes     int     `json:"votes"`
		Locked    bool    `json:"locked"`
		UUID      string  `json:"UUID"`
		UserID    string  `json:"userID,omitempty"`
	} `json:"thumbnails"`
	RandomTime    float32  `json:"randomTime"`
	VideoDuration *float32 `json:"videoDuration"`
}

// https://wiki.sponsor.ajay.app/w/API_Docs/DeArrow
func dearrowGetFirstMatchingTitle(response dearrowBrandingResponseJson) (string, bool) {
	if len(response.Titles) > 0 {
		for _, title := range response.Titles {
			if title.Locked || title.Votes >= 0 {
				return title.Title, true
			}
		}
	}

	for _, thumbnail := range response.Thumbnails {
		if thumbnail.Locked || thumbnail.Votes >= 0 {
			return "", true
		}
	}

	return "", false
}

func dearrowGetThumbnailTask(client requestDoer) func(*http.Request) (string, error) {
	return func(request *http.Request) (string, error) {
		response, err := client.Do(request)
		if err != nil {
			return "", err
		}
		defer response.Body.Close()

		if response.StatusCode == http.StatusOK { // status code 200
			return request.URL.String(), nil
		}

		if response.StatusCode == http.StatusNoContent { // status code 204
			failReason := response.Header.Get("X-Failure-Reason")
			slog.Error("Failed to get the DeArrow thumbnail. Reason:", failReason, nil)
			return "", fmt.Errorf("no content, X-Failure-Reason: %s", failReason)
		}

		return "", nil
	}
}

func parseYoutubeFeedTime(t string) time.Time {
	parsedTime, err := time.Parse("2006-01-02T15:04:05-07:00", t)
	if err != nil {
		return time.Now()
	}

	return parsedTime
}

type video struct {
	ThumbnailUrl string
	Title        string
	Url          string
	Author       string
	AuthorUrl    string
	TimePosted   time.Time
}

type videoList []video

func (v videoList) sortByNewest() videoList {
	sort.Slice(v, func(i, j int) bool {
		return v[i].TimePosted.After(v[j].TimePosted)
	})

	return v
}

func (w *videosWidget) fetchYoutubeChannelUploads(channelOrPlaylistIDs []string, videoUrlTemplate string, includeShorts bool, useDearrowTitles bool, useDearrowThumbnails bool, dearrowTitlesInstanceUrl string, dearrowThumbnailsInstanceUrl string, sortBy string) (videoList, error) {
	task := func(id string) (videoList, error) {
		if cached, ok := w.cachedVideoLists.Load(id); ok {
			entry := cached.(cachedEntry[videoList])
			if time.Since(entry.timestamp) < w.cacheDuration {
				return entry.value, nil
			}
		}

		list := make(videoList, 0, 15)

		var feedURL string
		if after, ok := strings.CutPrefix(id, videosWidgetPlaylistPrefix); ok {
			feedURL = "https://www.youtube.com/feeds/videos.xml?playlist_id=" + after
		} else if !includeShorts && strings.HasPrefix(id, "UC") {
			playlistId := strings.Replace(id, "UC", "UULF", 1)
			feedURL = "https://www.youtube.com/feeds/videos.xml?playlist_id=" + playlistId
		} else {
			feedURL = "https://www.youtube.com/feeds/videos.xml?channel_id=" + id
		}

		request, _ := http.NewRequest("GET", feedURL, nil)
		response, err := decodeXmlFromRequest[youtubeFeedResponseXml](defaultHTTPClient, request)
		if err != nil {
			cached, ok := w.cachedVideoLists.Load(id)
			if ok {
				return cached.(cachedEntry[videoList]).value, err
			}

			return list, err
		}

		for j := range response.Videos {
			v := &response.Videos[j]

			var videoUrl string
			if videoUrlTemplate == "" {
				videoUrl = v.Link.Href
			} else {
				parsedUrl, err := url.Parse(v.Link.Href)

				if err == nil {
					videoUrl = strings.ReplaceAll(videoUrlTemplate, "{VIDEO-ID}", parsedUrl.Query().Get("v"))
				} else {
					videoUrl = "#"
				}
			}

			var authorUrl string
			if videoUrlTemplate == "" {
				authorUrl = response.ChannelLink + "/videos"
			} else {
				baseUrl := strings.Split(videoUrlTemplate, "/watch")[0]
				authorUrl = baseUrl + strings.TrimPrefix(response.ChannelLink, "https://www.youtube.com")
			}

			list = append(list, video{
				ThumbnailUrl: v.Group.Thumbnail.Url,
				Title:        v.Title,
				Url:          videoUrl,
				Author:       response.Channel,
				AuthorUrl:    authorUrl,
				TimePosted:   parseYoutubeFeedTime(v.Published),
			})
		}

		w.cachedVideoLists.Store(id, cachedEntry[videoList]{value: list, timestamp: time.Now()})
		return list, nil
	}

	job := newJob(task, channelOrPlaylistIDs).withWorkers(30)
	lists, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	videos := make(videoList, 0, len(channelOrPlaylistIDs)*15)
	var failed int

	for i := range lists {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch youtube feed", "channel", channelOrPlaylistIDs[i], "error", errs[i])
		}

		// We still append the list even if it failed, because we may have a cached version of the list
		videos = append(videos, lists[i]...)
	}

	if len(videos) == 0 {
		return nil, errNoContent
	}

	if useDearrowTitles || useDearrowThumbnails {
		if dearrowTitlesInstanceUrl == "" {
			dearrowTitlesInstanceUrl = "https://sponsor.ajay.app"
		}

		if dearrowThumbnailsInstanceUrl == "" {
			dearrowThumbnailsInstanceUrl = "https://dearrow-thumb.ajay.app"
		}

		var dearrowTitleRequests []*http.Request
		var dearrowTitleIndices []int

		var dearrowThumbnailRequests []*http.Request
		var dearrowThumbnailIndices []int

		for i, vid := range videos {
			if vid.Url == "#" {
				continue
			}
			parsedUrl, err := url.Parse(vid.Url)
			if err != nil {
				slog.Error("Failed to parse video URL for dearrow", "url", vid.Url, "error", err)
				continue
			}
			videoID := parsedUrl.Query().Get("v")
			if videoID == "" {
				continue
			}

			if useDearrowTitles {
				req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/branding?videoID=%s", dearrowTitlesInstanceUrl, videoID), nil)
				if err != nil {
					slog.Error("Failed to create dearrow branding request", "videoID", videoID, "error", err)
					continue
				}
				dearrowTitleRequests = append(dearrowTitleRequests, req)
				dearrowTitleIndices = append(dearrowTitleIndices, i)
			}

			if useDearrowThumbnails {
				req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/getThumbnail?videoID=%s", dearrowThumbnailsInstanceUrl, videoID), nil)
				if err != nil {
					slog.Error("Failed to create dearrow branding request", "videoID", videoID, "error", err)
					continue
				}
				dearrowThumbnailRequests = append(dearrowThumbnailRequests, req)
				dearrowThumbnailIndices = append(dearrowThumbnailIndices, i)
			}
		}

		if useDearrowTitles {
			jobDearrowTitles := newJob(decodeJsonFromRequestTask[dearrowBrandingResponseJson](defaultHTTPClient), dearrowTitleRequests).withWorkers(30)
			dearrowTitleResponses, dearrowTitleErrs, err := workerPoolDo(jobDearrowTitles)
			if err != nil {
				slog.Error("Failed to complete dearrow branding job pool", "error", err)
			}

			for j, videoIndex := range dearrowTitleIndices {
				if dearrowTitleErrs[j] != nil {
					continue
				}

				brandingResponse := dearrowTitleResponses[j]
				if newTitle, ok := dearrowGetFirstMatchingTitle(brandingResponse); ok && newTitle != "" {
					videos[videoIndex].Title = newTitle
				}
			}
		}

		if useDearrowThumbnails {
			jobDearrowThumbnails := newJob(dearrowGetThumbnailTask(defaultHTTPClient), dearrowThumbnailRequests).withWorkers(30)
			dearrowThumbnailResponses, dearrowThumbnailErrs, err := workerPoolDo(jobDearrowThumbnails)
			if err != nil {
				slog.Error("Failed to complete dearrow getThumbnail job pool", "error", err)
			}

			for j, videoIndex := range dearrowThumbnailIndices {
				if dearrowThumbnailErrs[j] != nil {
					continue
				}

				thumbnailResponse := dearrowThumbnailResponses[j]
				if thumbnailResponse != "" {
					videos[videoIndex].ThumbnailUrl = thumbnailResponse
				}
			}
		}
	}

	videos.sortByNewest()

	if failed > 0 {
		return videos, fmt.Errorf("%w: missing videos from %d channels", errPartialContent, failed)
	}

	return videos, nil
}
