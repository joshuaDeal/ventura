package main
 
import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
 
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	tag "github.com/dhowden/tag"
	mpv "github.com/gen2brain/go-mpv"
	"github.com/qeesung/image2ascii/convert"
)
 
// Print a useful message hopefully.
func printHelp() {
	fmt.Println("A cli audio player.")
	fmt.Println("Usage:")
	flag.PrintDefaults()
}
 
// Custom type used to store audio file meta data.
type TrackMetadata struct {
	Title       string
	Album       string
	Artist      string
	AlbumArtist string
	Composer    string
	Genre       string
	Year        int
	//Track (int, int)
	//Disc (int, int)
	Artwork *tag.Picture
	Lyrics  string
	Comment string
}
 
// Extract metadata from an audio track.
func getTrackMetaData(path string) *TrackMetadata {
	meta := &TrackMetadata{Title: path}
 
	// Open the file
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
 
	// Read track metadata
	m, err := tag.ReadFrom(f)
	if err != nil {
		// File has no metadata error.
		if errors.Is(err, tag.ErrNoTagsFound) {
			return meta
		}
 
		// All other errors.
		log.Fatal(err)
	}
 
	// If there is no title, set the title to be the file name.
	if m.Title() != "" {
		meta.Title = m.Title()
	}
 
	meta.Album = m.Album()
	meta.Artist = m.Artist()
	meta.AlbumArtist = m.AlbumArtist()
	meta.Composer = m.Composer()
	meta.Genre = m.Genre()
	meta.Year = m.Year()
	//meta.Track = m.Track()
	//meta.Disc = m.Disc()
	meta.Artwork = m.Picture()
	meta.Lyrics = m.Lyrics()
	meta.Comment = m.Comment()
 
	return meta
}
 
// Custom type used for track queue.
type Queue struct {
	Tracks    []string
	PlayOrder []int
	Index     int
}
 
// Returns true if path leads to an acceptable audio file.
func isAudioFile(path string) bool {
	// Open the file for reading.
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
 
	// Read the file header.
	header := make([]byte, 12)
	n, err := f.Read(header)
	if err != nil {
		return false
	}
	header = header[:n]
 
	// MP3 (ID3 tag)
	if bytes.HasPrefix(header, []byte("ID3")) {
		return true
	}
 
	// MP3 (frame sync) — starts with 0xFF 0xFB or similar
	if len(header) >= 2 && header[0] == 0xFF && (header[1]&0xE0) == 0xE0 {
		return true
	}
 
	// FLAC
	if bytes.HasPrefix(header, []byte("fLaC")) {
		return true
	}
 
	// OGG (Vorbis, Opus, etc.)
	if bytes.HasPrefix(header, []byte("OggS")) {
		return true
	}
 
	// WAV (RIFF + WAVE)
	if len(header) >= 12 &&
		bytes.HasPrefix(header, []byte("RIFF")) &&
		bytes.Contains(header, []byte("WAVE")) {
		return true
	}
 
	// M4A / MP4
	if len(header) >= 8 && bytes.Equal(header[4:8], []byte("ftyp")) {
		return true
	}
 
	return false
}
 
// Add files or directories of files to a queue.
func addToQueue(queue *Queue, path string, recursive bool) error {
	pathInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("could not evaluate file path: %w", err)
	}
 
	// Check if path is for a file or a directory.
	if !(pathInfo.IsDir()) {
		if isAudioFile(path) {
			queue.Tracks = append(queue.Tracks, path)
			queue.PlayOrder = append(queue.PlayOrder, len(queue.PlayOrder))
		}
	} else if pathInfo.IsDir() {
		// Read the directory.
		files, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("could not read directory: %w", err)
		}
 
		// Basically for file in directory.
		for _, file := range files {
			fileName := file.Name()
			fullPath := filepath.Join(path, fileName)
 
			pathInfo, err = file.Info()
			if err != nil {
				return fmt.Errorf("could not evaluate file path: %w", err)
			}
 
			if pathInfo.IsDir() {
				if recursive {
					if err := addToQueue(queue, fullPath, true); err != nil {
						return err
					}
				}
			} else if isAudioFile(fullPath) {
				queue.Tracks = append(queue.Tracks, fullPath)
				queue.PlayOrder = append(queue.PlayOrder, len(queue.PlayOrder))
			}
		}
	}
	return nil
}
 
// Shuffle inputted PlayOrder slice. Since slices are referanced types, we don't need to use a pointer.
func shufflePlayOrder(playOrder []int) {
	// Seed rng.
	rand.Seed(time.Now().UnixNano())
 
	rand.Shuffle(len(playOrder), func(i, j int) { playOrder[i], playOrder[j] = playOrder[j], playOrder[i] })
}
 
