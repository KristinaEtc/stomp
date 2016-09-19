package server

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-stomp/stomp/frame"
	"github.com/go-stomp/stomp/server/client"
	"github.com/go-stomp/stomp/server/queue"
	"github.com/go-stomp/stomp/server/status"
	"github.com/go-stomp/stomp/server/topic"
)

//var log slf.StructuredLogger

//func init() {
//log = slf.WithContext("processor")
//}

type requestProcessor struct {
	server      *Server
	ch          chan client.Request
	tm          *topic.Manager
	qm          *queue.Manager
	connections map[int64]*client.Conn
	stop        bool // has stop been requested
}

func newRequestProcessor(server *Server) *requestProcessor {
	proc := &requestProcessor{
		server:      server,
		ch:          make(chan client.Request, 128),
		tm:          topic.NewManager(),
		connections: make(map[int64]*client.Conn),
	}

	if server.QueueStorage == nil {
		proc.qm = queue.NewManager(queue.NewMemoryQueueStorage())
	} else {
		proc.qm = queue.NewManager(server.QueueStorage)
	}

	return proc
}

func (proc *requestProcessor) createStatus() status.ServerStatus {
	//clients
	clients := make([]status.ServerClientStatus, 0)
	for _, conn := range proc.connections {
		clients = append(clients, conn.GetStatus())
	}

	//
	queues := proc.qm.GetStatus()
	//
	topics := proc.tm.GetStatus()

	hostname, _ := os.Hostname()

	return status.ServerStatus{
		Clients:      clients,
		Queues:       queues,
		Topics:       topics,
		Time:         time.Now().Format("2006-01-02T15:04:05"),
		Type:         "status",
		Id:           proc.server.Id(),
		Name:         proc.server.Name(),
		Version:      proc.server.Version(),
		Subtype:      "server",
		Subsystem:    "processor",
		ComputerName: hostname,
		UserName:     fmt.Sprintf("%s", os.Getuid()),
		ProcessName:  os.Args[0],
		Pid:          os.Getpid(),
		Severity:     20,
	}
}

func (proc *requestProcessor) createStatusFrame() *frame.Frame {
	f := frame.New("MESSAGE", frame.ContentType, "application/json")
	status := proc.createStatus()
	//log.Debugf("status %v", status)
	//bytes, err := json.MarshalIndent(status, "", "  ")
	bytes, err := json.Marshal(status)
	//log.Debugf("createStatusFrame %v", string(bytes))
	if err != nil {
		f.Body = []byte(fmt.Sprintf("error %v\n", err))
	} else {
		f.Body = bytes
	}
	return f
}

func (proc *requestProcessor) sendStatusFrame() {
	topic := proc.tm.Find("/topic/go-stomp.status")
	f := proc.createStatusFrame()
	f.Header.Add(frame.Destination, "/topic/go-stomp.status")
	//log.Debugf("status frame %v", f.Dump())
	topic.Enqueue(f)
}

func (proc *requestProcessor) Serve(l net.Listener) error {
	go proc.Listen(l)

	ticker := time.NewTicker(5 * time.Second)

	for {
		select {
		case _ = <-ticker.C:
			proc.sendStatusFrame()
		case r := <-proc.ch:
			switch r.Op {
			case client.SubscribeOp:
				if isQueueDestination(r.Sub.Destination()) {
					queue := proc.qm.Find(r.Sub.Destination())
					// todo error handling
					queue.Subscribe(r.Sub)
				} else {
					topic := proc.tm.Find(r.Sub.Destination())
					topic.Subscribe(r.Sub)
				}

			case client.UnsubscribeOp:
				if isQueueDestination(r.Sub.Destination()) {
					queue := proc.qm.Find(r.Sub.Destination())
					// todo error handling
					queue.Unsubscribe(r.Sub)
				} else {
					topic := proc.tm.Find(r.Sub.Destination())
					topic.Unsubscribe(r.Sub)
				}

			case client.EnqueueOp:
				destination, ok := r.Frame.Header.Contains(frame.Destination)
				if !ok {
					// should not happen, already checked in lower layer
					panic("missing destination")
				}

				if isQueueDestination(destination) {
					queue := proc.qm.Find(destination)
					queue.Enqueue(r.Frame)
				} else {
					topic := proc.tm.Find(destination)
					topic.Enqueue(r.Frame)
				}

			case client.RequeueOp:
				destination, ok := r.Frame.Header.Contains(frame.Destination)
				if !ok {
					// should not happen, already checked in lower layer
					panic("missing destination")
				}

				// only requeue to queues, should never happen for topics
				if isQueueDestination(destination) {
					queue := proc.qm.Find(destination)
					queue.Requeue(r.Frame)
				}
			case client.DisconnectedOp:
				delete(proc.connections, r.Conn.Id())
			}
		}
	}
	// this is no longer required for go 1.1
	panic("not reached")
}

func isQueueDestination(dest string) bool {
	return strings.HasPrefix(dest, QueuePrefix)
}

func (proc *requestProcessor) Listen(l net.Listener) {
	var conn_id int64 = 0
	config := newConfig(proc.server)
	timeout := time.Duration(0) // how long to sleep on accept failure
	for {
		rw, err := l.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				if timeout == 0 {
					timeout = 5 * time.Millisecond
				} else {
					timeout *= 2
				}
				if max := 5 * time.Second; timeout > max {
					timeout = max
				}
				log.Errorf("stomp: Accept error: %v; retrying in %v", err, timeout)
				time.Sleep(timeout)
				continue
			}
			return
		}
		timeout = 0
		// TODO: need to pass Server to connection so it has access to
		// configuration parameters.
		conn := client.NewConn(config, rw, proc.ch, conn_id)
		proc.connections[conn_id] = conn
		conn_id++
	}
	// This is no longer required for go 1.1
	log.Panic("not reached")
}

type config struct {
	server *Server
}

func newConfig(s *Server) *config {
	return &config{server: s}
}

func (c *config) HeartBeat() time.Duration {
	if c.server.HeartBeat == time.Duration(0) {
		return DefaultHeartBeat
	}
	return c.server.HeartBeat
}

func (c *config) Authenticate(login, passcode string) bool {
	if c.server.Authenticator != nil {
		return c.server.Authenticator.Authenticate(login, passcode)
	}

	// no authentication defined
	return true
}
