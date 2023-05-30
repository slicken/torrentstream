# torrentstream.io

This streaming website is for private use only.<br>
<br>
serveing any movie content, shared by others on the torrent networks as a stream.<br>
making meta-search on The Piratebay, Kickass torrents and plots results (with poster, info and score) to stream directly in browser.<br>
adds sub if found in torrent, if not then fetching it from subdb and inserts it.<br>
This streaming website is for private use only.<br>

### build ###
```
go build -o app
./run
```
### docker ###
```
docker build . -t ts
docker run -p 8080:8080 -p 5000:5000 ts
```
### app args ###
```
slicken@slk:~$ ./app --help
Usage of ./app:
  -dir string
    	directory for temp downloads (default "tmp")
  -dl int
    	max bytes per second (download) (default -1)
  -http string
    	http server address (default ":8080")
  -idle duration
    	idle time before closing (default 15m0s)
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

![Alt text](ts_screen.png?raw=true "torrentstream")
