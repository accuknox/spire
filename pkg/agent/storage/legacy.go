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

func loadLegacyBundle(dir string) ([]*x509.Certificate, time.Time, int64, error) {
	data, mtime, err := readFile(legacyBundlePath(dir))
	if err != nil {
		return nil, time.Time{}, 0, fmt.Errorf("failed to read legacy bundle: %w", err)
	}

	bundle, err := x509.ParseCertificates(data)
	if err != nil {
		return nil, time.Time{}, 0, fmt.Errorf("failed to parse legacy bundle: %w", err)
	}
	return bundle, mtime, 0, nil
}

func getLegacyData(namespace, secretname, dataType string, secretData map[string][]byte) ([]byte, []byte, []byte, error) {
	var timeByte, dataByte, versionByte []byte

	if secretData == nil {

		secret, err := util.GetK8sSecrets(namespace, secretname)

		if secret.Data == nil {
			err = ErrNoData
		}

		if err != nil {
			if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoData) {
				return nil, nil, nil, nil
			}
			return nil, nil, nil, err
		}
		secretData = secret.Data
	}

	for key, value := range secretData {
		if key == dataType+"-legacy" {
			dataByte = value
		}
		if key == dataType+"-legacy-time" {
			timeByte = value
		}
		if key == dataType+"-version" {
			versionByte = value
		}
	}

	return dataByte, timeByte, versionByte, nil
}
func loadLegacyBundleFromK8S(namespace, secretname string) ([]*x509.Certificate, time.Time, int64, error) {

	bundleByte, timeByte, versionByte, err := getLegacyData(namespace, secretname, "bundle", nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, time.Time{}, 0, nil
		}
		return nil, time.Time{}, 0, err
	}

	return loadCertAndTime(bundleByte, timeByte, versionByte)
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
func storeLegacyBundleToK8S(namespace, secret string, backupFile string, bundle []*x509.Certificate) error {
	mapData := make(map[string][]byte)
	data := new(bytes.Buffer)
	for _, cert := range bundle {
		data.Write(cert.Raw)
	}

	now := time.Now()

	td, err := now.MarshalText()
	if err != nil {
		log.WithError(err).Info("Could not marshal time.")
	}

	mapData["bundle-legacy"] = data.Bytes()
	mapData["bundle-legacy-time"] = td

	if fileErr := backupDataToFile(backupFile, mapData); fileErr != nil {
		log.WithError(fileErr).Error("Failed to backup data")
	}

	return createSecretWithBackoff(namespace, secret, mapData)

}

func loadLegacySVID(dir string) ([]*x509.Certificate, time.Time, int64, error) {
	data, mtime, err := readFile(legacySVIDPath(dir))
	if err != nil {
		return nil, time.Time{}, 0, fmt.Errorf("failed to read legacy SVID: %w", err)
	}

	certChain, err := x509.ParseCertificates(data)
	if err != nil {
		return nil, time.Time{}, 0, fmt.Errorf("failed to parse legacy SVID: %w", err)
	}
	return certChain, mtime, 0, nil
}

func loadLegacySVIDFromK8S(namespace, secretname string) ([]*x509.Certificate, time.Time, int64, error) {

	svidByte, timeByte, versionByte, err := getLegacyData(namespace, secretname, "svid", nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, time.Time{}, 0, nil
		}
		return nil, time.Time{}, 0, err
	}

	return loadCertAndTime(svidByte, timeByte, versionByte)
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

func storeLegacySVIDToK8S(namespace, secret, backupFile string, svidChain []*x509.Certificate) error {
	mapData := make(map[string][]byte)
	data := new(bytes.Buffer)

	for _, cert := range svidChain {
		data.Write(cert.Raw)
	}

	now := time.Now()

	td, err := now.MarshalText()
	if err != nil {
		log.WithError(err).Info("Could not marshal time. ")

	}
	mapData["svid-legacy-time"] = td

	mapData["svid-legacy"] = data.Bytes()

	if fileErr := backupDataToFile(backupFile, mapData); fileErr != nil {
		log.WithError(fileErr).Error("Failed to backup data")
	}

	return createSecretWithBackoff(namespace, secret, mapData)

}

func deleteLegacySVID(dir string) error {

	err := os.RemoveAll(dir)
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
func legacySVIDDataPath(dir string) string {
	return filepath.Join(dir, "agent-data.json")
}
