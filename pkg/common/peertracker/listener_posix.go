//go:build !windows
// +build !windows

package peertracker

import "net"

type ListenerFactoryOS struct {
	NewUnixListener func(network string, laddr *net.UnixAddr) (*net.UnixListener, error)
	NewTCPListener  func(network string, laddr *net.TCPAddr) (*net.TCPListener, error)
}

func (lf *ListenerFactory) ListenUnix(network string, laddr *net.UnixAddr) (*Listener, error) {
	if lf.NewUnixListener == nil {
		lf.NewUnixListener = net.ListenUnix
	}
	if lf.NewTracker == nil {
		lf.NewTracker = NewTracker
	}
	if lf.Log == nil {
		lf.Log = newNoopLogger()
	}
	return lf.listenUnix(network, laddr)
}

func (lf *ListenerFactory) listenUnix(network string, laddr *net.UnixAddr) (*Listener, error) {
	l, err := lf.NewUnixListener(network, laddr)
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

func (lf *ListenerFactory) ListenTCP(network string, laddr *net.TCPAddr) (*Listener, error) {
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

func (lf *ListenerFactory) listenTCP(network string, laddr *net.TCPAddr) (*Listener, error) {
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