// Tack track image data as input and return ascii art string.
func renderTrackImage(artwork *tag.Picture) string {
	// Return a default image if no artwork is found.
	if artwork == nil {
		return "                           ..;::;,.         \n                 .:dOKX0xoc:;,,,,;coxd,     \n            ':dOXOo;'.            c  dXd    \n       .:dKXKd;.                .k0   KX    \n     'xXXO:.               ..;oOXXl   lX;   \n   .kXXx.  lOdllcc::cldxOKXXX0d;OX.    Xk   \n  .KXX,    Ok,clooooooooc;'.    Xx     kK   \n  KXX,     dX                  lX.     OX   \n  XXc      ,X:                ,Xd      OX.  \n .XO        Xc                kX,      kX.  \n ;X;        Xo                dX.     .Xx   \n lX'        K0         ;dO0Od,cX.     xX;   \n xX'  .::.  cX.       .XO..c0XXX.    ;XX    \n xX; oXOkXO,cXc        lXo,..oXX,   .XXl    \n oXO kXc 'kXXX,         .:dkkko'  .oXXl     \n  OXx.oK0O0Od:                  'xXXX;      \n   cKXd'                     .c00xl,        \n     ;dOKOdl:,''..',:ccodkOkxc.             \n          ...'',;;;;;;,..                   \n                             "
	}
 
	// Raw image data should be in artwork.Data
	// Reader for raw image data.
	reader := bytes.NewReader(artwork.Data)
 
	// Decode the image data into an image.Image object
	img, _, err := image.Decode(reader)
	if err != nil {
		panic(err)
	}
 
	// Create convert options
	convertOptions := convert.DefaultOptions
	convertOptions.Colored = true
	convertOptions.FixedWidth = 44
	convertOptions.FixedHeight = 20
 
	// Create the image converter
	converter := convert.NewImageConverter()
 
	// Convert image.
	return converter.Image2ASCIIString(img, &convertOptions)
}
 
// Load file into mpv player.
func loadTrack(player *mpv.Mpv, path string) {
	err := player.Command([]string{"loadfile", path})
	if err != nil {
		fmt.Println("Failed to load file:", err)
		os.Exit(1)
	}
}
 
// Toggle mpv player's play/pause state.
func toggleTrackPause(player *mpv.Mpv) {
	err := player.Command([]string{"cycle", "pause"})
	if err != nil {
		fmt.Println("Failed to toggle play/pause:", err)
		os.Exit(1)
	}
}
 
// Mpv player seek.
func playerSeek(player *mpv.Mpv, seconds int) {
	// Check if any tracks are loaded before attempting to seek.
	trackCount, err := player.GetProperty("track-list/count", mpv.FormatInt64)
	if err != nil {
		fmt.Println("Failed to get track count:", err)
		os.Exit(1)
	}
 
	// If we have at least on track loaded, we can attempt to seek.
	if trackCount.(int64) > 0 {
		err := player.Command([]string{"seek", strconv.Itoa(seconds), "relative"})
		if err != nil {
			fmt.Println("Failed to seek:", err)
			os.Exit(1)
		}
	}
}
 
// Mpv player volume.
func playerSetVolume(player *mpv.Mpv, volume int) {
	err := player.Command([]string{"set", "volume", strconv.Itoa(volume)})
	if err != nil {
		fmt.Println("Failed to set volume:", err)
		os.Exit(1)
	}
}
 
// Mpv toggle mute.
func playerToggleMute(player *mpv.Mpv) {
	// Get the current status of the mute property.
	mute, err := player.GetProperty("mute", mpv.FormatFlag)
	if err != nil {
		fmt.Println("Failed to retrieve status of mute property:", err)
		os.Exit(1)
	}
 
	// Unwrap mute.
	isMuted := mute.(bool)
 
	// Decide what action to take in the command based on isMuted.
	setMute := "yes"
	if isMuted {
		setMute = "no"
	}
 
	err = player.Command([]string{"set", "mute", setMute})
	if err != nil {
		fmt.Println("Failed to toggle mute:", err)
		os.Exit(1)
	}
}
 
// Get track duration and timestamp.
func playerGetTimestamp(player *mpv.Mpv) (float64, float64) {
	// Get track duration.
	duration, err := player.GetProperty("duration", mpv.FormatDouble)
	if err != nil {
		return 0, 0
	}
 
	// Get timestamp.
	timeStamp, err := player.GetProperty("time-pos", mpv.FormatDouble)
	if err != nil {
		return 0, 0
	}
 
	// Unpack timeStamp and duration interfaces.
	_, ok1 := duration.(float64)
	_, ok2 := timeStamp.(float64)
 
	// Return zeros if duration or timestamp contained something other than flaot64.
	if !ok1 || !ok2 {
		return 0, 0
	}
 
	return timeStamp.(float64), duration.(float64)
}
 
