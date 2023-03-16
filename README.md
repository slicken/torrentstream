# torrentstream.io

This streaming website is for private use and friends only.<br>
<br>
serveing any movie content, shared by others on the torrent networks as a stream.<br>
making meta-search on tpb,kat, plots results (with poster, imdb info and scoring) to stream directly in browser.<br>
adds sub if found in torrent, if not then fetching it from subdb and inserts it.<br>
This streaming website is for private use and friends only.<br>

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
    	torrents directory for temp downloads (default "torrents")
  -dl int
    	max bytes per second for download (default -1)
  -http string
    	http server address (default ":8080")
  -idle duration
    	maximum torrent idle in seconds (default 900s)
  -maximum int
    	maximum active torrents (default 50)
  -node int
    	maximum node connections per torrent (default 100)
  -seed
    	seed after download is complete
  -seeders int
    	minimum seeders for torrent to show as a result (default 5)
  -site int
    	site handler check in minutes minutes (default 20)
  -ul int
    	max bytes per second for upload (default -1)
```

![Alt text](ts_screen.png?raw=true "torrentstream")
