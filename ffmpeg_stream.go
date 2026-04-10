package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Active FFmpeg transcodes (for shutdown cleanup).
var (
	ffmpegProcsMu sync.Mutex
	ffmpegProcs   []*exec.Cmd
)

func registerFFmpegProcess(cmd *exec.Cmd) {
	ffmpegProcsMu.Lock()
	defer ffmpegProcsMu.Unlock()
	ffmpegProcs = append(ffmpegProcs, cmd)
}

func unregisterFFmpegProcess(cmd *exec.Cmd) {
	ffmpegProcsMu.Lock()
	defer ffmpegProcsMu.Unlock()
	for i, c := range ffmpegProcs {
		if c == cmd {
			ffmpegProcs = append(ffmpegProcs[:i], ffmpegProcs[i+1:]...)
			return
		}
	}
}

// KillAllFFmpegProcesses sends SIGKILL to every FFmpeg child we started.
// Safe to call when -ffmpeg was not used (no-op).
func KillAllFFmpegProcesses() {
	if conf == nil || !conf.FFmpeg {
		return
	}
	ffmpegProcsMu.Lock()
	list := make([]*exec.Cmd, len(ffmpegProcs))
	copy(list, ffmpegProcs)
	ffmpegProcsMu.Unlock()

	for _, cmd := range list {
		if cmd == nil || cmd.Process == nil {
			continue
		}
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			log.Printf("ffmpeg shutdown: kill pid %d: %v", cmd.Process.Pid, err)
		}
	}
}

// flushResponseWriter passes each Write through to the underlying writer and
// calls Flush so chunked output reaches the browser quickly (helps avoid
// timeouts while FFmpeg is still probing / encoding the first fragment).
type flushResponseWriter struct {
	http.ResponseWriter
	f http.Flusher
}

func (w *flushResponseWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if n > 0 && w.f != nil {
		w.f.Flush()
	}
	return n, err
}

func responseWriterForCopy(w http.ResponseWriter) io.Writer {
	if f, ok := w.(http.Flusher); ok {
		return &flushResponseWriter{ResponseWriter: w, f: f}
	}
	return w
}

type byteCountWriter struct {
	w io.Writer
	n int64
}

func (c *byteCountWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// loopbackHTTPBase returns an http:// URL that reaches this server from the same machine
// (for FFmpeg to pull /stream while transcoding).
func loopbackHTTPBase() string {
	addr := conf.Http
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			return "http://127.0.0.1" + addr
		}
		return "http://127.0.0.1:8080"
	}
	switch host {
	case "", "0.0.0.0", "[::]", "::":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func ffmpegStream(w http.ResponseWriter, r *http.Request) {
	if !conf.FFmpeg {
		http.Error(w, "ffmpeg transcoding is disabled (run with -ffmpeg)", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, magnet, err := processMagnetURI(r)
	if err != nil {
		log.Println("ffmpeg stream magnet uri:", err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	torrent, ok := app.Get(magnet.InfoHash.String())
	if !ok {
		log.Printf("ffmpeg: torrent not found for infoHash %s\n", magnet.InfoHash.String())
		http.Error(w, "torrent not found", http.StatusNotFound)
		return
	}
	torrent.TrackConnectionActivity(r)

	input := loopbackHTTPBase() + "/stream?" + r.URL.RawQuery

	// FFmpeg must NOT be tied to r.Context().Done(): pausing the video often
	// cancels the request context (or the player closes the first connection)
	// even though the TCP stream could still be idle. Killing FFmpeg on every
	// context cancel makes pause look like "FFmpeg stopped". We only cancel
	// when copying to the client fails (real disconnect / broken pipe) or
	// when the process exits naturally.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, conf.FFmpegPath,
		"-hide_banner",
		"-loglevel", "warning",
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "4",
		"-i", input,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-tune", "zerolatency",
		"-g", "48",
		"-keyint_min", "48",
		"-sc_threshold", "0",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"pipe:1",
	)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("ffmpeg stdout pipe:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Println("ffmpeg start:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	registerFFmpegProcess(cmd)
	defer unregisterFFmpegProcess(cmd)

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	bcw := &byteCountWriter{w: responseWriterForCopy(w)}
	_, copyErr := io.Copy(bcw, stdout)
	if copyErr != nil {
		log.Printf("ffmpeg stream copy: %v", copyErr)
		cancel() // stop FFmpeg if it is still running (e.g. blocked on stdout)
	}
	waitErr := cmd.Wait()
	if waitErr == nil {
		return
	}
	bytesSent := bcw.n
	killed := strings.Contains(waitErr.Error(), "killed")
	if errors.Is(ctx.Err(), context.Canceled) {
		if killed && bytesSent < 64*1024 {
			log.Printf("ffmpeg: early client disconnect (%d bytes sent); players often retry the URL — ignore if playback works", bytesSent)
			return
		}
		log.Printf("ffmpeg: stopped after client disconnect (%d bytes sent): %v", bytesSent, waitErr)
		return
	}
	if ctx.Err() != nil {
		log.Printf("ffmpeg stopped: %v (%v)", ctx.Err(), waitErr)
		return
	}
	log.Printf("ffmpeg exit: %v (if signal:killed with no client cancel, check OOM: dmesg/journal) [%d bytes sent]", waitErr, bytesSent)
}
