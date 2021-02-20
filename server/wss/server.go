package wss

import (
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/liupeidong0620/hummingbird-server/server"
	"github.com/liupeidong0620/hummingbird/log"
	"github.com/liupeidong0620/hummingbird/module/wss/wssconn"
)

const (
	upStream = iota
	downStream
)

const (
	ProtocolTCP = "tcp"
	ProtocolDNS = "dns"
	ProtocolUDP = "udp"

	HeaderProtocol           = "Protocol"
	HeaderDestinationAddress = "Destination-Address"
	HeaderDestinationPort    = "Destination-Port"
	HeaderScheme             = "Scheme"
)

func upgradeError(w http.ResponseWriter, r *http.Request, status int, reason error) {
	w.Header().Set("Sec-Websocket-Version", "13")
	http.Error(w, http.StatusText(status), status)
}

var upgrader = websocket.Upgrader{
	Error: upgradeError,
}

type userInfo struct {
	protocol string
	dstIp    string
	dstPort  string
	scheme   string
}

type stat struct {
	up   uint64
	dows uint64
}

type connCfg struct {
	// ToDo
}

type context struct {
	user *userInfo
	stat stat

	cfg connCfg

	local  net.Conn
	remote net.Conn
}

type WssServer struct {
	http.Server

	base *server.Base
}

func NewWssServer(base *server.Base) (*WssServer, error) {

	ws := new(WssServer)
	ws.base = base
	ws.Handler = ws

	return ws, nil
}

func (ws *WssServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Info("ws request ", r.RemoteAddr, " ----------- ", r.Host)
	user := ws.parseUserInfo(w, r)
	if user == nil {
		return
	}
	log.Info("ws request user info:", *user)

	ctx := new(context)
	ctx.user = user

	ctx.process(w, r)

	defer ctx.stop()

	log.Info("proxy scheme: %s, dial %s, remote addr: %s:%s", ctx.user.scheme,
		ctx.user.protocol, ctx.user.dstIp, ctx.user.dstPort)
}

func (ws *WssServer) parseUserInfo(w http.ResponseWriter, r *http.Request) *userInfo {
	user := &userInfo{}

	protocol := r.Header.Get(HeaderProtocol)
	switch protocol {
	case ProtocolTCP, ProtocolUDP:
	default:
		log.Error("protocol error", protocol, "return:", http.StatusText(http.StatusBadRequest))
		upgrader.Error(w, r, http.StatusBadRequest, nil) // HTTP 400
		return nil
	}

	dstIp, dstPort := r.Header.Get(HeaderDestinationAddress), r.Header.Get(HeaderDestinationPort)

	switch protocol {
	case ProtocolTCP, ProtocolUDP:
		if dstIp == "" || dstPort == "" {
			upgrader.Error(w, r, http.StatusBadRequest, nil) // HTTP 400
			return nil
		}
	}

	scheme := r.Header.Get(HeaderScheme)

	user.dstIp = dstIp
	user.dstPort = dstPort
	user.protocol = protocol
	user.scheme = scheme

	return user
}

func (ws *context) httpToWsConn(w http.ResponseWriter, r *http.Request) error {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	ws.local = wssconn.WSConn(wsConn)

	return nil
}

func (ws *context) process(w http.ResponseWriter, r *http.Request) {

	if ws.user.scheme == ProtocolDNS {
		// 8.8.8.8 53
		ws.user.dstIp = "8.8.8.8"
		ws.user.dstPort = "53"
	}
	log.Info("dial protocol ", ws.user.protocol, " addr: ", ws.user.dstIp+":"+ws.user.dstPort, "start ...")
	rconn, err := net.DialTimeout(ws.user.protocol, ws.user.dstIp+":"+ws.user.dstPort, time.Second*10)
	if err != nil {
		upgrader.Error(w, r, http.StatusServiceUnavailable, nil) // HTTP 503
		return
	}
	log.Info("dial ok.")

	ws.remote = rconn

	log.Info("net conn to ws conn start ...")
	err = ws.httpToWsConn(w, r)
	if err != nil {
		return
	}
	log.Info("net conn to ws conn ok")

	// process
	ch := make(chan uint64)

	go func() {
		n := pipeThenClose(ws.remote, ws.local, time.Second*10, upStream)
		//log.Trace("UP END")
		ch <- n
	}()
	ws.stat.dows = pipeThenClose(ws.local, ws.remote, time.Second*10, downStream)
	//log.Trace("DOWN END")
	ws.stat.up = <-ch
	close(ch)
}

func (ws *context) stop() {
	if ws.local != nil {
		ws.local.Close()
	}
	if ws.remote != nil {
		ws.remote.Close()
	}
}

func (ws *WssServer) Start() error {
	log.Info("wss server stat ...")
	go func() {
		if ws.base.CertFile != "" && ws.base.KeyFile != "" {
			ws.ServeTLS(ws.base.Listen, ws.base.CertFile, ws.base.KeyFile)
		} else {
			ws.Serve(ws.base.Listen)
		}
	}()
	log.Info("wss Server listen ", ws.base.Listen.Addr().String(), " ok.")

	return nil
}

func (ws *WssServer) Stop() error {
	var err error

	err = ws.Server.Close()
	if err != nil {
		return err
	}
	err = ws.base.Stop()
	log.Info("wss Server close.")

	return err
}

func pipeThenClose(dst, src net.Conn, waitTime time.Duration, pipeType int) (n uint64) {
	defer dst.Close()

	var prefixW string
	var prefixR string

	switch pipeType {
	case upStream:
		prefixW = "[conn.dsterr]"
		prefixR = "[conn.wserr]"
	case downStream:
		prefixW = "[conn.wserr]"
		prefixR = "[conn.dsterr]"
	}

	buf := make([]byte, 1460)

	for {
		if waitTime > 0 {
			src.SetReadDeadline(time.Now().Add(waitTime))
		}
		nr, err := src.Read(buf)

		if nr > 0 {
			if nw, err := dst.Write(buf[:nr]); err != nil {
				//if (err != io.EOF) && (!websocket.IsCloseError(err, websocket.CloseNormalClosure)) {
				log.Error(prefixW, "write:", err)
				break
			} else {
				n += uint64(nw)
			}
		}

		if err != nil {
			//if err != io.EOF && (!websocket.IsCloseError(err, websocket.CloseNormalClosure)) {
			log.Error(prefixR, "read:", err)
			break
		}
	}

	return
}
