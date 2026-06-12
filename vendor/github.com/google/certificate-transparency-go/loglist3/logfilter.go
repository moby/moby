// Copyright 2022 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loglist3

import (
	"fmt"

	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
	"k8s.io/klog/v2"
)

// LogRoots maps Log-URLs and TiledLog-SubmissionURLs (stated at LogList)
// to the pools of their accepted root-certificates.
type LogRoots map[string]*x509util.PEMCertPool

// Compatible creates a new LogList containing only Logs matching the temporal,
// root-acceptance and Log-status conditions.
func (ll *LogList) Compatible(cert *x509.Certificate, certRoot *x509.Certificate, roots LogRoots) LogList {
	urls := make([]string, 0, len(roots))
	for url := range roots {
		urls = append(urls, url)
	}
	klog.V(1).Info(urls)
	active := ll.TemporallyCompatible(cert)
	// Do not check root compatbility if roots are not being provided.
	if certRoot == nil {
		return active
	}
	return active.RootCompatible(certRoot, roots)
}

// SelectByStatus creates a new LogList containing only logs with status
// provided from the original.
func (ll *LogList) SelectByStatus(lstats []LogStatus) LogList {
	var active LogList
	for _, op := range ll.Operators {
		activeOp := *op
		activeOp.Logs = []*Log{}
		for _, l := range op.Logs {
			for _, lstat := range lstats {
				if l.State.LogStatus() == lstat {
					activeOp.Logs = append(activeOp.Logs, l)
					break
				}
			}
		}
		activeOp.TiledLogs = []*TiledLog{}
		for _, l := range op.TiledLogs {
			for _, lstat := range lstats {
				if l.State.LogStatus() == lstat {
					activeOp.TiledLogs = append(activeOp.TiledLogs, l)
					break
				}
			}
		}
		if len(activeOp.Logs) > 0 || len(activeOp.TiledLogs) > 0 {
			active.Operators = append(active.Operators, &activeOp)
		}
	}
	return active
}

// RootCompatible creates a new LogList containing only the logs of original
// LogList that are compatible with the provided cert, according to
// the passed in collection of per-log roots. Logs that are missing from
// the collection are treated as always compatible and included, even if
// an empty cert root is passed in.
// Cert-root when provided is expected to be CA-cert.
func (ll *LogList) RootCompatible(certRoot *x509.Certificate, roots LogRoots) LogList {
	var compatible LogList

	// Check whether root is a CA-cert.
	if certRoot != nil && !certRoot.IsCA {
		// Compatible method expects fully rooted chain, while last cert of the chain provided is not root.
		// Proceed anyway.
		return compatible
	}

	for _, op := range ll.Operators {
		compatibleOp := *op
		compatibleOp.Logs = []*Log{}
		for _, l := range op.Logs {
			// If root set is not defined, we treat Log as compatible assuming no
			// knowledge of its roots.
			if _, ok := roots[l.URL]; !ok {
				compatibleOp.Logs = append(compatibleOp.Logs, l)
				continue
			}

			if certRoot == nil {
				continue
			}

			// Check root is accepted.
			if roots[l.URL].Included(certRoot) {
				compatibleOp.Logs = append(compatibleOp.Logs, l)
			}
		}
		compatibleOp.TiledLogs = []*TiledLog{}
		for _, l := range op.TiledLogs {
			// If root set is not defined, we treat Log as compatible assuming no
			// knowledge of its roots.
			if _, ok := roots[l.SubmissionURL]; !ok {
				compatibleOp.TiledLogs = append(compatibleOp.TiledLogs, l)
				continue
			}

			if certRoot == nil {
				continue
			}

			// Check root is accepted.
			if roots[l.SubmissionURL].Included(certRoot) {
				compatibleOp.TiledLogs = append(compatibleOp.TiledLogs, l)
			}
		}
		if len(compatibleOp.Logs) > 0 || len(compatibleOp.TiledLogs) > 0 {
			compatible.Operators = append(compatible.Operators, &compatibleOp)
		}
	}
	logMessage := "Root compatible operators: \n"
	for _, operator := range ll.Operators {
		logMessage += fmt.Sprintf("Operator: %s\n", operator.Name)
		for _, l := range operator.Logs {
			logMessage += fmt.Sprintf("\t%s\n", l.URL)
		}
		for _, l := range operator.TiledLogs {
			logMessage += fmt.Sprintf("\t%s\n", l.SubmissionURL)
		}
	}
	klog.V(1).Info(logMessage)
	return compatible
}

// TemporallyCompatible creates a new LogList containing only the logs of
// original LogList that are compatible with the provided cert, according to
// NotAfter and TemporalInterval matching.
// Returns empty LogList if nil-cert is provided.
func (ll *LogList) TemporallyCompatible(cert *x509.Certificate) LogList {
	var compatible LogList
	if cert == nil {
		return compatible
	}

	for _, op := range ll.Operators {
		compatibleOp := *op
		compatibleOp.Logs = []*Log{}
		for _, l := range op.Logs {
			if l.TemporalInterval == nil {
				compatibleOp.Logs = append(compatibleOp.Logs, l)
				continue
			}
			if cert.NotAfter.Before(l.TemporalInterval.EndExclusive) && (cert.NotAfter.After(l.TemporalInterval.StartInclusive) || cert.NotAfter.Equal(l.TemporalInterval.StartInclusive)) {
				compatibleOp.Logs = append(compatibleOp.Logs, l)
			}
		}
		compatibleOp.TiledLogs = []*TiledLog{}
		for _, l := range op.TiledLogs {
			if l.TemporalInterval == nil {
				compatibleOp.TiledLogs = append(compatibleOp.TiledLogs, l)
				continue
			}
			if cert.NotAfter.Before(l.TemporalInterval.EndExclusive) && (cert.NotAfter.After(l.TemporalInterval.StartInclusive) || cert.NotAfter.Equal(l.TemporalInterval.StartInclusive)) {
				compatibleOp.TiledLogs = append(compatibleOp.TiledLogs, l)
			}
		}
		if len(compatibleOp.Logs) > 0 || len(compatibleOp.TiledLogs) > 0 {
			compatible.Operators = append(compatible.Operators, &compatibleOp)
		}
	}
	logMessage := "Temporal compatible logs: \n"
	for _, operator := range ll.Operators {
		logMessage += fmt.Sprintf("Operator: %s\n", operator.Name)
		for _, l := range operator.Logs {
			logMessage += fmt.Sprintf("\t%s\n", l.URL)
		}
		for _, l := range operator.TiledLogs {
			logMessage += fmt.Sprintf("\t%s\n", l.SubmissionURL)
		}
	}
	klog.V(1).Info(logMessage)
	return compatible
}
