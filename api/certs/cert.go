package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/go-acme/lego/v3/certcrypto"
	"github.com/go-acme/lego/v3/certificate"
	"github.com/go-acme/lego/v3/lego"

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util"
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
	bundle, err := os.ReadFile(certPath)
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
	err = certs[0].VerifyHostname(domain)
	if err != nil {
		return false, err
	}
	return true, nil
}

func VerifyLocalCertificate(certPath string, keyPath string, domain string) (bool, error) {
	if len(certPath) > 0 && len(keyPath) > 0 && len(domain) > 0 && common.FileExisted(certPath) && common.FileExisted(keyPath) {
		return VerifyCertificate(certPath, keyPath, domain)
	}
	return false, nil
}

func CheckUserCertAvailable() (bool, bool) {
	// check preset user certificates
	wssUserCertAvailable, err := VerifyLocalCertificate(config.Parameters.HttpWssCert, config.Parameters.HttpWssKey, config.Parameters.HttpWssDomain)
	if err != nil {
		log.Error(err)
	}
	if wssUserCertAvailable {
		ok, err := domainDNSCheck(config.Parameters.HttpWssDomain)
		if err != nil {
			log.Error(err)
		}
		wssUserCertAvailable = ok
	}
	httpsUserCertAvailable, err := VerifyLocalCertificate(config.Parameters.HttpsJsonCert, config.Parameters.HttpsJsonKey, config.Parameters.HttpsJsonDomain)
	if err != nil {
		log.Error(err)
	}
	if httpsUserCertAvailable {
		ok, err := domainDNSCheck(config.Parameters.HttpsJsonDomain)
		if err != nil {
			log.Error(err)
		}
		httpsUserCertAvailable = ok
	}

	return wssUserCertAvailable, httpsUserCertAvailable
}

func domainDNSCheck(domain string) (bool, error) {
	if domain == config.Parameters.CertDomainName {
		return true, nil
	}
	ip, err := net.LookupIP(domain)
	if err != nil {
		return false, err
	}
	if len(ip) == 0 {
		return false, errors.New("incorrect dns record")
	}
	return ip[0].String() == config.Parameters.CertDomainName, nil
}

func PrepareCerts() (chan struct{}, chan struct{}, error) {
	wssCertReady := make(chan struct{}, 1)
	httpsCertReady := make(chan struct{}, 1)
	wssCertAvailable, httpsCertAvailable := CheckUserCertAvailable()
	if !httpsCertAvailable || !wssCertAvailable {
		defaultCertAvailable, err := SetDefaultDomain(httpsCertAvailable, wssCertAvailable)
		if err != nil {
			return nil, nil, err
		}
		wssCertAvailable = wssCertAvailable || defaultCertAvailable
		httpsCertAvailable = httpsCertAvailable || defaultCertAvailable
		go prepareCerts(wssCertAvailable, httpsCertAvailable, wssCertReady, httpsCertReady)
	}
	if wssCertAvailable {
		go func() {
			wssCertReady <- struct{}{}
		}()
	}
	if httpsCertAvailable {
		go func() {
			httpsCertReady <- struct{}{}
		}()
	}

	return wssCertReady, httpsCertReady, nil
}

func prepareCerts(wssCertAvailable, httpsCertAvailable bool, wssCertReady, httpsCertReady chan struct{}) {
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
			if !wssCertAvailable {
				select {
				case wssCertReady <- struct{}{}:
					log.Info("wss cert ready")
				default:
					log.Info("no wss cert available yet")
				}
			}
			if !httpsCertAvailable {
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

func SetDefaultDomain(https, wss bool) (bool, error) {
	var domainList []string
	var defaultCertAvailable bool
	for _, d := range config.Parameters.DefaultTlsDomainTmpl {
		domain, err := util.GetDefaultDomainFromIP(config.Parameters.CertDomainName, d)
		if err != nil {
			log.Error(err)
			continue
		}
		ok, err := domainDNSCheck(domain)
		if err != nil {
			log.Error(err)
			continue
		}
		if ok {
			domainList = append(domainList, domain)
			_, _, certFile, keyFile := GetACMEFileNames(domain, config.Parameters.CertDirectory)
			defaultCertAvailable, err = VerifyLocalCertificate(certFile, keyFile, domain)
			if err != nil {
				log.Error(err)
				continue
			}
			if defaultCertAvailable {
				domainList = domainList[len(domainList)-1:]
				break
			}
		}
	}
	if len(domainList) == 0 {
		return false, errors.New("no domain names available")
	}
	config.Parameters.CertDomainName = domainList[0]

	userFile, resourceFile, certFile, keyFile := GetACMEFileNames(config.Parameters.CertDomainName, config.Parameters.CertDirectory)
	log.Infof("set default cert domain to: %s", config.Parameters.CertDomainName)
	config.Parameters.DefaultTlsCert = certFile
	config.Parameters.DefaultTlsKey = keyFile
	config.Parameters.ACMEUserFile = userFile
	config.Parameters.ACMEResourceFile = resourceFile
	if !https {
		log.Info("use default https certs")
		config.Parameters.HttpsJsonCert = config.Parameters.DefaultTlsCert
		config.Parameters.HttpsJsonKey = config.Parameters.DefaultTlsKey
		config.Parameters.HttpsJsonDomain = config.Parameters.CertDomainName
	}
	if !wss {
		log.Info("use default wss certs")
		config.Parameters.HttpWssCert = config.Parameters.DefaultTlsCert
		config.Parameters.HttpWssKey = config.Parameters.DefaultTlsKey
		config.Parameters.HttpWssDomain = config.Parameters.CertDomainName
	}
	return defaultCertAvailable, nil
}
