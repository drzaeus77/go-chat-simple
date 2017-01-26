// Copyright 2017 Brenden Blanco
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

type MsgType int

const (
	LOGIN MsgType = iota
	LOGOUT
	TEXTLINE
)

type Notification struct {
	Type    MsgType
	Msg     string
	Name    string
	ReplyCh chan<- *Notification
}

// Board is an object to handle a single string of messages for a set of
// clients. A Board supports login, logout, and publish operations. No history
// is kept.
type Board struct {
	Name     string
	wakeupCh chan *Notification
	clients  map[string]chan<- *Notification
}

func NewBoard(name string) *Board {
	return &Board{
		Name:     name,
		wakeupCh: make(chan *Notification),
		clients:  make(map[string]chan<- *Notification),
	}
}

// HandleBoard handles and serializes all events for a board. Input and output
// channels serve as the synchronization primitive.
// Never exits.
func (b *Board) HandleBoard() {
	for {
		select {
		case m := <-b.wakeupCh:
			switch m.Type {
			case LOGIN:
				fmt.Printf("login from [%s]\n", m.Name)
				b.clients[m.Name] = m.ReplyCh
			case LOGOUT:
				fmt.Printf("logout from [%s]\n", m.Name)
				delete(b.clients, m.Name)
			case TEXTLINE:
				fmt.Printf("msg from [%s]\n", m.Name)
				for name, ch := range b.clients {
					if name == m.Name {
						continue
					}
					fmt.Printf("  fwd to [%s]\n", name)
					ch <- m
				}
			}
		}
	}
}

// Login adds a user to a board to be notified of messages.
// replyCh - a channel on which a subscribed goroutine will listen for new
// messages.
func (b *Board) Login(name string, replyCh chan<- *Notification) {
	b.wakeupCh <- &Notification{
		Type:    LOGIN,
		Name:    name,
		ReplyCh: replyCh,
	}
}

// Logout removes a user from a board
func (b *Board) Logout(name string) {
	b.wakeupCh <- &Notification{
		Type: LOGOUT,
		Name: name,
	}
}

// Publish sends a message to a board to be published to others
func (b *Board) Publish(name, msg string) {
	b.wakeupCh <- &Notification{
		Type: TEXTLINE,
		Name: name,
		Msg:  msg,
	}
}

// Serve handles the communication for an individual client.
// One additional helper goroutine is created.
func Serve(b *Board, conn net.Conn) {
	// Ensure the handle is freed, regardless of how we exit.
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// login prompt
	if _, err := writer.WriteString("username> "); err != nil {
		return
	}
	if err := writer.Flush(); err != nil {
		return
	}
	name, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	name = strings.TrimSpace(name)

	// Add ourselves to the board to be notified when someone posts a
	// message
	reply := make(chan *Notification)
	b.Login(name, reply)

	// Run a goroutine to read from the client and post to the board.
	// The goroutine will exit when the client closes the conn.
	// NOTE: This doesn't handle closing of boards top-down, only
	// preventing the leaking of sockets upon client logout.  Handling of
	// that case would require another channel for graceful cleanup (see
	// https://blog.golang.org/pipelines)
	go func() {
		defer close(reply)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				b.Logout(name)
				return
			}
			b.Publish(name, line)
		}
	}()

	// Handle publishing of other clients messages back to this goroutines'
	// client.
	for {
		select {
		case r, ok := <-reply:
			if !ok {
				// chan was closed in above goroutine
				return
			}
			_, err := writer.WriteString(fmt.Sprintf("%s: %s", r.Name, r.Msg))
			if err != nil {
				return
			}
			if err := writer.Flush(); err != nil {
				return
			}
		}
	}
}

// Single routine to accept all new connections
func Run() {
	// Only handle a singleton board in this implementation.
	b := NewBoard("1")

	// Each board has its own goroutine for serialization of events.
	go b.HandleBoard()

	listen, err := net.Listen("tcp", ":5001")
	if err != nil {
		panic(fmt.Errorf("net.Listen: %s", err))
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Printf("net.Accept: %s\n", err)
			continue
		}
		go Serve(b, conn)
	}
}
