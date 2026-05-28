package certdb

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx/types"
)

// CertificateRecord encodes a certificate and its metadata
// that will be recorded in a database.
type CertificateRecord struct {
	Serial    string    `db:"serial_number"`
	AKI       string    `db:"authority_key_identifier"`
	CALabel   string    `db:"ca_label"`
	Status    string    `db:"status"`
	Reason    int       `db:"reason"`
	Expiry    time.Time `db:"expiry"`
	RevokedAt time.Time `db:"revoked_at"`
	PEM       string    `db:"pem"`
	// the following fields will be empty for data inserted before migrate 002 has been run.
	IssuedAt     *time.Time     `db:"issued_at"`
	NotBefore    *time.Time     `db:"not_before"`
	MetadataJSON types.JSONText `db:"metadata"`
	SANsJSON     types.JSONText `db:"sans"`
	CommonName   sql.NullString `db:"common_name"`
}

// SetMetadata sets the metadata json
func (c *CertificateRecord) SetMetadata(meta map[string]interface{}) error {
	marshaled, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	c.MetadataJSON = types.JSONText(marshaled)
	return nil
}

// GetMetadata returns the json metadata
func (c *CertificateRecord) GetMetadata() (map[string]interface{}, error) {
	var meta map[string]interface{}
	err := c.MetadataJSON.Unmarshal(&meta)
	return meta, err
}

// SetSANs sets the list of sans
func (c *CertificateRecord) SetSANs(meta []string) error {
	marshaled, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	c.SANsJSON = types.JSONText(marshaled)
	return nil
}

// GetSANs returns the json SANs
func (c *CertificateRecord) GetSANs() ([]string, error) {
	var sans []string
	err := c.SANsJSON.Unmarshal(&sans)
	return sans, err
}

// OCSPRecord encodes a OCSP response body and its metadata
// that will be recorded in a database.
type OCSPRecord struct {
	Serial string    `db:"serial_number"`
	AKI    string    `db:"authority_key_identifier"`
	Body   string    `db:"body"`
	Expiry time.Time `db:"expiry"`
}

// Accessor abstracts the CRUD of certdb objects from a DB.
type Accessor interface {
	InsertCertificate(cr CertificateRecord) error
	GetCertificate(serial, aki string) ([]CertificateRecord, error)
	GetUnexpiredCertificates() ([]CertificateRecord, error)
	GetRevokedAndUnexpiredCertificates() ([]CertificateRecord, error)
	GetUnexpiredCertificatesByLabel(labels []string) (crs []CertificateRecord, err error)
	GetRevokedAndUnexpiredCertificatesByLabel(label string) ([]CertificateRecord, error)
	GetRevokedAndUnexpiredCertificatesByLabelSelectColumns(label string) ([]CertificateRecord, error)
	RevokeCertificate(serial, aki string, reasonCode int) error
	InsertOCSP(rr OCSPRecord) error
	GetOCSP(serial, aki string) ([]OCSPRecord, error)
	GetUnexpiredOCSPs() ([]OCSPRecord, error)
	UpdateOCSP(serial, aki, body string, expiry time.Time) error
	UpsertOCSP(serial, aki, body string, expiry time.Time) error
}
