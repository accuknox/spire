package storage

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/accuknox/spire/pkg/common/pemutil"
	"github.com/accuknox/spire/pkg/common/util"
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

func Open(dir, ns, secret, backupDir string) (Storage, error) {
	// TODO: stop updating and instead delete legacy files in 1.5.0

	bFile := backupDataPath(backupDir)

	legacySVID, legacySVIDTime, legacySvidVersion, err := loadLegacyDataWithBackoff(LegacyDataTypeSVID, dir, ns, secret, bFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	legacyBundle, legacyBundleTime, legacyBundleVer, err := loadLegacyDataWithBackoff(LegacyDataTypeBundle, dir, ns, secret, bFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	data, dataTime, err := loadDataWithBackoff(dir, ns, secret, bFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	storeNow := false

	if !legacySVIDTime.IsZero() && (dataTime.IsZero() || dataTime.Before(legacySVIDTime)) {
		storeNow = true
		data.SVID = legacySVID
		data.Version = legacySvidVersion
	}

	if !legacyBundleTime.IsZero() && (dataTime.IsZero() || dataTime.Before(legacyBundleTime)) {
		storeNow = true
		data.Bundle = legacyBundle
		data.Version = legacyBundleVer
	}

	if data.Version == 0 {
		data.Version = 1
	} else {
		data.Version++
	}

	if storeNow {
		if err := storeData(dir, ns, secret, bFile, data); err != nil {
			return nil, err
		}
	}

	return &storage{
		dir:        dir,
		data:       data,
		Namespace:  ns,
		SecretName: secret,
		backupFile: bFile,
	}, nil
}

type storage struct {
	dir        string
	Namespace  string
	SecretName string
	backupFile string

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
		if err := storeLegacyBundleToK8S(s.Namespace, s.SecretName, s.backupFile, bundle); err != nil {
			return err
		}
	} else {

		if err := storeLegacyBundle(s.dir, bundle); err != nil {
			return err
		}
	}

	data := s.data
	data.Bundle = bundle

	if err := storeData(s.dir, s.Namespace, s.SecretName, s.backupFile, data); err != nil {
		return err
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
		if err := storeLegacySVIDToK8S(s.Namespace, s.SecretName, s.backupFile, svid); err != nil {
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

	if err := storeData(s.dir, s.Namespace, s.SecretName, s.backupFile, data); err != nil {
		return err
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
	if err := storeData(s.dir, s.Namespace, s.SecretName, s.backupFile, data); err != nil {
		return err
	}

	s.data = data
	return nil
}

type storageJSON struct {
	SVID         [][]byte `json:"svid"`
	Bundle       [][]byte `json:"bundle"`
	Reattestable bool     `json:"reattestable"`
	Version      int64    `json:"version"`
}

type storageData struct {
	SVID         []*x509.Certificate
	Bundle       []*x509.Certificate
	Reattestable bool
	Version      int64
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
		Version:      d.Version,
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
	d.Version = j.Version
	return nil
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

func backupDataPath(dir string) string {
	return filepath.Join(dir, "agent-data.json.bak")
}
