package zmq

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/oursky/ourd/oddb"
	odplugin "github.com/oursky/ourd/plugin"
	"github.com/oursky/ourd/plugin/common"
	"github.com/zeromq/goczmq"
)

type zmqTransport struct {
	name   string
	iaddr  string // the internal addr used by goroutines to make request to plugin
	eaddr  string // the addr exposed for plugin to connect to with REP.
	broker *Broker
}

type request struct {
	Kind  string
	Name  string
	Param interface{}
}

// type-safe constructors for request.Param assignment

func newLambdaRequest(name string, args json.RawMessage) *request {
	return &request{Kind: "op", Name: name, Param: args}
}

func newHandlerRequest(name string, input json.RawMessage) *request {
	return &request{Kind: "handler", Name: name, Param: input}
}

func newHookRequest(trigger string, record *oddb.Record) *request {
	return &request{Kind: "hook", Name: trigger, Param: (*common.JSONRecord)(record)}
}

// TODO(limouren): reduce copying of this method
func (req *request) MarshalJSON() ([]byte, error) {
	if rawParam, ok := req.Param.(json.RawMessage); ok {
		rawParamReq := struct {
			Kind  string          `json:"kind"`
			Name  string          `json:"name,omitempty"`
			Param json.RawMessage `json:"param,omitempty"`
		}{req.Kind, req.Name, rawParam}
		return json.Marshal(&rawParamReq)
	}

	paramReq := struct {
		Kind  string      `json:"kind"`
		Name  string      `json:"name,omitempty"`
		Param interface{} `json:"param,omitempty"`
	}{req.Kind, req.Name, req.Param}

	return json.Marshal(&paramReq)
}

func (p zmqTransport) RunInit() (out []byte, err error) {
	req := request{Kind: "init"}
	out, err = p.ipc(&req)
	return
}

func (p zmqTransport) RunLambda(name string, in []byte) (out []byte, err error) {
	out, err = p.rpc(newLambdaRequest(name, in))
	return
}

func (p zmqTransport) RunHandler(name string, in []byte) (out []byte, err error) {
	out, err = p.rpc(newHandlerRequest(name, in))
	return
}

func (p zmqTransport) RunHook(recordType string, trigger string, record *oddb.Record) (*oddb.Record, error) {
	out, err := p.rpc(newHookRequest(trigger, record))
	if err != nil {
		return nil, err
	}

	var recordout oddb.Record
	if err := json.Unmarshal(out, (*common.JSONRecord)(&recordout)); err != nil {
		log.WithField("data", string(out)).Error("failed to unmarshal record")
		return nil, fmt.Errorf("failed to unmarshal record: %v", err)
	}
	recordout.OwnerID = record.OwnerID

	return &recordout, nil
}

func (p zmqTransport) RunTimer(name string, in []byte) (out []byte, err error) {
	req := request{Kind: "timer", Name: name}
	out, err = p.rpc(&req)
	return
}

func (p zmqTransport) RunProvider(request *odplugin.AuthRequest) (*odplugin.AuthResponse, error) {
	panic("Thou shalt not call me")
}

func (p *zmqTransport) rpc(req *request) (out []byte, err error) {
	var rawResp []byte

	rawResp, err = p.ipc(req)
	if err != nil {
		return
	}

	var resp struct {
		Result json.RawMessage   `json:"result"`
		Err    *common.ExecError `json:"error"`
	}

	if err = json.Unmarshal(rawResp, &resp); err != nil {
		return
	}
	if resp.Err != nil {
		err = resp.Err
		return
	}

	out = resp.Result
	return
}

func (p *zmqTransport) ipc(req *request) (out []byte, err error) {
	var (
		in      []byte
		reqSock *goczmq.Sock
	)

	in, err = json.Marshal(req)
	if err != nil {
		return
	}

	reqSock, err = goczmq.NewReq(p.iaddr)
	if err != nil {
		return
	}
	err = reqSock.SendMessage([][]byte{in})
	if err != nil {
		return
	}

	msg, err := reqSock.RecvMessage()
	if err != nil {
		return
	}

	if len(msg) != 1 {
		err = fmt.Errorf("malformed resp msg = %s", msg)
	} else {
		out = msg[0]
	}

	return
}

type zmqTransportFactory struct {
}

func (f zmqTransportFactory) Open(name string, args []string) (transport odplugin.Transport) {
	const internalAddrFmt = `ipc://%s.ipc`

	internalAddr := fmt.Sprintf(internalAddrFmt, name)
	externalAddr := args[0]

	broker, err := NewBroker(internalAddr, externalAddr)
	if err != nil {
		log.Panicf("Failed to init broker for zmq transport: %v", err)
	}

	p := zmqTransport{
		name:   name,
		iaddr:  internalAddr,
		eaddr:  externalAddr,
		broker: broker,
	}

	go func() {
		log.Infof("Running zmq broker:\niaddr = %s\neaddr = %s", internalAddr, externalAddr)
		broker.Run()
	}()

	return p
}

func init() {
	odplugin.RegisterTransport("zmq", zmqTransportFactory{})
}
