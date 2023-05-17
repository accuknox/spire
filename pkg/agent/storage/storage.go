package storage

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/accuknox/spire/pkg/common/diskutil"
	"github.com/accuknox/spire/pkg/common/pemutil"
	"github.com/accuknox/spire/pkg/common/util"
	log "github.com/sirupsen/logrus"
)

var (
	ErrNotCached = errors.New("not cached")
	ErrNotFound  = errors.New("not found")
	ErrNoData    = errors.New("no data found")
)

type Storage interface {
	// LoadSVID loads the SVID from storage. Returns ErrNotCached if the SVID
	// does not exist in the cache.
	LoadSVID() ([]*x509.Certificate, bool, error)

	// StoreSVID stores the SVID.
	StoreSVID(certs []*x509.Certificate, reattestable bool) error

	// DeleteSVID deletes the SVID.
	DeleteSVID() error

	// LoadBundle loads the bundle from storage. Returns ErrNotCached if the
	// bundle does not exist in the cache.
	LoadBundle() ([]*x509.Certificate, error)

	// StoreBundle stores the bundle.
	StoreBundle(certs []*x509.Certificate) error
}

func Open(dir, ns, secret string) (Storage, error) {
	// TODO: stop updating and instead delete legacy files in 1.5.0
	var data storageData
	var dataTime, legacySVIDTime, legacyBundleTime time.Time
	var err error
	var legacySVID, legacyBundle []*x509.Certificate

	if dir == "" {
		legacySVID, legacySVIDTime, err = loadLegacySVIDFromK8S(ns, secret)
	} else {
		legacySVID, legacySVIDTime, err = loadLegacySVID(dir)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if dir == "" {
		legacyBundle, legacyBundleTime, err = loadLegacyBundleFromK8S(ns, secret)
	} else {
		legacyBundle, legacyBundleTime, err = loadLegacyBundle(dir)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	if ns != "" && secret != "" && dir == "" {
		data, dataTime, err = loadDataFromK8S(ns, secret)
	} else {
		data, dataTime, err = loadData(dir)
	}

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	storeNow := false

	if !legacySVIDTime.IsZero() && (dataTime.IsZero() || dataTime.Before(legacySVIDTime)) {
		storeNow = true
		data.SVID = legacySVID
	}

	if !legacyBundleTime.IsZero() && (dataTime.IsZero() || dataTime.Before(legacyBundleTime)) {
		storeNow = true
		data.Bundle = legacyBundle
	}

	if storeNow {
		if ns != "" && secret != "" && dir == "" {
			if err := storeDataToK8S(ns, secret, data); err != nil {
				return nil, err
			}
		} else {
			if err := storeData(dir, data); err != nil {
				return nil, err
			}
		}
	}

	return &storage{
		dir:        dir,
		data:       data,
		Namespace:  ns,
		SecretName: secret,
	}, nil
}

type storage struct {
	dir        string
	Namespace  string
	SecretName string

	mtx  sync.RWMutex
	data storageData
}

func (s *storage) LoadBundle() ([]*x509.Certificate, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	if len(s.data.Bundle) == 0 {
		return nil, ErrNotCached
	}
	return s.data.Bundle, nil
}

func (s *storage) StoreBundle(bundle []*x509.Certificate) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.Namespace != "" && s.SecretName != "" {
		if err := storeLegacyBundleToK8S(s.Namespace, s.SecretName, bundle); err != nil {
			return err
		}
	} else {

		if err := storeLegacyBundle(s.dir, bundle); err != nil {
			return err
		}
	}

	data := s.data
	data.Bundle = bundle

	if s.Namespace != "" && s.SecretName != "" {
		if err := storeDataToK8S(s.Namespace, s.SecretName, data); err != nil {
			return err
		}
	} else {
		if err := storeData(s.dir, data); err != nil {
			return err
		}
	}

	s.data = data
	return nil
}

func (s *storage) LoadSVID() ([]*x509.Certificate, bool, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	if len(s.data.SVID) == 0 {
		return nil, false, ErrNotCached
	}
	return s.data.SVID, s.data.Reattestable, nil
}

func (s *storage) StoreSVID(svid []*x509.Certificate, reattestable bool) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.Namespace != "" && s.SecretName != "" {
		if err := storeLegacySVIDToK8S(s.Namespace, s.SecretName, svid); err != nil {
			return err
		}
	} else {

		if err := storeLegacySVID(s.dir, svid); err != nil {
			return err
		}
	}
	data := s.data
	data.SVID = svid
	data.Reattestable = reattestable

	if s.Namespace != "" && s.SecretName != "" {
		if err := storeDataToK8S(s.Namespace, s.SecretName, data); err != nil {
			return err
		}
	} else {
		if err := storeData(s.dir, data); err != nil {
			return err
		}
	}

	s.data = data
	return nil
}

