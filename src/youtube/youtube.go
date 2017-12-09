package youtube

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	spttb_system "system"
	spttb_track "track"

	"github.com/PuerkitoBio/goquery"
)

type YouTubeTracks struct {
	Track             *spttb_track.Track
	Selection         *goquery.Selection
	SelectionDesc     *goquery.Selection
	SelectionDuration *goquery.Selection
	SelectionPointer  int
}

type YouTubeTrack struct {
	Track    *spttb_track.Track
	ID       string
	URL      string
	Title    string
	User     string
	Duration int
}

func QueryTracks(track *spttb_track.Track) (*YouTubeTracks, error) {
	var (
		doc          *goquery.Document
		query_string = fmt.Sprintf(spttb_system.YOUTUBE_QUERY_PATTERN,
			strings.Replace(track.SearchPattern, " ", "+", -1))
	)
	request, _ := http.NewRequest("GET", query_string, nil)
	request.Header.Add("Accept-Language", "en")
	response, err := http.DefaultClient.Do(request)
	if err == nil {
		doc, _ = goquery.NewDocumentFromResponse(response)
	} else {
		doc, err = goquery.NewDocument(query_string)
	}
	if err != nil {
		return &YouTubeTracks{}, errors.New(fmt.Sprintf("Cannot retrieve doc from \"%s\": %s", query_string, err.Error()))
	}
	// html, _ := doc.Html()
	// logger.Debug(html)
	return &YouTubeTracks{
		Track:             track,
		Selection:         doc.Find(spttb_system.YOUTUBE_VIDEO_SELECTOR),
		SelectionDesc:     doc.Find(spttb_system.YOUTUBE_DESC_SELECTOR),
		SelectionDuration: doc.Find(spttb_system.YOUTUBE_DURATION_SELECTOR),
		SelectionPointer:  0,
	}, nil
}

func (youtube_tracks *YouTubeTracks) HasNext() bool {
	return youtube_tracks.SelectionPointer+1 < len(youtube_tracks.Selection.Nodes)
}

func (youtube_tracks *YouTubeTracks) Next() (*YouTubeTrack, error) {
	var err error
	if youtube_tracks.HasNext() {
		youtube_tracks.SelectionPointer++
		item := youtube_tracks.Selection.Eq(youtube_tracks.SelectionPointer)
		item_href, item_href_ok := item.Attr("href")
		item_title, item_title_ok := item.Attr("title")
		item_user, item_user_ok := "UNKNOWN", false
		item_length, item_length_ok := 0, false
		if youtube_tracks.SelectionPointer < len(youtube_tracks.SelectionDesc.Nodes) {
			item_desc := youtube_tracks.SelectionDesc.Eq(youtube_tracks.SelectionPointer)
			item_user = strings.TrimSpace(item_desc.Find("a").Text())
			item_user_ok = true
		}
		if youtube_tracks.SelectionPointer < len(youtube_tracks.SelectionDuration.Nodes) {
			var item_length_m, item_length_s int
			item_duration := youtube_tracks.SelectionDuration.Eq(youtube_tracks.SelectionPointer)
			item_length_str := strings.TrimSpace(item_duration.Text())
			if strings.Contains(item_length_str, ": ") {
				item_length_str = strings.Split(item_length_str, ": ")[1]
				item_length_m, err = strconv.Atoi(strings.Split(item_length_str, ":")[0])
				if err == nil {
					item_length_s, err = strconv.Atoi(strings.Split(item_length_str, ":")[1][:2])
					if err == nil {
						item_length = item_length_m*60 + item_length_s
						item_length_ok = true
					}
				}
			}
		}
		if !(item_href_ok && item_title_ok && item_user_ok && item_length_ok) {
			return &YouTubeTrack{}, errors.New(fmt.Sprintf("Non-standard YouTube video entry structure: "+
				"url is %s, title is %s, user is %s, duration is %s.",
				strconv.FormatBool(item_href_ok), strconv.FormatBool(item_title_ok),
				strconv.FormatBool(item_user_ok), strconv.FormatBool(item_length_ok)))
		} else if !strings.Contains(strings.ToLower(item_href), "youtu.be") &&
			strings.Contains(strings.ToLower(item_href), "&list=") {
			return &YouTubeTrack{}, errors.New(fmt.Sprintf("Playlist URL found: %s", item_href))
		} else if !strings.Contains(strings.ToLower(item_href), "youtu.be") &&
			!strings.Contains(strings.ToLower(item_href), "watch?v=") {
			return &YouTubeTrack{}, errors.New(fmt.Sprintf("Advertising URL found: %s", item_href))
		}

		return &YouTubeTrack{
			Track:    youtube_tracks.Track,
			ID:       IdFromUrl(spttb_system.YOUTUBE_VIDEO_PREFIX + item_href),
			URL:      spttb_system.YOUTUBE_VIDEO_PREFIX + item_href,
			Title:    item_title,
			User:     item_user,
			Duration: item_length,
		}, nil
	}

	return &YouTubeTrack{}, errors.New("No more results left on page.")
}

func (youtube_track YouTubeTrack) Match(track spttb_track.Track) error {
	if int(math.Abs(float64(track.Duration-youtube_track.Duration))) > spttb_system.YOUTUBE_DURATION_TOLERANCE {
		return errors.New(fmt.Sprintf("The duration difference is excessive: | %d - %d | = %d (max tolerated: %d)",
			track.Duration, youtube_track.Duration, int(math.Abs(float64(track.Duration-youtube_track.Duration))), spttb_system.YOUTUBE_DURATION_TOLERANCE))
	}
	if strings.Contains(youtube_track.URL, "&list=") || strings.Contains(youtube_track.URL, "/user/") {
		return errors.New("Track is actually pointing to playlist or user.")
	}
	if err := track.Seems(youtube_track.Title); err != nil {
		return err
	}
	return nil
}

func (track YouTubeTrack) Download() error {
	var command_out bytes.Buffer
	command_cmd := "youtube-dl"
	command_args := []string{"--output", track.Track.FilenameTemp + ".%(ext)s", "--format", "bestaudio", "--extract-audio", "--audio-format", track.Track.FilenameExt[1:], "--audio-quality", "0", track.URL}
	command_obj := exec.Command(command_cmd, command_args...)
	command_obj.Stderr = &command_out
	if command_err := command_obj.Run(); command_err != nil {
		return errors.New(fmt.Sprintf("Something went wrong while executing \"%s %s\":\n%s", command_cmd, strings.Join(command_args, " "), command_out.String()))
	}
	return nil
}

func IdFromUrl(url string) string {
	var id_part string
	if strings.Contains(strings.ToLower(url), "youtu.be/") {
		id_part = strings.Split(url, "youtu.be/")[1]
	} else {
		id_part = strings.Split(url, "watch?v=")[1]
	}
	if strings.Contains(id_part, "?") {
		id_part = strings.Split(id_part, "?")[0]
	}
	if strings.Contains(id_part, "&list") {
		id_part = strings.Split(id_part, "&list")[0]
	}
	return id_part
}

func ValidateURL(url string) error {
	if !strings.Contains(strings.ToLower(url), "youtu.be/") &&
		!strings.Contains(strings.ToLower(url), "watch?v=") {
		return errors.New(fmt.Sprintf("URL %s doesn't seem to be pointing to any YouTube video.", url))
	}
	return nil
}
