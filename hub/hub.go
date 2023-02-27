package hub

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os/exec"
	"sync"
	"time"

	soundcloud "github.com/zackradisic/soundcloud-api"
)

const (
	// bitrate in bytes
	// default bitrate is 128kbps
	// in bytes = 128 * 1000 / 8
	BITRATE = 16000

	// Lofi playlist
	PLAYLIST = "https://soundcloud.com/mustafkhan/sets/greatest_bollywood_songs"
)

type Client struct {
	Bytes chan []byte
}

type Hub struct {
	lock      *sync.RWMutex
	Consumers map[*Client]bool
	file      *bytes.Reader
	playlist  soundcloud.Playlist
	current   int
	sc        *soundcloud.API
}

func NewHub() *Hub {
	return &Hub{
		Consumers: make(map[*Client]bool),
		lock:      &sync.RWMutex{},
		file:      &bytes.Reader{},
		playlist:  soundcloud.Playlist{},
		current:   0,
		sc:        &soundcloud.API{},
	}
}

func (h *Hub) Add(c *Client) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if _, ok := h.Consumers[c]; !ok {
		h.Consumers[c] = true
	}
}

func (h *Hub) Remove(c *Client) {
	h.lock.Lock()
	defer h.lock.Unlock()
	delete(h.Consumers, c)
}

func (h *Hub) fill() error {

	song := h.playlist.Tracks[h.current]

	url, err := h.sc.GetDownloadURL(song.PermalinkURL, "")
	if err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// input
	i, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// output
	out := &bytes.Buffer{}

	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-map_metadata", "-1",
		"-c:a", "libmp3lame",
		"-vsync", "2",
		"-b:a", "128k",
		"-f", "mp3",
		"pipe:1",
	)
	// cmd.Stderr = os.Stderr
	cmd.Stdout = out

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	_, err = stdin.Write(i)
	if err != nil {
		return err
	}

	err = stdin.Close()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	h.lock.Lock()
	h.file = bytes.NewReader(out.Bytes())
	h.lock.Unlock()
	return nil
}

func (h *Hub) load() error {
	// setup a soundcloud client
	sc, err := soundcloud.New(soundcloud.APIOptions{})
	if err != nil {
		return err
	}
	h.sc = sc

	// get tracks from playlist
	p, err := sc.GetPlaylistInfo(PLAYLIST)
	if err != nil {
		return err
	}

	// Shuffle tracks in playlist
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(p.Tracks), func(i, j int) {
		p.Tracks[i], p.Tracks[j] = p.Tracks[j], p.Tracks[i]
	})

	h.playlist = p

	err = h.fill()
	if err != nil {
		fmt.Println(err)
		h.next()
	}

	return nil
}

func (h *Hub) next() {
	n := len(h.playlist.Tracks)
	if h.current < (n - 1) {
		h.current += 1
	} else {
		h.current = 0
	}
	err := h.fill()
	if err != nil {
		h.next()
	}
}

func (h *Hub) Start() {

	err := h.load()
	if err != nil {
		fmt.Println(err)
	}

	buffer := make([]byte, BITRATE)

	ticker := time.NewTicker(time.Second)

	for {
		<-ticker.C
		h.lock.Lock()
		_, err := h.file.Read(buffer)
		h.lock.Unlock()
		if err != nil {
			if err == io.EOF {
				h.next()
				continue
			}
		}
		h.Broadcast(buffer)
	}
}

func (h *Hub) Broadcast(b []byte) {
	h.lock.RLock()
	for c := range h.Consumers {
		c.Bytes <- b
	}
	h.lock.RUnlock()
}
