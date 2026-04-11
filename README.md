# torrentstream.io

This streamer app is for private use only.<br>
<br>
serveing any movie content, shared by others on the torrent networks as a stream.<br>
making meta-search on The Piratebay, Kickass torrents and plots results (with poster, info and score) to stream directly in browser.<br>
adds sub if found in torrent, if not then fetching it from subdb and inserts it.<br>

### build ###
```
CGO_ENABLE=0 go build
export OMDB=your_omdb_key
./torrentstream
```

### `-ffmpeg` ###

**`-ffmpeg`** mode serves **standardized** video (H.264/AAC) so more files play in the browser. It does **not** support **seek / scrub** the way normal **`/stream`** does (no full-length timeline; you mostly watch from the current position). Uses more CPU; FFmpeg is found on `PATH` or via **`FFMPEG_PATH`** / **`-ffmpeg.path`**.

### app args ###
```
$ ./torrentstream --help
Usage of ./torrentstream:
  -dir string
    	directory for temp downloads (default "tmp")
  -dl int
    	max bytes per second (download) (default -1)
  -http string
    	http server address (default ":8080")
  -maximum int
    	maximum active torrents (default 50)
  -nodes int
    	maximum connections per torrent (default 100)
  -seed
    	seed after download
  -seeders int
    	minimum seeders (default 1)
  -site int
    	check torrent sites (minutes) (default 100)
  -ul int
    	max bytes per second (upload) (default -1)
  -ffmpeg
    	standardized browser output; weak seeking (see above)
  -ffmpeg.path string
    	ffmpeg binary if not on PATH
```

![Alt text](screenshot_new.png?raw=true "torrentstream.png")

### License ###
This software is provided for educational and research purposes only. The user is responsible for ensuring compliance with local laws and regulations regarding torrent usage and content streaming.

This project is not intended to encourage or facilitate copyright infringement. Users should only stream content they have the legal right to access.

The developers and contributors of this project are not responsible for any misuse or illegal activities performed using this software.

