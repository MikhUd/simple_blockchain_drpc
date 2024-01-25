package peer_manager

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/MikhUd/blockchain/internal/config"
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"log/slog"
	"net"
	"storj.io/drpc/drpcconn"
	"sync/atomic"
)

type writer struct {
	writeAddr  string
	nconn      net.Conn
	dconn      *drpcconn.Conn
	stream     bcproto.DRPCPeerManager_ReceiveStream
	serializer Serializer
	state      atomic.Uint32
	tlsConfig  *tls.Config
}

func NewWriter(writeAddr string) Sender {
	return &writer{writeAddr: writeAddr}
}

func (w *writer) Send(ctx *Context) error {
	const op = "writer.Send"
	if w.state.Load() != config.Running {
		err := w.start(w.writeAddr)
		if err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("writer start error: %s", err.Error()))
			return err
		}
	}
	var ser ProtoSerializer
	data, err := ser.Serialize(ctx.Msg())
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("serialize error: %s", err.Error()))
		return err
	}
	msg := &bcproto.BlockchainMessage{Data: data, TypeName: ser.TypeName(ctx.Msg())}
	err = w.stream.Send(msg)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("writer send error: %s", err.Error()))
		return err
	}
	return nil
}

func (w *writer) start(addr string) error {
	const op = "writer.Start"
	//TODO: implement tls
	nconn, err := net.Dial("tcp", addr)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("nconn init error: %s", err.Error()))
		return err
	}
	w.nconn = nconn

	dconn := drpcconn.New(nconn)
	client := bcproto.NewDRPCPeerManagerClient(dconn)
	stream, err := client.Receive(context.Background())
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("dconn creating stream error: %s", err.Error()))
	}
	w.dconn = dconn
	w.stream = stream

	slog.With(slog.String("op", op)).Debug("writer connected", "peer manager", w.writeAddr)

	go func() {
		<-w.dconn.Closed()
		slog.Debug("writer connection closed", "peer manager addr", w.writeAddr)
	}()

	return nil
}
