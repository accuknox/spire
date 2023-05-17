package storage

import (
	"bytes"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/accuknox/spire/pkg/common/diskutil"
	"github.com/accuknox/spire/pkg/common/util"
	log "github.com/sirupsen/logrus"
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

func getLegacyDataFromK8SSecret(namespace, secretname, dataType string) ([]byte, []byte, error) {
	secret, err := util.GetK8sSecrets(namespace, secretname)

	var timeByte, bundleByte []byte

	if secret.Data == nil {
		err = ErrNoData
	}

	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoData) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	for key, value := range secret.Data {
		if key == dataType+"-legacy" {
			bundleByte = value
		}
		if key == dataType+"-legacy-time" {
			timeByte = value
		}
	}

	return bundleByte, timeByte, nil
}
func loadLegacyBundleFromK8S(namespace, secretname string) ([]*x509.Certificate, time.Time, error) {

	bundleByte, timeByte, err := getLegacyDataFromK8SSecret(namespace, secretname, "bundle")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, err
	}

	bundle, err := x509.ParseCertificates(bundleByte)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse legacy bundle: %w", err)
	}

	var td time.Time

	err = td.UnmarshalBinary(timeByte)
	if err != nil {
		log.WithError(err).Info("Could not unmarshal time. Updating time as current time")
		td = time.Now()
	}

	return bundle, td, nil
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
	data := new(bytes.Buffer)
	for _, cert := range bundle {
		data.Write(cert.Raw)
	}

	now := time.Now()

	td, err := now.MarshalBinary()
	if err != nil {
		log.WithError(err).Info("Could not marshal time.")
	}

	mapData["bundle-legacy"] = data.Bytes()
	mapData["bundle-legacy-time"] = td

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

	svidByte, timeByte, err := getLegacyDataFromK8SSecret(namespace, secretname, "svid")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, err
	}

	svids, err := x509.ParseCertificates(svidByte)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse legacy SVIDs: %w", err)
	}

	var td time.Time

	err = td.UnmarshalBinary(timeByte)
	if err != nil {
		log.WithError(err).Info("Could not unmarshal time. Updating time as current time")
		td = time.Now()
	}

	return svids, td, nil
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

func storeLegacySVIDToK8S(namespace, secret string, svidChain []*x509.Certificate) error {
	mapData := make(map[string][]byte)
	data := new(bytes.Buffer)

	for _, cert := range svidChain {
		data.Write(cert.Raw)
	}

	now := time.Now()

	td, err := now.MarshalBinary()
	if err != nil {
		log.WithError(err).Info("Could not marshal time. ")

	}
	mapData["svid-legacy-time"] = td

	mapData["svid-legacy"] = data.Bytes()

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
