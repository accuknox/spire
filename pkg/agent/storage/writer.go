package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/accuknox/spire/pkg/common/diskutil"
	"github.com/accuknox/spire/pkg/common/util"
	log "github.com/sirupsen/logrus"
)

func storeDataToK8S(namespace, secret, backupFile string, data storageData) error {

	mapData := make(map[string][]byte)

	marshaled, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	now := time.Now()

	td, err := now.MarshalText()
	if err != nil {
		log.WithError(err).Warn("Could not marshal time.")
	}

	mapData["agent-data"] = marshaled
	mapData["agent-data-time"] = td
	mapData["version"] = util.Int64ToBytes(data.Version)

	if fileErr := backupDataToFile(backupFile, mapData); fileErr != nil {
		log.WithError(fileErr).Error("Failed to backup data")
	}

	return createSecretWithBackoff(namespace, secret, mapData)
}

func createSecretWithBackoff(namespace, secret string, data map[string][]byte) error {

	var err error
	for i := range 3 {
		err = util.CreateK8sSecrets(namespace, secret, data)
		if err == nil {
			break
		}
		if !shouldRetry(err) {
			break
		}

		time.Sleep(backoff(i))
	}

	return err

}

func backupDataToFile(backupFile string, data map[string][]byte) error {

	fileDataMap := make(map[string][]byte, 0)

	var oldVersion, newVersion int64

	if tmpVersion, ok := data["version"]; ok {
		newVersion = util.BytesToInt64(tmpVersion)
	}

	fileData, err := os.ReadFile(backupFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read backup file: %w", err)
		}
	} else {
		if err := json.Unmarshal(fileData, &fileDataMap); err != nil {
			return fmt.Errorf("failed to unmarshal backup file: %w", err)
		}

		if tmpVersion, ok := fileDataMap["version"]; ok {
			oldVersion = util.BytesToInt64(tmpVersion)
		}
	}
	for key, value := range data {
		if value == nil {
			continue
		}

		fileDataMap[key] = value

	}
	fileDataMap["version"] = util.Int64ToBytes(max(oldVersion, newVersion) + 1)

	return writeDataToFile(backupFile, fileDataMap)

}

func storeData(dir, ns, secret, backupFile string, data storageData) error {

	if ns != "" && secret != "" && dir == "" {
		if err := storeDataToK8S(ns, secret, backupFile, data); err != nil {
			return err
		}
	} else {
		path := dataPath(dir)

		return writeDataToFile(path, data)
	}
	return nil
}

func writeDataToFile(path string, data any) error {
	marshaled, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := diskutil.AtomicWritePrivateFile(path, marshaled); err != nil {
		return fmt.Errorf("failed to write data file: %w", err)
	}

	return nil
}
