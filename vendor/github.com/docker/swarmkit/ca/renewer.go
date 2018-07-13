package ca

import (
	"sync"
	"time"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/connectionbroker"
	"github.com/docker/swarmkit/log"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// RenewTLSExponentialBackoff sets the exponential backoff when trying to renew TLS certificates that have expired
var RenewTLSExponentialBackoff = events.ExponentialBackoffConfig{
	Base:   time.Second * 5,
	Factor: time.Second * 5,
	Max:    1 * time.Hour,
}

// TLSRenewer handles renewing TLS certificates, either automatically or upon
// request.
type TLSRenewer struct {
	mu           sync.Mutex
	s            *SecurityConfig
	connBroker   *connectionbroker.Broker
	renew        chan struct{}
	expectedRole string
	rootPaths    CertPaths
}

// NewTLSRenewer creates a new TLS renewer. It must be started with Start.
func NewTLSRenewer(s *SecurityConfig, connBroker *connectionbroker.Broker, rootPaths CertPaths) *TLSRenewer {
	return &TLSRenewer{
		s:          s,
		connBroker: connBroker,
		renew:      make(chan struct{}, 1),
		rootPaths:  rootPaths,
	}
}

// SetExpectedRole sets the expected role. If a renewal is forced, and the role
// doesn't match this expectation, renewal will be retried with exponential
// backoff until it does match.
func (t *TLSRenewer) SetExpectedRole(role string) {
	t.mu.Lock()
	t.expectedRole = role
	t.mu.Unlock()
}

// Renew causes the TLSRenewer to renew the certificate (nearly) right away,
// instead of waiting for the next automatic renewal.
func (t *TLSRenewer) Renew() {
	select {
	case t.renew <- struct{}{}:
	default:
	}
}

// Start will continuously monitor for the necessity of renewing the local certificates, either by
// issuing them locally if key-material is available, or requesting them from a remote CA.
func (t *TLSRenewer) Start(ctx context.Context) <-chan CertificateUpdate {
	updates := make(chan CertificateUpdate)

	go func() {
		var (
			retry      time.Duration
			forceRetry bool
		)
		expBackoff := events.NewExponentialBackoff(RenewTLSExponentialBackoff)
		defer close(updates)
		for {
			ctx = log.WithModule(ctx, "tls")
			log := log.G(ctx).WithFields(logrus.Fields{
				"node.id":   t.s.ClientTLSCreds.NodeID(),
				"node.role": t.s.ClientTLSCreds.Role(),
			})
			// Our starting default will be 5 minutes
			retry = 5 * time.Minute

			// Since the expiration of the certificate is managed remotely we should update our
			// retry timer on every iteration of this loop.
			// Retrieve the current certificate expiration information.
			validFrom, validUntil, err := readCertValidity(t.s.KeyReader())
			if err != nil {
				// We failed to read the expiration, let's stick with the starting default
				log.Errorf("failed to read the expiration of the TLS certificate in: %s", t.s.KeyReader().Target())

				select {
				case updates <- CertificateUpdate{Err: errors.New("failed to read certificate expiration")}:
				case <-ctx.Done():
					log.Info("shutting down certificate renewal routine")
					return
				}
			} else {
				// If we have an expired certificate, try to renew immediately: the hope that this is a temporary clock skew, or
				// we can issue our own TLS certs.
				if validUntil.Before(time.Now()) {
					log.Warn("the current TLS certificate is expired, so an attempt to renew it will be made immediately")
					// retry immediately(ish) with exponential backoff
					retry = expBackoff.Proceed(nil)
				} else if forceRetry {
					// A forced renewal was requested, but did not succeed yet.
					// retry immediately(ish) with exponential backoff
					retry = expBackoff.Proceed(nil)
				} else {
					// Random retry time between 50% and 80% of the total time to expiration
					retry = calculateRandomExpiry(validFrom, validUntil)
				}
			}

			log.WithFields(logrus.Fields{
				"time": time.Now().Add(retry),
			}).Debugf("next certificate renewal scheduled for %v from now", retry)

			select {
			case <-time.After(retry):
				log.Info("renewing certificate")
			case <-t.renew:
				forceRetry = true
				log.Info("forced certificate renewal")

				// Pause briefly before attempting the renewal,
				// to give the CA a chance to reconcile the
				// desired role.
				select {
				case <-time.After(500 * time.Millisecond):
				case <-ctx.Done():
					log.Info("shutting down certificate renewal routine")
					return
				}
			case <-ctx.Done():
				log.Info("shutting down certificate renewal routine")
				return
			}

			// ignore errors - it will just try again later
			var certUpdate CertificateUpdate
			if err := RenewTLSConfigNow(ctx, t.s, t.connBroker, t.rootPaths); err != nil {
				certUpdate.Err = err
				expBackoff.Failure(nil, nil)
			} else {
				newRole := t.s.ClientTLSCreds.Role()
				t.mu.Lock()
				expectedRole := t.expectedRole
				t.mu.Unlock()
				if expectedRole != "" && expectedRole != newRole {
					expBackoff.Failure(nil, nil)
					continue
				}

				certUpdate.Role = newRole
				expBackoff.Success(nil)
				forceRetry = false
			}

			select {
			case updates <- certUpdate:
			case <-ctx.Done():
				log.Info("shutting down certificate renewal routine")
				return
			}
		}
	}()

	return updates
}