// Convert float timestamp to mm:ss format.
func formatTimestamp(s float64) string {
	total := int(s)
	minutes := total / 60
	seconds := total % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
 
// Return "On" for true and "Off" for false.
func boolToOnOff(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}
 
// Helper function for creating a progress bar.
func renderProgressBar(current, total float64, width int) string {
	if total <= 0 {
		return strings.Repeat("-", width)
	}
 
	ratio := current / total
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
 
	filled := int(ratio * float64(width))
	empty := width - filled
 
	return strings.Repeat("#", filled) + strings.Repeat("-", empty)
}
 
// Tick message type used for bubbletea's timer.
type tickMsg time.Time
 
// Tick command for bubbletea.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg { return tickMsg(t) })
}
 
func listenMpvEvents(ch <-chan mpv.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return ev
	}
}
 
// ---------------------------------------------------------------------------
// Player screen
// ---------------------------------------------------------------------------
 
// Model used for the player screen.
type playerModel struct {
	image         string
	playing       bool
	shuffle       bool
	loopTrack     bool
	loopPlaylist  bool
	trackMetadata TrackMetadata
	player        *mpv.Mpv
	volume        int
	mute          bool
	mpvEvents     chan mpv.Event
	queue         *Queue
	manualSkip    bool
}
 
// Initialize the player model.
func initialPlayerModel(shuffle bool, loopTrack bool, loopPlaylist bool, trackMetadata TrackMetadata, player *mpv.Mpv, mpvEvents chan mpv.Event, queue *Queue, image string) playerModel {
	return playerModel{
		image:        image,
		playing:      true,
		shuffle:      shuffle,
		loopTrack:    loopTrack,
		loopPlaylist: loopPlaylist,
		trackMetadata: trackMetadata,
		player:       player,
		volume:       100,
		mute:         false,
		mpvEvents:    mpvEvents,
		queue:        queue,
		manualSkip:   false,
	}
}
 
