package storage

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/accuknox/spire/pkg/common/util"
	log "github.com/sirupsen/logrus"
)

const (
	LegacyDataTypeSVID   = "svid"
	LegacyDataTypeBundle = "bundle"
)

func loadData(dir string) (storageData, time.Time, error) {
	path := dataPath(dir)

	marshaled, mtime, err := readFile(path)
	if err != nil {
		return storageData{}, time.Time{}, fmt.Errorf("failed to read data: %w", err)
	}

	var data storageData
	if err := json.Unmarshal(marshaled, &data); err != nil {
		return storageData{}, time.Time{}, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return data, mtime, nil
}
func loadDataFromK8S(namespace, secretname string) (storageData, time.Time, error) {

	secret, err := util.GetK8sSecrets(namespace, secretname)
	if secret.Data == nil {
		err = ErrNoData
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoData) {
			return storageData{}, time.Time{}, nil
		}
		return storageData{}, time.Time{}, err
	}

	return getAgentData(secret.Data)

}

func getAgentData(secretData map[string][]byte) (storageData, time.Time, error) {
	var (
		data                            storageData
		timeByte, dataByte, versionByte []byte
	)
	for key, value := range secretData {
		if key == "agent-data" {
			dataByte = value
		}
		if key == "agent-data-time" {
			timeByte = value
		}
		if key == "agent-data-version" {
			versionByte = value
		}
	}

	if dataByte == nil {
		log.Warn("no agent data found")
		return storageData{}, time.Time{}, nil
	}

	err := json.Unmarshal(dataByte, &data)
	if err != nil {
		return storageData{}, time.Time{}, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	var td = time.Now()

	if timeByte != nil {
		err = td.UnmarshalText(timeByte)
		if err != nil {
			log.WithField("error", err).Warn("Could not unmarshal time. Updating time as current time")
		}
	}

	var version int64
	if versionByte != nil {
		version = util.BytesToInt64(versionByte)
		data.Version = version
	}

	return data, td, nil
}

func loadDataWithBackoff(dir, ns, secret, backupFile string) (storageData, time.Time, error) {
	if dir != "" {
		return loadData(dir)
	}
	return loadAgentData(ns, secret, backupFile)
}

func loadAgentData(ns, secret, backupFile string) (storageData, time.Time, error) {
	var lastErr error
	if ns != "" && secret != "" {
		for i := range 3 {
			data, dataTime, err := loadDataFromK8S(ns, secret)
			if err == nil {
				return data, dataTime, nil
			}
			lastErr = err
			if !shouldRetry(err) {
				break
			}
			time.Sleep(backoff(i))
		}
	}

	if data, _, err := readFile(backupFile); err == nil {
		mapData := make(map[string][]byte)
		if err := json.Unmarshal(data, &mapData); err == nil {
			return getAgentData(mapData)
		}
	}

	return storageData{}, time.Time{}, fmt.Errorf("failed to get agent data: %w", lastErr)

}

func loadLegacyDataWithBackoff(dataType, dir, ns, secret, backupFile string) ([]*x509.Certificate, time.Time, int64, error) {
	switch dataType {
	case LegacyDataTypeSVID:
		if dir != "" {
			return loadLegacySVID(dir)
		}
		return loadLegacySecret(LegacyDataTypeSVID, ns, secret, backupFile)
	case LegacyDataTypeBundle:
		if dir != "" {
			return loadLegacyBundle(dir)
		}
		return loadLegacySecret(LegacyDataTypeSVID, ns, secret, backupFile)
	default:
		return nil, time.Time{}, 0, fmt.Errorf("unknown legacy data type: %q", dataType)
	}
}

func loadLegacySecret(dataType, namespace, secretname, backupFile string) ([]*x509.Certificate, time.Time, int64, error) {
	var (
		err, fileErr  error
		fileCerts     []*x509.Certificate
		fileTime      time.Time
		fileVersion   int64
		secretCerts   []*x509.Certificate
		secretTime    time.Time
		secretVersion int64
	)

	for i := range 3 {
		switch dataType {
		case LegacyDataTypeSVID:
			secretCerts, secretTime, secretVersion, err = loadLegacySVIDFromK8S(namespace, secretname)
		case LegacyDataTypeBundle:
			secretCerts, secretTime, secretVersion, err = loadLegacySVIDFromK8S(namespace, secretname)
		}
		if err == nil {
			break
		}
		if !shouldRetry(err) {
			break
		}
		time.Sleep(backoff(i))
	}

	if data, _, err := readFile(backupFile); err == nil {
		mapData := make(map[string][]byte)
		if err := json.Unmarshal(data, &mapData); err == nil {
			if cert, time, version, err := getLegacyData("", "", dataType, mapData); err == nil {
				fileCerts, fileTime, fileVersion, fileErr = loadCertAndTime(cert, time, version)
			}
		}
	}

	if err == nil && fileErr != nil {
		return secretCerts, secretTime, secretVersion, nil
	} else if fileErr == nil && err != nil {

		return fileCerts, fileTime, fileVersion, nil
	} else if fileErr == nil && err == nil {
		if secretVersion > fileVersion {
			return secretCerts, secretTime, secretVersion, nil
		}
		return fileCerts, fileTime, fileVersion, nil
	}

	return nil, time.Time{}, 0, fmt.Errorf("failed to read legacy SVID: %w", err)

}

func shouldRetry(err error) bool {

	if errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

func backoff(attempt int) time.Duration {
	base := time.Second
	max := 30 * time.Second

	d := min(time.Duration(1<<attempt)*base, max)

	jitter := time.Duration(rand.Int63n(int64(d / 3)))
	return d - jitter
}

func loadCertAndTime(certByte, timeByte, versionByte []byte) ([]*x509.Certificate, time.Time, int64, error) {
	cert, err := x509.ParseCertificates(certByte)
	if err != nil {
		return nil, time.Time{}, 0, fmt.Errorf("failed to parse legacy bundle: %w", err)
	}

	var td = time.Now()
	if timeByte != nil {
		err = td.UnmarshalText(timeByte)
		if err != nil {
			log.WithError(err).Warn("Could not unmarshal time. Updating time as current time")
		}
	}
	var version int64
	if versionByte != nil {
		version = util.BytesToInt64(versionByte)
	}

	return cert, td, version, nil
}

func readFile(path string) ([]byte, time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	fi, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to stat file: %w", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to read file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to close file: %w", err)
	}
	return data, fi.ModTime(), nil
}
