// Copyright 2016 Apcera Inc. All rights reserved.
// +build go1.5,!go1.7

package util

import (
	"crypto/tls"
)

// CloneTLSConfig returns a copy of c. Only the exported fields are copied.
// This is temporary, until this is provided by the language.
// https://go-review.googlesource.com/#/c/28075/
func CloneTLSConfig(c *tls.Config) *tls.Config {
	return &tls.Config{
		Rand:                     c.Rand,
		Time:                     c.Time,
		Certificates:             c.Certificates,
		NameToCertificate:        c.NameToCertificate,
		GetCertificate:           c.GetCertificate,
		RootCAs:                  c.RootCAs,
		NextProtos:               c.NextProtos,
		ServerName:               c.ServerName,
		ClientAuth:               c.ClientAuth,
		ClientCAs:                c.ClientCAs,
		InsecureSkipVerify:       c.InsecureSkipVerify,
		CipherSuites:             c.CipherSuites,
		PreferServerCipherSuites: c.PreferServerCipherSuites,
		SessionTicketsDisabled:   c.SessionTicketsDisabled,
		SessionTicketKey:         c.SessionTicketKey,
		ClientSessionCache:       c.ClientSessionCache,
		MinVersion:               c.MinVersion,
		MaxVersion:               c.MaxVersion,
		CurvePreferences:         c.CurvePreferences,
	}
}
