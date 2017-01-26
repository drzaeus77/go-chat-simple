== Overview ==
This project is created as a reference for how to build an extremely simple
chat server in golang. The single package within implements an IRC style (no
scrollback) message board, with just a single chatroom (so far).

The server builds on low-level primitives (posix sockets) for its message
delivery, and go channels for the synchronization primitive.

== Using it ==
Start the server:
```
go install github.com/drzaeus77/go-chat-simple/chat-daemon
$GOPATH/bin/chat-daemon
```

Connect a client:
```
nc localhost 5001
username> bob
```

Connect another:
```
nc localhost 5001
username> alice
Hello, everyone!
```

```
nc localhost 5001
username> bob
alice: Hello, everyone!
```

== Scalability ==
Tested up to 4 clients so far :)

== Todo ==
* Unit testing
* Multiple boards
