# torrentstream.io

This streaming website is for private use only.<br>
<br>
serveing any movie content, shared by others on the torrent networks as a stream.<br>
making meta-search on The Piratebay, Kickass torrents and plots results (with poster, info and score) to stream directly in browser.<br>
adds sub if found in torrent, if not then fetching it from subdb and inserts it.<br>
This streaming website is for private use only.<br>

### build ###
```
CGO_ENABLE=0 go build
export OMDB=your_omdb_key
./torrentstream
```
### docker ###
```
docker build . -t torrentstream
docker run -p 8080:8080 -p 5000:5000 torrentstream
```
### systemd service ###
For automatic restart on failure, install as a systemd service:

1. Copy the service file:
```bash
sudo cp torrentstream.service /etc/systemd/system/
```

2. Edit the service file to set your:
- Username (replace %i)
- Working directory
- OMDB API key

3. Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable torrentstream
sudo systemctl start torrentstream
```

4. Check status and logs:
```bash
sudo systemctl status torrentstream
sudo journalctl -u torrentstream -f
```

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
```

![Alt text](screenshot_new.png?raw=true "torrentstream.png")

### License ###
This software is provided for educational and research purposes only. The user is responsible for ensuring compliance with local laws and regulations regarding torrent usage and content streaming.

This project is not intended to encourage or facilitate copyright infringement. Users should only stream content they have the legal right to access.

The developers and contributors of this project are not responsible for any misuse or illegal activities performed using this software.

