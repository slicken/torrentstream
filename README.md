# torrentstream.io

Private torrent-to-browser streaming service written in Go.

`torrentstream` searches configured torrent indexers, shows movie metadata when an OMDB key is available, loads a selected torrent, and streams the largest media file directly to the browser. Subtitles included in the torrent are detected and exposed to the player when available.

> This project is intended for private, educational, and research use. Only stream content you have the legal right to access.

![torrentstream screenshot](screenshot_new.png?raw=true "torrentstream")

## Features

- Meta-search across supported torrent sites.
- Browser playback through raw `/stream` range requests.
- Optional FFmpeg transcoding for browser-friendly H.264/AAC output.
- Torrent subtitle discovery for `.srt` and `.vtt` files.
- OMDB metadata support for posters, ratings, and movie details.
- Active playback heartbeat so paused or seeking browser sessions are kept alive.
- Automatic cleanup of idle torrents and temporary files.

## Build

```sh
go build
```

For a static build:

```sh
CGO_ENABLED=0 go build
```

## Configuration

OMDB metadata is optional. Set `OMDB_KEY` before starting the server if you want posters and movie details:

```sh
export OMDB_KEY="your_omdb_api_key"
```

Run the service:

```sh
./torrentstream
```

Then open [http://localhost:8080](http://localhost:8080).

## FFmpeg Mode

Use `-ffmpeg` when the source file is not directly playable by the browser, for example MKV or video codecs the browser cannot decode.

```sh
./torrentstream -ffmpeg
```

FFmpeg mode transcodes the stream to fragmented MP4 with H.264/AAC. This improves browser compatibility but uses more CPU and does not provide the same full seek/scrub behavior as the raw `/stream` endpoint. FFmpeg is resolved from `PATH` by default, or you can provide a path with `-ffmpeg.path`.

## Command Line Options

```text
torrentstream

Private torrent-to-browser streaming service.

Usage:
  torrentstream [options]

Server:
  -http <addr>          HTTP listen address                  default: :8080
  -dir <path>           temporary download directory          default: tmp
  -maximum <n>          maximum active torrents               default: 0 (unlimited)

Torrent:
  -seeders <n>          minimum seeders in search results     default: 1
  -nodes <n>            maximum peer connections per torrent  default: 100
  -trackers             enable tracker peer discovery         default: true
  -seed                 keep seeding after download           default: false

Bandwidth:
  -dl <bytes/sec>       download rate limit                   default: -1 (unlimited)
  -ul <bytes/sec>       upload rate limit                     default: -1 (unlimited)

Search:
  -site <minutes>       torrent-site health check interval    default: 60

Playback:
  -ffmpeg               transcode to browser-friendly MP4     default: false
  -ffmpeg.path <path>   FFmpeg binary path                    default: ffmpeg

Examples:
  torrentstream -http :8080
  torrentstream -ffmpeg -maximum 5
```

## Notes

- Raw `/stream` playback supports browser range requests and is best for files the browser can already decode.
- `-ffmpeg` is a compatibility mode. Use it when raw playback fails because of unsupported codecs or containers.
- Torrents remain active while the player page is open and sending heartbeats. They are cleaned up after the idle timeout once playback activity stops.
- Trackers, DHT, PEX, uTP, IPv6, and WebTorrent support are enabled by default to improve peer discovery.

## Responsible Use

This software is provided for educational and research purposes only. The user is responsible for complying with all local laws and regulations related to torrent usage and content streaming.

This project is not intended to encourage or facilitate copyright infringement. The developers and contributors are not responsible for misuse or illegal activity performed with this software.

## License

This project is provided as-is for private, educational, and research use. See the repository license file for full license terms, if one is included.
