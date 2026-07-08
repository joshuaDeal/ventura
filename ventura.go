package main

import (
	"fmt"
	"os"
	"strings"
	"flag"
	"log"
	"errors"
	"strconv"
	"time"
	"bytes"
	"path/filepath"
	"math/rand"
	"image"
	_ "image/jpeg"
	_ "image/png"
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
	Title string
	Album string
	Artist string
	AlbumArtist string
	Composer string
	Genre string
	Year int
	//Track (int, int)
	//Disc (int, int)
	Artwork *tag.Picture
	Lyrics string
	Comment string
}

// Extract metadata from an audio track.
func getTrackMetaData(path string) *TrackMetadata {
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
			return &TrackMetadata{}
		}

		// All other errors.
		log.Fatal(err)
	}

	return &TrackMetadata{
		Title: m.Title(),
		Album: m.Album(),
		Artist: m.Artist(),
		AlbumArtist: m.AlbumArtist(),
		Composer: m.Composer(),
		Genre: m.Genre(),
		Year: m.Year(),
		//Track: m.Track(),
		//Disc: m.Disc(),
		Artwork: m.Picture(),
		Lyrics: m.Lyrics(),
		Comment: m.Comment(),
	}
}

// Custom type used for track queue.
type Queue struct {
	Tracks []string
	PlayOrder []int
	Index int
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
func addToQueue(queue *Queue, path string, recursive bool) {
	pathInfo, err := os.Stat(path)
	if err != nil {
		fmt.Println("Could not evaluate file path:", err)
		os.Exit(1)
	}

	// Check if path is for a file or a directory.
	if !(pathInfo.IsDir()) {
		if (isAudioFile(path)) {
			queue.Tracks = append(queue.Tracks, path)
			queue.PlayOrder = append(queue.PlayOrder, len(queue.PlayOrder))
		}
	} else if (pathInfo.IsDir()) {
		// Read the directory.
		files, err := os.ReadDir(path)
		if err != nil {
			fmt.Println("Could not read directory:", err)
			os.Exit(1)
		}

		// Basically for file in directory.
		for _, file := range files {
			fileName := file.Name()
			fullPath := filepath.Join(path, fileName)

			pathInfo, err = file.Info()
			if err != nil {
				fmt.Println("Could not evaluate file path:", err)
				os.Exit(1)
			}

			if (pathInfo.IsDir()) {
				if recursive {
					addToQueue(queue, fullPath, true)
				}
			} else if (isAudioFile(fullPath)) {
				queue.Tracks = append(queue.Tracks, fullPath)
				queue.PlayOrder = append(queue.PlayOrder, len(queue.PlayOrder))
			}
		}
	}
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
		return "                   -`                 \n                  .o+`                \n                 `ooo/                \n                `+oooo:               \n               `+oooooo:              \n               -+oooooo+:             \n             `/:-:++oooo+:            \n            `/++++/+++++++:           \n           `/++++++++++++++:          \n          `/+++ooooooooooooo/`        \n         ./ooosssso++osssssso+`       \n        .oossssso-````/ossssss+`      \n       -osssssso.      :ssssssso.     \n      :osssssss/        osssso+++.    \n     /ossssssss/        +ssssooo/-    \n   `/ossssso+/:-        -:/+osssso+-  \n  `+sso+:-`                 `.-/+oso:\n `++:.                           `-/+/\n .`                                 `/"
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

// Tick message type used for bubbletea's timer.
type tickMsg time.Time

// Tick command for bubbletea.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Model used for bubbletea.
type model struct {
	image string
	playing bool
	shuffle bool
	loopTrack bool
	loopPlaylist bool
	trackMetadata TrackMetadata
	player *mpv.Mpv
	volume int
	mute bool
	mpvEvents chan mpv.Event
	queue Queue
	manualSkip bool
}

// Initialize the model used by bubbletea.
func initialModel(shuffle bool, loopTrack bool, loopPlaylist bool, trackMetadata TrackMetadata, player *mpv.Mpv, mpvEvents chan mpv.Event, queue Queue, image string) model {
	m := model {
		image: image,
		playing: true,
		shuffle: shuffle,
		loopTrack: loopTrack,
		loopPlaylist: loopPlaylist,
		trackMetadata: trackMetadata,
		player: player,
		volume: 100,
		mute: false,
		mpvEvents: mpvEvents,
		queue: queue,
		manualSkip: false,
	}

	return m
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

// Bubbletea init function. Start things that happen after the program launches.
func (m model) Init() tea.Cmd {
	// Start timer for periodic updates.
	return tea.Batch(tickCmd(), listenMpvEvents(m.mpvEvents))
}

// bubbletea update function. Update values. Listen for keypresses.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
		case tickMsg:
			return m, tickCmd()
		case mpv.Event:
			// What to do at the end of a track.
			if (msg.EventID == mpv.EventEnd) {
				// Check for manualSkip flag.
				if (m.manualSkip) {
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
				if ((m.queue.Index + 1) < len(m.queue.PlayOrder)) {
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
					if ((m.queue.Index + 1) < len(m.queue.PlayOrder)) {
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
					if ((m.queue.Index - 1) > -1) {
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
					if (m.volume > 0) {
						m.volume -= 5
					} else {
						m.volume = 0
					}
					playerSetVolume(m.player, m.volume)
				case "=":
					if (m.volume < 100) {
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
func renderImagePanel(m model) string {
	// Return image.
	return m.image
}

// Helper function to render track info panel.
func renderTrackInfo(m model) string {
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

// Helper function to render control panel.
func renderControls(m model) string {
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
		labelStyle.Render("[") + progressBar + labelStyle.Render("]") + labelStyle.Render(" (") + strconv.FormatFloat((timeStamp / duration) * 100, 'f', 0, 64) + " %" + labelStyle.Render(")"),
	}

	// Return concatenated list as a string.
	return strings.Join(lines, "\n")
}

// Bubbletea view function. Ran after each update. Updates visuals.
func (m model) View() tea.View {
	// Render image
	imagePanel := lipgloss.NewStyle().PaddingRight(2).Render(renderImagePanel(m))

	// Render track panel.
	trackPanel := renderTrackInfo(m)

	// Render control panel.
	controlPanel := renderControls(m)

	// Use lipgloss to create the layout.
	// First we set  up the middle column, consisting of the track panel and control panel.
	middleColumn := lipgloss.JoinVertical(lipgloss.Left, trackPanel, controlPanel,)

	// Then, the final layout. Image to the left, next to the middle column.
	s := lipgloss.JoinHorizontal(lipgloss.Top, imagePanel, middleColumn,)

	return tea.NewView(s)
}

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
	q := Queue {
		Tracks: []string{},
		PlayOrder: []int{},
		Index: 0,
	}

	// Add initial tracks to queue via provided path.
	addToQueue(&q, path, true)

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

	// Create our bubbletea program.
	p := tea.NewProgram(initialModel(*shuffle, *loopTrack, *loopPlaylist, *trackMetaData, player, mpvEvents, q, initialArtwork))

	// Make sure our bubbletea program launches without an error.
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
