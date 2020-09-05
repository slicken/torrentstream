# torrentstream.io

This streaming website is for private use and friends only.<br>
<br>
serveing any movie content, shared by others on the torrent networks as a stream.<br>
making meta-search on tpb,kat, plots results (with poster, imdb info and scoring) to stream directly in browser.<br>
adds sub if found in torrent, if not then fetching it from subdb and inserts it.<br>
This streaming website is for private use and friends only.<br>
<br>
<br>
---build---<br>
go build -o app<br>
./run<br>
<br>
---docker---<br>
docker build . -t ts<br>
docker run -p 8080:8080 -p 5000:5000 ts<br>

![Alt text](ts_screen.png?raw=true "torrentstream")
