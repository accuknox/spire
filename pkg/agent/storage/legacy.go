package storage

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/accuknox/spire/pkg/common/diskutil"
	"github.com/accuknox/spire/pkg/common/util"
)

func loadLegacyBundle(dir string) ([]*x509.Certificate, time.Time, error) {
	data, mtime, err := readFile(legacyBundlePath(dir))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to read legacy bundle: %w", err)
	}

	bundle, err := x509.ParseCertificates(data)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse legacy bundle: %w", err)
	}
	return bundle, mtime, nil
}
func loadLegacyBundleFromK8S(namespace, secretname string) ([]*x509.Certificate, time.Time, error) {
	store, tm, err := loadDataFromK8S(namespace, secretname)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, err
	}
	return store.Bundle, tm, nil
}

func storeLegacyBundle(dir string, bundle []*x509.Certificate) error {
	data := new(bytes.Buffer)
	for _, cert := range bundle {
		data.Write(cert.Raw)
	}
	if err := diskutil.AtomicWritePrivateFile(legacyBundlePath(dir), data.Bytes()); err != nil {
		return fmt.Errorf("failed to store legacy bundle: %w", err)
	}
	return nil
}
func storeLegacyBundleToK8S(namespace, secret string, bundle []*x509.Certificate) error {
	mapData := make(map[string][]byte)

	for i, cert := range bundle {
		block := pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
		pemBytes := pem.EncodeToMemory(&block)
		if isUnique(mapData, pemBytes) {
			key := fmt.Sprintf("bundle-legacy-%d", i)
			mapData[key] = pemBytes
		}
	}

	return util.CreateK8sSecrets(namespace, secret, mapData)

}

func loadLegacySVID(dir string) ([]*x509.Certificate, time.Time, error) {
	data, mtime, err := readFile(legacySVIDPath(dir))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to read legacy SVID: %w", err)
	}

	certChain, err := x509.ParseCertificates(data)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse legacy SVID: %w", err)
	}
	return certChain, mtime, nil
}

func loadLegacySVIDFromK8S(namespace, secretname string) ([]*x509.Certificate, time.Time, error) {
	store, tm, err := loadDataFromK8S(namespace, secretname)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, err
	}
	return store.SVID, tm, nil
}

func storeLegacySVID(dir string, svidChain []*x509.Certificate) error {
	data := new(bytes.Buffer)
	for _, cert := range svidChain {
		data.Write(cert.Raw)
	}
	if err := diskutil.AtomicWritePrivateFile(legacySVIDPath(dir), data.Bytes()); err != nil {
		return fmt.Errorf("failed to store legacy SVID: %w", err)
	}
	return nil
}

func isUnique(mapData map[string][]byte, byteData []byte) bool {

	isUnique := true

	for _, v := range mapData {
		if bytes.Equal(v, byteData) {
			// if the value already exists, set isUnique to false and break the loop
			isUnique = false
			break
		}
	}

	return isUnique
}

func storeLegacySVIDToK8S(namespace, secret string, svidChain []*x509.Certificate) error {
	mapData := make(map[string][]byte)

	for i, cert := range svidChain {
		block := pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
		pemBytes := pem.EncodeToMemory(&block)
		if isUnique(mapData, pemBytes) {
			key := fmt.Sprintf("svid-legacy-%d", i)
			mapData[key] = pemBytes
		}
	}

	return util.CreateK8sSecrets(namespace, secret, mapData)

}

func deleteLegacySVID(dir string) error {
	err := os.Remove(legacySVIDPath(dir))
	switch {
	case err == nil, errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("failed to delete legacy SVID: %w", err)
	}
}

func legacyBundlePath(dir string) string {
	return filepath.Join(dir, "bundle.der")
}

func legacySVIDPath(dir string) string {
	return filepath.Join(dir, "agent_svid.der")
}
