package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/go-acme/lego/v3/certcrypto"
	"github.com/go-acme/lego/v3/certificate"
	"github.com/go-acme/lego/v3/lego"

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/log"
)

var (
	certFileMode = os.FileMode(0600)
	certDirMode  = os.FileMode(0755)
)

type certCache struct {
	Domain            string `json:"domain"`
	CertURL           string `json:"certUrl"`
	CertStableURL     string `json:"certStableUrl"`
	PrivateKey        []byte `json:"privateKey"`
	Certificate       []byte `json:"certificate"`
	IssuerCertificate []byte `json:"issuerCertificate"`
	CSR               []byte `json:"csr"`
}

func (c *certCache) toResource() *certificate.Resource {
	return &certificate.Resource{
		Domain:            c.Domain,
		CertURL:           c.CertURL,
		CertStableURL:     c.CertStableURL,
		PrivateKey:        c.PrivateKey,
		Certificate:       c.Certificate,
		IssuerCertificate: c.IssuerCertificate,
		CSR:               c.CSR,
	}
}

func VerifyOCSP(bundle []byte) (bool, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return false, err
	}

	myUser := User{
		Key: privateKey,
	}

	c := lego.NewConfig(&myUser)

	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(c)
	if err != nil {
		log.Error(err)
		return false, err
	}
	_, oscp, err := client.Certificate.GetOCSP(bundle)
	if err != nil {
		return false, err
	}
	if oscp.Status == 0 {
		return true, nil
	}
	return false, fmt.Errorf("cert did not pass OCSP check")
}

func VerifyCertificate(certPath, keyPath, domain string) (bool, error) {
	_, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return false, err
	}
	bundle, err := ioutil.ReadFile(certPath)
	if err != nil {
		return false, err
	}
	unRevoked, err := VerifyOCSP(bundle)
	if !unRevoked {
		return false, err
	}
	certs, err := certcrypto.ParsePEMBundle(bundle)
	if err != nil {
		return false, err
	}
	if certs[0].Subject.CommonName != domain {
		return false, fmt.Errorf("inconsistent domain names")
	}
	return true, nil
}

func VerifyLocalCertificate(certPath string, keyPath string, domain string) (bool, error) {
	if len(certPath) > 0 && len(keyPath) > 0 && len(domain) > 0 && common.FileExisted(certPath) && common.FileExisted(keyPath) {
		return VerifyCertificate(certPath, keyPath, domain)
	}
	return false, nil
}

func CertExists() (bool, bool, bool) {
	wssUserCertExists, err := VerifyLocalCertificate(config.Parameters.HttpWssCert, config.Parameters.HttpWssKey, config.Parameters.HttpWssDomain)
	if err != nil {
		log.Error(err)
	}
	httpsUserCertExists, err := VerifyLocalCertificate(config.Parameters.HttpsJsonCert, config.Parameters.HttpsJsonKey, config.Parameters.HttpsJsonDomain)
	if err != nil {
		log.Error(err)
	}
	defaultDomain, err := node.GetDefaultDomainFromIP(config.Parameters.Hostname, config.Parameters.DefaultTlsDomainTmpl)
	if err != nil {
		log.Error(err)
	}
	_, _, certFile, keyFile := GetACMEFileNames(defaultDomain, config.Parameters.CertDirectory)
	defaultCertExists, err := VerifyLocalCertificate(certFile, keyFile, defaultDomain)
	if err != nil {
		log.Error(err)
	}

	return wssUserCertExists, httpsUserCertExists, defaultCertExists
}

func PrepareCerts() (chan struct{}, chan struct{}) {
	wssCertExists, httpsCertExists, defaultCertExists := CertExists()
	ChangeToDefault(httpsCertExists, wssCertExists)

	wssCertReady := make(chan struct{}, 1)
	httpsCertReady := make(chan struct{}, 1)

	if wssCertExists || defaultCertExists {
		go func() {
			wssCertReady <- struct{}{}
		}()
	}
	if httpsCertExists || defaultCertExists {
		go func() {
			httpsCertReady <- struct{}{}
		}()
	}
	if !(wssCertExists && httpsCertExists) {
		go prepareCerts(wssCertExists, httpsCertExists, wssCertReady, httpsCertReady)
	}
	return wssCertReady, httpsCertReady
}

func prepareCerts(wssCertExists, httpsCertExists bool, wssCertReady, httpsCertReady chan struct{}) {
	first := true
	for {
		cm, err := NewCertManager()
		if err != nil {
			log.Errorf("create new certManager err: %v", err)
			return
		}
		if !first {
			time.Sleep(config.Parameters.CertCheckInterval * time.Second)
		}
		first = false
		if cm.NeedObtain() {
			cert, err := cm.ObtainNewCert()
			if err != nil {
				log.Errorf("apply cert failed: %v", err)
				continue
			}
			err = saveCertToDisk(cert)
			if err != nil {
				log.Errorf("save cert failed: %v", err)
				continue
			}
			if !wssCertExists {
				select {
				case wssCertReady <- struct{}{}:
					log.Info("wss cert ready")
				default:
					log.Info("no wss cert available yet")
				}
			}
			if !httpsCertExists {
				select {
				case httpsCertReady <- struct{}{}:
					log.Info("https cert ready")
				default:
					log.Info("no https cert available yet")
				}
			}
		} else if cm.NeedRenew() {
			renew, err := cm.RenewCert()
			if err != nil {
				log.Errorf("renew cert failed: %v", err)
				continue
			}
			err = saveCertToDisk(renew)
			if err != nil {
				log.Errorf("save renew cert failed: %v", err)
				continue
			}
		}
	}
}

func ChangeToDefault(https, wss bool) {
	defaultDomain, err := node.GetDefaultDomainFromIP(config.Parameters.Hostname, config.Parameters.DefaultTlsDomainTmpl)
	if err != nil {
		log.Error(err)
	}

	userFile, resourceFile, certFile, keyFile := GetACMEFileNames(defaultDomain, config.Parameters.CertDirectory)
	log.Infof("set default cert domain to: %s", defaultDomain)
	config.Parameters.DefaultTlsCert = certFile
	config.Parameters.DefaultTlsKey = keyFile
	config.Parameters.ACMEUserFile = userFile
	config.Parameters.ACMEResourceFile = resourceFile
	if !https {
		log.Info("use default https certs")
		config.Parameters.HttpsJsonCert = config.Parameters.DefaultTlsCert
		config.Parameters.HttpsJsonKey = config.Parameters.DefaultTlsKey
		config.Parameters.HttpsJsonDomain = config.Parameters.DefaultTlsDomainTmpl
	}
	if !wss {
		log.Info("use default wss certs")
		config.Parameters.HttpWssCert = config.Parameters.DefaultTlsCert
		config.Parameters.HttpWssKey = config.Parameters.DefaultTlsKey
		config.Parameters.HttpWssDomain = config.Parameters.DefaultTlsDomainTmpl
	}
}
