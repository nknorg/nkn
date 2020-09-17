package certs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/go-acme/lego/v3/certcrypto"
	"github.com/go-acme/lego/v3/certificate"
	"github.com/go-acme/lego/v3/challenge/http01"
	"github.com/go-acme/lego/v3/lego"
	"github.com/go-acme/lego/v3/registration"

	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/log"
)

type certManager struct {
	User       *User
	LegoConfig *lego.Config
	LegoClient *lego.Client
	Resource   *certificate.Resource
}

func NewCertManager() (*certManager, error) {
	u, err := GetUser()
	if err != nil {
		return nil, err
	}

	c := lego.NewConfig(u)
	c.CADirURL = lego.LEDirectoryProduction
	c.Certificate.KeyType = certcrypto.RSA2048

	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(c)
	if err != nil {
		return nil, err
	}
	if u.Registration == nil {
		// Register Client and agree to TOS
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, err
		}
		u.Registration = reg
		log.Info("client registration complete")
	}
	err = u.SaveToDisk()
	if err != nil {
		return nil, err
	}
	cache := certCache{}
	resBytes, err := ioutil.ReadFile(config.Parameters.ACMEResourceFile)
	if err == nil {
		err = json.Unmarshal(resBytes, &cache)
		if err != nil {
			return nil, err
		}
	} else {
		log.Info("read resourceFile err:", err)
	}

	return &certManager{User: u, LegoConfig: c, LegoClient: client, Resource: cache.toResource()}, nil
}

func (cm *certManager) ObtainNewCert() (*certificate.Resource, error) {
	err := cm.LegoClient.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80"))
	if err != nil {
		return nil, err
	}

	request := certificate.ObtainRequest{
		Domains: []string{cm.User.Domain},
		Bundle:  true,
	}
	cm.Resource, err = cm.LegoClient.Certificate.Obtain(request)
	if err != nil {
		return nil, err
	}

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private Key, and a certificate URL. SAVE THESE TO DISK.
	return cm.Resource, nil
}

func (cm *certManager) RenewCert() (*certificate.Resource, error) {
	certificates, err := certcrypto.ParsePEMBundle(cm.Resource.Certificate)
	if err != nil {
		return nil, err
	}
	err = cm.LegoClient.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80"))
	if err != nil {
		return nil, err
	}
	// check if first cert is CA
	x509Cert := certificates[0]
	if x509Cert.IsCA {
		return nil, fmt.Errorf("[%s] certificate bundle starts with a CA certificate", cm.Resource.Domain)
	}

	timeLeft := x509Cert.NotAfter.Sub(time.Now().UTC())
	log.Infof("[%s] acme: %d hours remaining, renewBefore: %d\n", cm.Resource.Domain, int(timeLeft.Hours()), config.Parameters.CertRenewBefore)

	cert, err := cm.LegoClient.Certificate.Renew(*cm.Resource, true, false)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

func (cm *certManager) NeedObtain() bool {
	if cm.Resource == nil || len(cm.Resource.Domain) == 0 {
		return true
	}
	return false
}

func (cm *certManager) NeedRenew() bool {
	certificates, err := certcrypto.ParsePEMBundle(cm.Resource.Certificate)
	if err != nil {
		log.Errorf("failed to parsePEMBundle: %s", err)
		return false
	}
	if len(certificates) == 0 {
		log.Info("no certs found")
		return false
	}
	// check if first cert is CA
	x509Cert := certificates[0]
	if x509Cert.IsCA {
		log.Errorf("%s certificate bundle starts with a CA certificate", x509Cert.DNSNames)
		return false
	}
	// Calculate TimeLeft
	timeLeft := x509Cert.NotAfter.Sub(time.Now().UTC())
	return int(timeLeft.Hours()) < int(config.Parameters.CertRenewBefore)
}

func GetACMEFileNames(domainName string, dirName string) (string, string, string, string) {
	certSuffix := ".cert"
	keySuffix := ".key"
	userFile := filepath.Join(dirName, domainName+".user.json")
	resourceFile := filepath.Join(dirName, domainName+".resource.json")
	certFile := filepath.Join(dirName, domainName+certSuffix)
	keyFile := filepath.Join(dirName, domainName+keySuffix)
	return userFile, resourceFile, certFile, keyFile
}

func saveCertToDisk(cert *certificate.Resource) error {
	// JSON encode certificate resource
	// needs to be a certCache otherwise the fields with the keys will be lost
	b, err := json.MarshalIndent(certCache{
		Domain:            cert.Domain,
		CertURL:           cert.CertURL,
		CertStableURL:     cert.CertStableURL,
		PrivateKey:        cert.PrivateKey,
		Certificate:       cert.Certificate,
		IssuerCertificate: cert.IssuerCertificate,
		CSR:               cert.CSR,
	}, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(config.Parameters.CertDirectory, certDirMode)
	// write certificate resource to disk
	err = ioutil.WriteFile(config.Parameters.ACMEResourceFile, b, certFileMode)
	if err != nil {
		return err
	}

	// write certificate PEM to disk
	err = ioutil.WriteFile(config.Parameters.DefaultTlsCert, cert.Certificate, certFileMode)
	if err != nil {
		return err
	}

	// write private Key PEM to disk
	err = ioutil.WriteFile(config.Parameters.DefaultTlsKey, cert.PrivateKey, certFileMode)
	if err != nil {
		return err
	}
	return nil
}