// Bubbletea update function for the player screen.
func (m playerModel) update(msg tea.Msg) (playerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		return m, tickCmd()
	case mpv.Event:
		// What to do at the end of a track.
		if msg.EventID == mpv.EventEnd {
			// Check for manualSkip flag.
			if m.manualSkip {
				m.manualSkip = false
				return m, listenMpvEvents(m.mpvEvents)
			}
 
			// Logic for loopTrack.
			if m.loopTrack {
				loadTrack(m.player, m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
				m.trackMetadata = *getTrackMetaData(m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
				m.image = renderTrackImage(m.trackMetadata.Artwork)
				return m, listenMpvEvents(m.mpvEvents)
			}
 
			// Keep index in bounds.
			if (m.queue.Index + 1) < len(m.queue.PlayOrder) {
				m.queue.Index++
				loadTrack(m.player, m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
				m.trackMetadata = *getTrackMetaData(m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
				m.image = renderTrackImage(m.trackMetadata.Artwork)
			} else if (m.queue.Index + 1) == len(m.queue.PlayOrder) {
				// Last track.
				if m.loopPlaylist {
					m.queue.Index = 0
					loadTrack(m.player, m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
					m.trackMetadata = *getTrackMetaData(m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
					m.image = renderTrackImage(m.trackMetadata.Artwork)
					return m, listenMpvEvents(m.mpvEvents)
				}
 
				// Last track. Not looping. Exit program.
				return m, tea.Quit
			}
		}
 
		return m, listenMpvEvents(m.mpvEvents)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// In order to exit, we first must stop and unload the current track.
			err := m.player.Command([]string{"stop"})
			if err != nil {
				fmt.Println("Failed to stop current track:", err)
				os.Exit(1)
			}
			return m, tea.Quit
		case "space", "mediaplay", "mediapause":
			m.playing = !m.playing
			toggleTrackPause(m.player)
		case "right":
			playerSeek(m.player, 5)
		case "left":
			playerSeek(m.player, -5)
		case "up":
			playerSeek(m.player, 60)
		case "down":
			playerSeek(m.player, -60)
		case "enter", "medianext":
			// Skip forwards a track.
			// Keep index in bounds.
			if (m.queue.Index + 1) < len(m.queue.PlayOrder) {
				m.manualSkip = true
				m.queue.Index++
				loadTrack(m.player, m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
				m.trackMetadata = *getTrackMetaData(m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
				m.image = renderTrackImage(m.trackMetadata.Artwork)
			} else if (m.queue.Index + 1) == len(m.queue.PlayOrder) {
				// LoopPlaylist logic
				if m.loopPlaylist {
					m.manualSkip = true
					m.queue.Index = 0
					loadTrack(m.player, m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
					m.trackMetadata = *getTrackMetaData(m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
					m.image = renderTrackImage(m.trackMetadata.Artwork)
				}
			}
		case "backspace", "mediaprev":
			// Skip backwards a track.
			// Keep index in bounds.
			if (m.queue.Index - 1) > -1 {
				m.manualSkip = true
				m.queue.Index--
				loadTrack(m.player, m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
				m.trackMetadata = *getTrackMetaData(m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
				m.image = renderTrackImage(m.trackMetadata.Artwork)
			} else if (m.queue.Index - 1) == -1 {
				// LoopPlaylist logic
				if m.loopPlaylist {
					m.manualSkip = true
					m.queue.Index = len(m.queue.PlayOrder) - 1
					loadTrack(m.player, m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
					m.trackMetadata = *getTrackMetaData(m.queue.Tracks[m.queue.PlayOrder[m.queue.Index]])
					m.image = renderTrackImage(m.trackMetadata.Artwork)
				}
			}
		case "-":
			if m.volume > 0 {
				m.volume -= 5
			} else {
				m.volume = 0
			}
			playerSetVolume(m.player, m.volume)
		case "=":
			if m.volume < 100 {
				m.volume += 5
			} else {
				m.volume = 100
			}
			playerSetVolume(m.player, m.volume)
		case "m":
			playerToggleMute(m.player)
			m.mute = !m.mute
		case "s":
			m.shuffle = !m.shuffle
 
			if m.shuffle {
				// Get current track position.
				currentTrack := m.queue.PlayOrder[m.queue.Index]
 
				// Shuffle PlayOrder
				shufflePlayOrder(m.queue.PlayOrder)
 
				// Find current track in shuffled PlayOrder and set index accordingly.
				for i, v := range m.queue.PlayOrder {
					if v == currentTrack {
						m.queue.Index = i
						break
					}
				}
			} else {
				// Set index to that of current track.
				m.queue.Index = m.queue.PlayOrder[m.queue.Index]
 
				// Change play order back to default.
				m.queue.PlayOrder = make([]int, len(m.queue.Tracks))
				for i := range m.queue.PlayOrder {
					m.queue.PlayOrder[i] = i
				}
			}
		case "l":
			m.loopTrack = !m.loopTrack
		case "L":
			m.loopPlaylist = !m.loopPlaylist
		}
	}
 
	return m, nil
}
 
// Helper function to render image panel.
func renderImagePanel(m playerModel) string {
	return m.image
}
 
// Helper function to render track info panel.
func renderTrackInfo(m playerModel) string {
	// Lipgloss styles.
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	linebreakStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
 
	// Build lines.
	lines := []string{
		"Current Track",
		linebreakStyle.Render("-------------"),
		labelStyle.Render("Title") + ": " + m.trackMetadata.Title,
		labelStyle.Render("Artist") + ": " + m.trackMetadata.Artist,
		labelStyle.Render("Album") + ": " + m.trackMetadata.Album,
		labelStyle.Render("Date") + ": " + strconv.Itoa(m.trackMetadata.Year),
		labelStyle.Render("Genre") + ": " + m.trackMetadata.Genre,
		"",
	}
 
	// Return concatenated list as a string.
	return strings.Join(lines, "\n")
}
 
// Helper function to render control panel.
func renderControls(m playerModel) string {
	// Lipgloss styles.
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	linebreakStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
 
	// Find play/pause status.
	PlayingStatus := "Playing"
	if !m.playing {
		PlayingStatus = "Paused"
	}
 
	// Create volume line accounting for mute state.
	volumeLine := labelStyle.Render("Volume") + ": " + strconv.Itoa(m.volume) + " %"
	if m.mute {
		volumeLine += " (MUTE)"
	}
 
	// Find timestamp and duration.
	timeStamp, duration := playerGetTimestamp(m.player)
 
	progress := labelStyle.Render("(") + formatTimestamp(timeStamp) + labelStyle.Render(" / ") + formatTimestamp(duration) + labelStyle.Render(")")
 
	progressBar := renderProgressBar(timeStamp, duration, 30)
 
	// Build lines.
	lines := []string{
		"Controls",
		linebreakStyle.Render("--------"),
		labelStyle.Render(PlayingStatus) + " " + progress,
		volumeLine,
		labelStyle.Render("Shuffle") + ": " + boolToOnOff(m.shuffle),
		labelStyle.Render("Loop Track") + ": " + boolToOnOff(m.loopTrack),
		labelStyle.Render("Loop Playlist") + ": " + boolToOnOff(m.loopPlaylist),
		"",
		labelStyle.Render("[") + progressBar + labelStyle.Render("]") + labelStyle.Render(" (") + strconv.FormatFloat((timeStamp/duration)*100, 'f', 0, 64) + " %" + labelStyle.Render(")"),
	}
 
	// Return concatenated list as a string.
	return strings.Join(lines, "\n")
}
 
// Render the player screen.
func renderPlayerScreen(m playerModel) string {
	// Render image
	imagePanel := lipgloss.NewStyle().PaddingRight(2).Render(renderImagePanel(m))
 
	// Render track panel.
	trackPanel := renderTrackInfo(m)
 
	// Render control panel.
	controlPanel := renderControls(m)
 
	// Use lipgloss to create the layout.
	// First we set up the middle column, consisting of the track panel and control panel.
	middleColumn := lipgloss.JoinVertical(lipgloss.Left, trackPanel, controlPanel)
 
	// Then, the final layout. Image to the left, next to the middle column.
	return lipgloss.JoinHorizontal(lipgloss.Top, imagePanel, middleColumn)
}
 
// ---------------------------------------------------------------------------
// Queue editor screen
// ---------------------------------------------------------------------------
 
const (
	queueViewportHeight = 20 // number of track rows visible at once
	queueScrollPadding  = 3  // rows of context kept above/below the cursor
)
 
// queueModel is the model for the queue editor screen.
type queueModel struct {
	queue        *Queue
	player       *mpv.Mpv
	cursor       int  // current cursor row in PlayOrder
	scrollOffset int  // index of the first visible row
	selected     int  // index of grabbed track in PlayOrder, -1 = none
	adding       bool // true = path input mode
	input        string
	addError     string
	searching    bool   // true = currently typing a search query
	searchQuery  string // active/last query (persists after esc like vim)
	matches      []int  // PlayOrder indices whose display name matches searchQuery
	matchCursor  int    // index into matches for n/N navigation
}
 
// Initialize the queue model.
func initialQueueModel(queue *Queue, player *mpv.Mpv) queueModel {
	return queueModel{
		queue:        queue,
		player:       player,
		cursor:       queue.Index,
		scrollOffset: 0,
		selected:     -1,
		adding:       false,
		input:        "",
		addError:     "",
		searching:    false,
		searchQuery:  "",
		matches:      nil,
		matchCursor:  0,
	}
}
 
// Update for the queue editor screen. Returns the updated model and a command.
func (m queueModel) update(msg tea.Msg) (queueModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Add-path input mode.
		if m.adding {
			switch msg.String() {
			case "esc":
				m.adding = false
				m.input = ""
				m.addError = ""
		case "enter":
			path := strings.TrimSpace(m.input)
			if path != "" {
				// Try to add tracks from the given path.
				before := len(m.queue.Tracks)
				if err := addToQueue(m.queue, path, true); err != nil {
					m.addError = "Invalid path: " + path
				} else {
					added := len(m.queue.Tracks) - before
					if added == 0 {
						m.addError = "No audio files found at: " + path
					} else {
						m.addError = ""
					}
				}
			}
			m.adding = false
			m.input = ""
			case "backspace":
				if len(m.input) > 0 {
					m.input = m.input[:len(m.input)-1]
				}
			default:
				// Append printable characters to input.
				if msg.String() == "space" {
					m.input += " "
				} else if len(msg.String()) == 1 {
					m.input += msg.String()
				}
			}
			return m, nil
		}
 
		// Search input mode.
		if m.searching {
			switch msg.String() {
			case "esc":
				// Exit search mode but keep query and matches (vim-style).
				m.searching = false
			case "enter":
				// Confirm query, exit search mode, stay on current match.
				m.searching = false
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.matches = computeMatches(m.queue, m.searchQuery)
					m.matchCursor = 0
					if len(m.matches) > 0 {
						m.cursor = m.matches[0]
					}
				}
			default:
				ch := msg.String()
				if ch == "space" {
					ch = " "
				}
				if len(ch) == 1 {
					m.searchQuery += ch
					m.matches = computeMatches(m.queue, m.searchQuery)
					m.matchCursor = 0
					if len(m.matches) > 0 {
						m.cursor = m.matches[0]
					}
				}
			}
			m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, len(m.queue.PlayOrder))
			return m, nil
		}
 
		// Normal navigation mode.
		total := len(m.queue.PlayOrder)
		switch msg.String() {
		case "j":
			if m.cursor < total-1 {
				m.cursor++
			}
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "home":
			m.cursor = 0
		case "end":
			if total > 0 {
				m.cursor = total - 1
			}
		case "pgup":
			m.cursor -= queueViewportHeight
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "pgdown":
			m.cursor += queueViewportHeight
			if m.cursor >= total {
				m.cursor = total - 1
			}
		case "g":
			// Jump to the currently playing track.
			m.cursor = m.queue.Index
		case "enter":
			if m.selected == -1 {
				// Select/grab the track under the cursor.
				m.selected = m.cursor
			} else {
				// Drop the grabbed track at the cursor position.
				if m.selected != m.cursor {
					m.queue.PlayOrder = moveElement(m.queue.PlayOrder, m.selected, m.cursor)
					// Adjust queue.Index to follow the currently playing track.
					m.queue.Index = findIndex(m.queue.Index, m.selected, m.cursor)
				}
				m.selected = -1
			}
		case "esc":
			m.selected = -1
		case "d":
			if m.selected != -1 {
				// Remove the selected track.
				removed := m.selected
				m.queue.PlayOrder = removeElement(m.queue.PlayOrder, removed)
				// Adjust queue.Index.
				m.queue.Index = adjustIndexAfterRemove(m.queue.Index, removed, len(m.queue.PlayOrder))
				// Clamp cursor to valid range after removal.
				if m.cursor >= len(m.queue.PlayOrder) && m.cursor > 0 {
					m.cursor = len(m.queue.PlayOrder) - 1
				}
				m.selected = -1
				// Recompute matches since PlayOrder changed.
				m.matches = computeMatches(m.queue, m.searchQuery)
			}
		case "a":
			m.adding = true
			m.input = ""
			m.addError = ""
		case "/":
			// Enter search mode, starting a fresh query.
			m.searching = true
			m.searchQuery = ""
			m.matches = nil
			m.matchCursor = 0
		case "n":
			// Jump to next match (wrap).
			if len(m.matches) > 0 {
				m.matchCursor = (m.matchCursor + 1) % len(m.matches)
				m.cursor = m.matches[m.matchCursor]
			}
		case "N":
			// Jump to previous match (wrap).
			if len(m.matches) > 0 {
				m.matchCursor = (m.matchCursor - 1 + len(m.matches)) % len(m.matches)
				m.cursor = m.matches[m.matchCursor]
			}
		case "ctrl+c", "q":
			// In order to exit, we first must stop and unload the current track.
			err := m.player.Command([]string{"stop"})
			if err != nil {
				fmt.Println("Failed to stop current track:", err)
				os.Exit(1)
			}
			return m, tea.Quit
		}
		m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, len(m.queue.PlayOrder))
	}
	return m, nil
}
 
// clampScroll adjusts scrollOffset so the cursor stays within the visible
// viewport with at least `padding` rows of context above and below when possible.
func clampScroll(cursor, offset, total int) int {
	// Scroll up if cursor is above the padded top boundary.
	if cursor < offset+queueScrollPadding {
		offset = cursor - queueScrollPadding
	}
	// Scroll down if cursor is below the padded bottom boundary.
	if cursor >= offset+queueViewportHeight-queueScrollPadding {
		offset = cursor - queueViewportHeight + queueScrollPadding + 1
	}
	// Hard clamp: offset must stay in [0, total-viewportHeight].
	if offset < 0 {
		offset = 0
	}
	maxOffset := total - queueViewportHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	return offset
}
 
// trackDisplayName returns the display name for a track path (filename without extension).
func trackDisplayName(path string) string {
	name := filepath.Base(path)
	if ext := filepath.Ext(name); ext != "" {
		name = name[:len(name)-len(ext)]
	}
	return name
}
 
// computeMatches returns the PlayOrder positions whose display name contains
// query (case-insensitive). Returns nil if query is empty.
func computeMatches(queue *Queue, query string) []int {
	if query == "" {
		return nil
	}
	lower := strings.ToLower(query)
	var result []int
	for i, trackIdx := range queue.PlayOrder {
		name := strings.ToLower(trackDisplayName(queue.Tracks[trackIdx]))
		if strings.Contains(name, lower) {
			result = append(result, i)
		}
	}
	return result
}
 
// moveElement moves the element at index `from` to index `to` in a slice,
// shifting other elements as needed. Returns a new slice.
func moveElement(s []int, from, to int) []int {
	result := make([]int, len(s))
	copy(result, s)
	val := result[from]
	// Remove from current position.
	result = append(result[:from], result[from+1:]...)
	// Insert at target position.
	result = append(result[:to], append([]int{val}, result[to:]...)...)
	return result
}
 
// removeElement removes the element at index i from a slice.
func removeElement(s []int, i int) []int {
	result := make([]int, 0, len(s)-1)
	result = append(result, s[:i]...)
	result = append(result, s[i+1:]...)
	return result
}
 
// findIndex returns the new queue.Index after a move operation.
// oldIndex is the current queue.Index (position in PlayOrder).
// from/to are the positions that were just swapped.
func findIndex(oldIndex, from, to int) int {
	// The currently playing track's PlayOrder position may have shifted.
	// We moved the element at `from` to `to`.
	if oldIndex == from {
		return to
	}
	// If the playing track was between from and to, it shifted by one.
	if from < to {
		// Moving down: tracks between from+1..to shift up by 1.
		if oldIndex > from && oldIndex <= to {
			return oldIndex - 1
		}
	} else {
		// Moving up: tracks between to..from-1 shift down by 1.
		if oldIndex >= to && oldIndex < from {
			return oldIndex + 1
		}
	}
	return oldIndex
}
 
// adjustIndexAfterRemove adjusts queue.Index after a track has been removed.
// removed is the PlayOrder index that was deleted.
// newLen is the length of PlayOrder after removal.
func adjustIndexAfterRemove(currentIndex, removed, newLen int) int {
	if newLen == 0 {
		return 0
	}
	if currentIndex > removed {
		// Playing track shifted up.
		return currentIndex - 1
	}
	if currentIndex == removed {
		// The playing track was removed; clamp to valid range.
		if currentIndex >= newLen {
			return newLen - 1
		}
		return currentIndex
	}
	return currentIndex
}
 
// Render the queue editor screen.
func renderQueueScreen(m queueModel) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	linebreakStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	playingStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	matchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
 
	// Build a set of match positions for O(1) lookup during row rendering.
	matchSet := make(map[int]bool, len(m.matches))
	for _, idx := range m.matches {
		matchSet[idx] = true
	}
 
	trackCount := len(m.queue.PlayOrder)
 
	// Compute the visible window.
	viewStart := m.scrollOffset
	viewEnd := viewStart + queueViewportHeight
	if viewEnd > trackCount {
		viewEnd = trackCount
	}
 
	// Header with scroll position indicator when the list doesn't fit on screen.
	header := labelStyle.Render("Queue") + " (" + strconv.Itoa(trackCount) + " tracks)"
	if trackCount > queueViewportHeight {
		header += "  " + linebreakStyle.Render(
			"["+strconv.Itoa(viewStart+1)+"-"+strconv.Itoa(viewEnd)+"]",
		)
	}
	separator := linebreakStyle.Render("------------------")
 
	var lines []string
	lines = append(lines, header)
	lines = append(lines, separator)
 
	for i := viewStart; i < viewEnd; i++ {
		trackIdx := m.queue.PlayOrder[i]
		name := trackDisplayName(m.queue.Tracks[trackIdx])
		num := strconv.Itoa(i+1) + "."
 
		isPlaying := (i == m.queue.Index)
		isCursor := (i == m.cursor)
		isSelected := (i == m.selected)
		isMatch := matchSet[i]
 
		prefix := "  "
		if isCursor {
			prefix = "> "
		}
 
		var row string
		switch {
		case isSelected:
			row = prefix + selectedStyle.Render(num+" ["+name+"]")
		case isCursor && isPlaying:
			row = prefix + cursorStyle.Render(num) + " " + playingStyle.Render("♪ "+name)
		case isCursor:
			row = prefix + cursorStyle.Render(num+" "+name)
		case isPlaying:
			row = prefix + num + " " + playingStyle.Render("♪ "+name)
		case isMatch:
			row = prefix + matchStyle.Render(num+" "+name)
		default:
			row = prefix + num + " " + name
		}
 
		lines = append(lines, row)
	}
 
	lines = append(lines, "")
 
	// Status area: search prompt, add prompt, or error — mutually exclusive.
	switch {
	case m.adding:
		lines = append(lines, labelStyle.Render("Add path: ")+m.input+"_")
	case m.searching:
		status := linebreakStyle.Render("/") + " " + m.searchQuery + "_"
		if len(m.matches) > 0 {
			status += "  " + linebreakStyle.Render(
				"["+strconv.Itoa(m.matchCursor+1)+"/"+strconv.Itoa(len(m.matches))+"]",
			)
		} else if m.searchQuery != "" {
			status += "  " + errorStyle.Render("no matches")
		}
		lines = append(lines, status)
	case m.addError != "":
		lines = append(lines, errorStyle.Render(m.addError))
	case len(m.matches) > 0:
		// Query is active but not in search mode — show match count as a reminder.
		lines = append(lines, linebreakStyle.Render(
			"/"+m.searchQuery+"  ["+strconv.Itoa(m.matchCursor+1)+"/"+strconv.Itoa(len(m.matches))+"]",
		))
	}
 
	// Key hint bar.
	hints := linebreakStyle.Render("[j/k]") + " Navigate  " +
		linebreakStyle.Render("[home/end]") + " Top/Bottom  " +
		linebreakStyle.Render("[pgup/pgdn]") + " Page  " +
		linebreakStyle.Render("[g]") + " Current track  " +
		linebreakStyle.Render("[enter]") + " Select/Drop  " +
		linebreakStyle.Render("[esc]") + " Deselect  " +
		linebreakStyle.Render("[d]") + " Remove  " +
		linebreakStyle.Render("[a]") + " Add  " +
		linebreakStyle.Render("[/]") + " Search  " +
		linebreakStyle.Render("[tab]") + " Player"
	if len(m.matches) > 0 {
		hints += "  " + linebreakStyle.Render("[n/N]") + " Next/Prev match"
	}
	lines = append(lines, hints)
 
	return strings.Join(lines, "\n")
}
 
// ---------------------------------------------------------------------------
// Root app model
// ---------------------------------------------------------------------------
 
type activeScreen int
 
const (
	screenPlayer activeScreen = iota
	screenQueue
)
 
// appModel is the root bubbletea model. It owns the shared state and
// delegates to whichever child screen is currently active.
type appModel struct {
	screen activeScreen
	player playerModel
	queue  queueModel
}
 
// initialAppModel constructs the root model.
func initialAppModel(pm playerModel, qm queueModel) appModel {
	return appModel{
		screen: screenPlayer,
		player: pm,
		queue:  qm,
	}
}
 
// Init starts the tick and mpv event listener.
func (m appModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), listenMpvEvents(m.player.mpvEvents))
}
 
// Update routes messages to the appropriate child screen.
func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Tab always switches screens, regardless of active screen.
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "tab" {
		if m.screen == screenPlayer {
			m.screen = screenQueue
			// Sync cursor to currently playing track when entering queue screen.
			m.queue.cursor = m.player.queue.Index
		} else {
			m.screen = screenPlayer
		}
		return m, nil
	}
 
	// mpv events are always routed to the player (playback must continue
	// regardless of which screen is visible), but we must re-subscribe.
	if ev, ok := msg.(mpv.Event); ok {
		var cmd tea.Cmd
		m.player, cmd = m.player.update(ev)
		return m, cmd
	}
 
	// Ticks must always be re-scheduled regardless of the active screen,
	// otherwise switching to the queue screen permanently breaks the tick
	// chain and the player stops updating automatically.
	if _, ok := msg.(tickMsg); ok {
		return m, tickCmd()
	}
 
	// Route all other messages to the active screen.
	switch m.screen {
	case screenPlayer:
		var cmd tea.Cmd
		m.player, cmd = m.player.update(msg)
		return m, cmd
	case screenQueue:
		var cmd tea.Cmd
		m.queue, cmd = m.queue.update(msg)
		return m, cmd
	}
 
	return m, nil
}
 
// View renders the active screen.
func (m appModel) View() tea.View {
	var s string
	switch m.screen {
	case screenPlayer:
		s = renderPlayerScreen(m.player)
	case screenQueue:
		s = renderQueueScreen(m.queue)
	}
	return tea.NewView(s)
}
 
// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------
 
func main() {
	// Command line arguments.
	flag.Usage = printHelp
	shuffle := flag.Bool("s", false, "Shuffle")
	loopTrack := flag.Bool("l", false, "Loop track")
	loopPlaylist := flag.Bool("L", false, "Loop playlist")
	flag.Parse()
 
	// Final argument expected to be a path.
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Error: missing required argument.\nTry running with -h for help.")
		os.Exit(1)
	} else if len(args) > 1 {
		fmt.Println("Error: Invalid arguments Provided.\nTry running with -h for help.")
		os.Exit(1)
	}
 
	// Get file/directory name from final argument.
	path := args[0]
 
	// Create new empty queue.
	q := &Queue{
		Tracks:    []string{},
		PlayOrder: []int{},
		Index:     0,
	}
 
	// Add initial tracks to queue via provided path.
	if err := addToQueue(q, path, true); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
 
	// Initial shuffle
	if *shuffle {
		shufflePlayOrder(q.PlayOrder)
	}
 
	// Extract meta data for initial track.
	trackMetaData := getTrackMetaData(q.Tracks[q.PlayOrder[q.Index]])
 
	// Render artwork for initial track.
	initialArtwork := renderTrackImage(trackMetaData.Artwork)
 
	// Create mpv instance.
	player := mpv.New()
	defer player.TerminateDestroy()
 
	// Configure mpv.
	// Disable video by setting video output to null.
	_ = player.SetPropertyString("vo", "null")
 
	// Initialize mpv player.
	err := player.Initialize()
	if err != nil {
		fmt.Println("Failed to initialize mpv player:", err)
		os.Exit(1)
	}
 
	// Start mpv event handler loop.
	mpvEvents := make(chan mpv.Event)
 
	go func() {
		for {
			ev := player.WaitEvent(1000)
			if ev.EventID == mpv.EventShutdown {
				close(mpvEvents)
				return
			}
			mpvEvents <- *ev
		}
	}()
 
	// Load and play file.
	loadTrack(player, q.Tracks[q.PlayOrder[q.Index]])
 
	// Build child models. Both share the same *Queue pointer.
	pm := initialPlayerModel(*shuffle, *loopTrack, *loopPlaylist, *trackMetaData, player, mpvEvents, q, initialArtwork)
	qm := initialQueueModel(q, player)
 
	// Create our bubbletea program.
	p := tea.NewProgram(initialAppModel(pm, qm))
 
	// Make sure our bubbletea program launches without an error.
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
