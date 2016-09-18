package tlsutil

import "crypto/tls"

// ShallowCopyTLSConfig copies *tls.Config. This is only
// work-around for go-vet tests, which complains
//
//   assignment copies lock value to p: crypto/tls.Config contains sync.Once contains sync.Mutex
//
// Keep up-to-date with 'go/src/crypto/tls/common.go'
//
// (etcd pkg.transport.ShallowCopyTLSConfig)
func ShallowCopyTLSConfig(cfg *tls.Config) *tls.Config {
	ncfg := tls.Config{
		Time:                        cfg.Time,
		Certificates:                cfg.Certificates,
		NameToCertificate:           cfg.NameToCertificate,
		GetCertificate:              cfg.GetCertificate,
		RootCAs:                     cfg.RootCAs,
		NextProtos:                  cfg.NextProtos,
		ServerName:                  cfg.ServerName,
		ClientAuth:                  cfg.ClientAuth,
		ClientCAs:                   cfg.ClientCAs,
		InsecureSkipVerify:          cfg.InsecureSkipVerify,
		CipherSuites:                cfg.CipherSuites,
		PreferServerCipherSuites:    cfg.PreferServerCipherSuites,
		SessionTicketKey:            cfg.SessionTicketKey,
		ClientSessionCache:          cfg.ClientSessionCache,
		MinVersion:                  cfg.MinVersion,
		MaxVersion:                  cfg.MaxVersion,
		DynamicRecordSizingDisabled: cfg.DynamicRecordSizingDisabled,
		Renegotiation:               cfg.Renegotiation,
		CurvePreferences:            cfg.CurvePreferences,
	}
	return &ncfg
}
