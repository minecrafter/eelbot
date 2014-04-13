eelbot
======

Minimalist "snake" bots for testing Minecraft servers. Program works as a proxy. When you are joining, the proxy connects specified amount of bots to server. You are controlling all of them at once in standard Minecraft client.

Warning! Eelbot is in developent and it could be unstable.

Compiling
---------

1. Download and install Go from http://code.google.com/p/go/downloads/list
2. Set up your Go environment
3. Clone project to `$GOPATH/src/github.com/Teapot418/eelbot`
4. Run `go build` in eelbot folder

Usage
-----

```
./eelbot [options...]
```

Options:
* count - count of bots
* eeld - eel timeout
* errd - timeout in millis if client was kicked while connecting
* joind - timeout between bot joins in millis
* proxy - proxy address with port
* target - server address with port

Example: 30 bots, timeout between bots (snake effect) = 200, proxy on `0.0.0.0:25588` and target server on `127.0.0.1:25565`

```
./eelbot -count=30 -eeld=200 -proxy=0.0.0.0:25588 -target=127.0.0.1:25565
```

Then join from Minecraft client to `0.0.0.0:25588`. Eelbot will connect bots to target server.

Todo
----

* fix lags, when there are a lot of bots
* entity id translator
* status support for proxy
* changing nicknames?