func (s *storage) DeleteSVID() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.Namespace != "" && s.SecretName != "" {
		if err := util.DeleteK8sSecrets(s.Namespace, s.SecretName, "svid-legacy"); err != nil {
			return err
		}
	} else {
		if err := deleteLegacySVID(s.dir); err != nil {
			return err
		}
	}
	data := s.data
	data.SVID = nil
	data.Reattestable = false
	if s.Namespace != "" && s.SecretName != "" {
		if err := storeDataToK8S(s.Namespace, s.SecretName, data); err != nil {
			return err
		}
	} else {
		if err := storeData(s.dir, data); err != nil {
			return err
		}
	}

	s.data = data
	return nil
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

type storageJSON struct {
	SVID         [][]byte `json:"svid"`
	Bundle       [][]byte `json:"bundle"`
	Reattestable bool     `json:"reattestable"`
}

type storageData struct {
	SVID         []*x509.Certificate
	Bundle       []*x509.Certificate
	Reattestable bool
}

func (d storageData) MarshalJSON() ([]byte, error) {
	svid, err := encodeCertificates(d.SVID)
	if err != nil {
		return nil, fmt.Errorf("failed to encode SVID: %w", err)
	}
	bundle, err := encodeCertificates(d.Bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to encode bundle: %w", err)
	}
	return json.Marshal(storageJSON{
		SVID:         svid,
		Bundle:       bundle,
		Reattestable: d.Reattestable,
	})
}

func (d *storageData) UnmarshalJSON(b []byte) error {
	j := new(storageJSON)
	if err := json.Unmarshal(b, j); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}
	svid, err := parseCertificates(j.SVID)
	if err != nil {
		return fmt.Errorf("failed to parse SVID: %w", err)
	}
	bundle, err := parseCertificates(j.Bundle)
	if err != nil {
		return fmt.Errorf("failed to parse bundle: %w", err)
	}

	d.SVID = svid
	d.Bundle = bundle
	d.Reattestable = j.Reattestable
	return nil
}

func storeData(dir string, data storageData) error {
	path := dataPath(dir)

	marshaled, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := diskutil.AtomicWritePrivateFile(path, marshaled); err != nil {
		return fmt.Errorf("failed to write data file: %w", err)
	}

	return nil
}
func storeDataToK8S(namespace, secret string, data storageData) error {

	mapData := make(map[string][]byte)

	marshaled, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	now := time.Now()

	td, err := now.MarshalBinary()
	if err != nil {
		log.WithError(err).Info("Could not marshal time.")
	}

	mapData["agent-data"] = marshaled
	mapData["agent-data-time"] = td

	return util.CreateK8sSecrets(namespace, secret, mapData)
}

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

	var data storageData
	secret, err := util.GetK8sSecrets(namespace, secretname)

	if secret.Data == nil {
		err = ErrNoData
	}

	var dataByte, timeByte []byte

	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoData) {
			return storageData{}, time.Time{}, nil
		}
		return storageData{}, time.Time{}, err
	}

	for key, value := range secret.Data {
		if key == "agent-data" {
			dataByte = value
		}
		if key == "agent-data-time" {
			timeByte = value
		}
	}
	if err := json.Unmarshal(dataByte, &data); err != nil {
		return storageData{}, time.Time{}, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	var td time.Time

	err = td.UnmarshalBinary(timeByte)
	if err != nil {
		log.WithError(err).Info("Could not unmarshal time. Updating time as current time")
		td = time.Now()
	}

	return data, td, nil
}

func parseCertificates(certsPEM [][]byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for _, certPEM := range certsPEM {
		cert, err := pemutil.ParseCertificate(certPEM)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func encodeCertificates(certs []*x509.Certificate) ([][]byte, error) {
	var certsPEM [][]byte
	for _, cert := range certs {
		if _, err := x509.ParseCertificate(cert.Raw); err != nil {
			return nil, err
		}
		certsPEM = append(certsPEM, pemutil.EncodeCertificate(cert))
	}
	return certsPEM, nil
}

func dataPath(dir string) string {
	return filepath.Join(dir, "agent-data.json")
}
