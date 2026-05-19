//go:build windows

package peertracker

import (
	"net"

	"github.com/Microsoft/go-winio"
)

type ListenerFactoryOS struct {
	NewPipeListener func(pipe string, pipeConfig *winio.PipeConfig) (net.Listener, error)
	NewTCPListener  func(network string, laddr *net.TCPAddr) (*net.TCPListener, error)
}

func (lf *ListenerFactory) ListenPipe(pipe string, pipeConfig *winio.PipeConfig) (*Listener, error) {
	if lf.NewPipeListener == nil {
		lf.NewPipeListener = winio.ListenPipe
	}
	if lf.NewTracker == nil {
		lf.NewTracker = NewTracker
	}
	if lf.Log == nil {
		lf.Log = newNoopLogger()
	}
	return lf.listenPipe(pipe, pipeConfig)
}

func (lf *ListenerFactory) listenPipe(pipe string, pipeConfig *winio.PipeConfig) (*Listener, error) {
	l, err := lf.NewPipeListener(pipe, pipeConfig)
	if err != nil {
		return nil, err
	}

	tracker, err := lf.NewTracker(lf.Log)
	if err != nil {
		l.Close()
		return nil, err
	}

	return &Listener{
		l:       l,
		Tracker: tracker,
		log:     lf.Log,
	}, nil
}

func (lf *ListenerFactory) ListenTCP(
	network string,
	laddr *net.TCPAddr,
) (*Listener, error) {

	if lf.NewTCPListener == nil {
		lf.NewTCPListener = net.ListenTCP
	}

	if lf.NewTracker == nil {
		lf.NewTracker = NewTracker
	}

	if lf.Log == nil {
		lf.Log = newNoopLogger()
	}

	return lf.listenTCP(network, laddr)
}

func (lf *ListenerFactory) listenTCP(
	network string,
	laddr *net.TCPAddr,
) (*Listener, error) {

	l, err := lf.NewTCPListener(network, laddr)
	if err != nil {
		return nil, err
	}

	tracker, err := lf.NewTracker(lf.Log)
	if err != nil {
		l.Close()
		return nil, err
	}

	return &Listener{
		l:       l,
		Tracker: tracker,
		log:     lf.Log,
	}, nil
}
