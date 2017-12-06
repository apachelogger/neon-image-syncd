/*
	Copyright 2017 Harald Sitter <sitter@kde.org>

	This program is free software; you can redistribute it and/or
	modify it under the terms of the GNU General Public License as
	published by the Free Software Foundation; either version 3 of
	the License or any later version accepted by the membership of
	KDE e.V. (or its successor approved by the membership of KDE
	e.V.), which shall act as a proxy defined in Section 14 of
	version 3 of the license.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/coreos/go-systemd/activation"
	"github.com/gin-gonic/gin"

	"net/http"
	_ "net/http/pprof"
)

var rsyncMutex sync.Mutex

// Event is a helper struct to queue up events for streaming.
type Event struct {
	name string
	data string
}

/**
 * @api {get} /sync RSync neon images
 *
 * @apiVersion 1.0.0
 * @apiGroup Sync
 * @apiName v1Sync
 *
 * @apiDescription Queues an RSync run. Make sure that your client side timeouts
 *   are long enough. The request may take a while to complete.
 *   Once the request has started processing a text/event-stream response is
 *   started. Note that due to the nature of this request the response is
 *   always code 200. Actual error information is communicated through an
 *   error event at the end of the stream.
 *
 * @apiSuccessExample {event-stream} Success-Response:
 *   < HTTP/1.1 200 OK
 *   < Cache-Control: no-cache
 *   < Content-Type: text/event-stream
 *   < Date: Mon, 04 Dec 2017 11:58:32 GMT
 *   < Transfer-Encoding: chunked
 *   <
 *   event:stdout
 *   data:total 64K
 *
 *   event:stdout
 *   data:drwxrwxr-x 1 me me  122 Dez  4 11:55 .
 *
 *   event:error
 *   data:
 *
 * @apiErrorExample {event-stream} Error-Response:
 *   < HTTP/1.1 200 OK
 *   < Cache-Control: no-cache
 *   < Content-Type: text/event-stream
 *   < Date: Mon, 04 Dec 2017 12:02:10 GMT
 *   < Transfer-Encoding: chunked
 *   <
 *   event:stdout
 *   data:hi there
 *
 *   event:stderr
 *   data:error
 *
 *   event:error
 *   data:exit status 1
 */
func v1Sync(c *gin.Context) {
	rsyncMutex.Lock()
	defer rsyncMutex.Unlock()

	// This possibly would benefit from loading form a config or something.
	// Origin and target path at least.
	// TODO: we quite possibly can drop the chown as the files should 664 by
	// default, which is good enough for the server to READ them.
	// cmd := exec.Command("ls", "-lah")
	cmd := exec.Command("/usr/bin/rsync",
		"-rlptv", "--info=progress", "--delete",
		"rsync://racnoss.kde.org/applicationdata/neon",
		"/mnt/volume-do-cacher-storage/files.kde.org/")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	events := make(chan Event)
	var wg sync.WaitGroup

	// Scan stdout and stderr and send their chunks as server side events.
	// Once both finished scanning we return.

	wg.Add(1)
	go func() {
		s := bufio.NewScanner(stdout)
		for s.Scan() {
			events <- Event{"stdout", s.Text()}
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			events <- Event{"stderr", s.Text()}
		}
		wg.Done()
	}()

	go func() {
		err = cmd.Start()
		if err != nil {
			events <- Event{"error", func() string {
				if err != nil {
					return err.Error()
				}
				return ""
			}()}
			panic(err)
		}

		// Wait for all IO to be done, lest we want to crash on writing to
		// a closed channel.
		wg.Wait()

		// Retrieve exit error and close the events channel
		err = cmd.Wait()
		events <- Event{"error", func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}()}

		close(events)
	}()

	// The funcs above feeds us events, we only need to stream them.
	// The channel is closed when the command returned and the error event is
	// pushed.
	c.Stream(func(w io.Writer) bool {
		if event, ok := <-events; ok {
			c.SSEvent(event.name, event.data)
			return true
		}
		return false
	})
}

func main() {
	flag.Parse()

	router := gin.Default()
	router.GET("/v1/sync", v1Sync)

	fmt.Println("starting servers on sockets")
	listeners, err := activation.Listeners(true)
	if err != nil {
		panic(err)
	}

	var servers []*http.Server
	for _, listener := range listeners {
		server := &http.Server{Handler: router}
		go server.Serve(listener)
		servers = append(servers, server)
	}

	if len(servers) <= 0 {
		fmt.Println("no sockets provided, listening on iface")
		port := os.Getenv("PORT")
		if len(port) <= 0 {
			port = "8080"
		}

		host := os.Getenv("HOST")
		if len(host) <= 0 {
			host = "localhost"
		}

		router.Run(host + ":" + port)
	}

	select {}
}
