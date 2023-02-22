package peertracker

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"syscall"
)

func CallerFromUDSConn(conn net.Conn) (CallerInfo, error) {
	var info CallerInfo

	sysconn, ok := conn.(syscall.Conn)
	if !ok {
		return info, ErrInvalidConnection
	}

	rawconn, err := sysconn.SyscallConn()
	if err != nil {
		return info, err
	}

	ctrlErr := rawconn.Control(func(fd uintptr) {
		info, err = getCallerInfoFromFileDescriptor(fd)
	})
	if ctrlErr != nil {
		return info, ctrlErr
	}
	if err != nil {
		return info, err
	}

	info.Addr = conn.RemoteAddr()
	return info, nil
}

func receiveTCPConn(conn net.Conn) (CallerInfo, error) {
	var info CallerInfo
	tcpCon, ok := conn.(*net.TCPConn)
	if !ok {
		return info, ErrInvalidConnection
	}
	defer func() {
		_ = tcpCon.Close()
	}()

	buf := make([]byte, 4096)
	n, err := tcpCon.Read(buf)
	if err != nil {
		return info, ErrInvalidConnection
	}

	payload := buf[:n]
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(payload)))
	if err != nil {
		return info, ErrInvalidConnection
	}
	fmt.Printf("req: %v\n", req)
	return info, nil
}
