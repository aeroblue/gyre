package gyre

import (
	"github.com/armen/gyre/msg"
	zmq "github.com/vaughan0/go-zmq"

	"fmt"
	"time"
)

var (
	peerEvasive  = 5 * time.Second  // Five seconds' silence is evasive
	peerExpired  = 10 * time.Second // Ten seconds' silence is expired
	reapInterval = 1 * time.Second  //  Once per second
)

type peer struct {
	mailbox      *zmq.Socket // Socket through to peer
	identity     string
	endpoint     string            // Endpoint connected to
	name         string            // Peer's public name
	evasiveAt    time.Time         // Peer is being evasive
	expiredAt    time.Time         // Peer has expired by now
	connected    bool              // Peer will send messages
	ready        bool              // Peer has said Hello to us
	status       byte              // Our status counter
	sentSequence uint16            // Outgoing message sequence
	wantSequence uint16            // Incoming message sequence
	headers      map[string]string // Peer headers
}

// newPeer creates a new peer
func newPeer(identity string) (p *peer) {
	p = &peer{
		identity: identity,
		headers:  make(map[string]string),
	}
	p.refresh()
	return
}

// destroy disconnects peer mailbox. No more messages will be sent to peer until connected again
func (p *peer) destroy() {
	p.disconnect()
	for k := range p.headers {
		delete(p.headers, k)
	}
}

// connect configures mailbox and connects to peer's router endpoint
func (p *peer) connect(from, endpoint string) (err error) {
	// Create new outgoing socket (drop any messages in transit)
	p.mailbox, err = zmq.NewSocket(zmq.Dealer)
	if err != nil {
		return err
	}

	// Set our own identity on the socket so that receiving node
	// knows who each message came from. Note that we cannot use
	// the UUID directly as the identity since it may contain a
	// zero byte at the start, which libzmq does not like for
	// historical and arguably bogus reasons that it nonetheless
	// enforces.
	routingId := append([]byte{1}, []byte(from)...)
	p.mailbox.SetIdentitiy(routingId)

	// Set a high-water mark that allows for reasonable activity
	// p.mailbox.SetSendHWM(uint64(peerExpired * time.Microsecond))

	// Send messages immediately or return EAGAIN
	p.mailbox.SetSendTimeout(0)

	// Connect through to peer node
	err = p.mailbox.Connect(fmt.Sprintf("tcp://%s", endpoint))
	if err != nil {
		return err
	}
	p.endpoint = endpoint
	p.connected = true
	p.ready = false

	return nil
}

// disconnects peer mailbox. No more messages will be sent to peer until connected again
func (p *peer) disconnect() {
	if p.connected {
		if p.mailbox != nil {
			p.mailbox.Disconnect(p.endpoint)
			p.mailbox.Close()
			p.mailbox = nil
		}
		p.endpoint = ""
		p.connected = false
		p.ready = false
	}
}

// send sends message to peer
func (p *peer) send(t msg.Transit) (err error) {
	if p.connected {
		p.sentSequence++
		t.SetSequence(p.sentSequence)
		err = t.Send(p.mailbox)
		if err != nil {
			p.disconnect()
		}
	}

	return
}

// refresh refreshes activity at peer
func (p *peer) refresh() {
	p.evasiveAt = time.Now().Add(peerEvasive)
	p.expiredAt = time.Now().Add(peerExpired)
}

// checkMessage checks peer message sequence
func (p *peer) checkMessage(t msg.Transit) bool {
	p.wantSequence++
	valid := p.wantSequence == t.Sequence()
	if !valid {
		p.wantSequence--
	}

	return valid
}

// setName sets name.
func (p *peer) setName(name string) {
	p.name = name
}

// Returns a header in headers map
func (p *peer) Header(key string) (value string, ok bool) {
	value, ok = p.headers[key]
	return
}

func (p *peer) Headers() map[string]string {
	return p.headers
}

// Returns identity (uuid) of the peer
func (p *peer) Identity() string {
	return p.identity
}

// SetExpired sets expired.
func SetExpired(expired time.Duration) {
	peerExpired = expired
}

// SetEvasive sets evasive.
func SetEvasive(evasive time.Duration) {
	peerEvasive = evasive
}

// SetPingInterval sets interval of pinging other peers
func SetPingInterval(interval time.Duration) {
	reapInterval = interval
}