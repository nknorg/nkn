package commands

import (
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/portmapper"
)

var (
	gateway *portmapper.PortMapper
)

func SetupPortMapping() error {
	if config.SkipNAT || !config.Parameters.NAT {
		log.Infof("Skip automatic port forwading. You need to set up port forwarding and firewall yourself.")
		return nil
	}

	log.Infof("Discovering NAT gateway...")

	var err error
	gateway, err = portmapper.Discover()
	if err == portmapper.NoGatewayFound {
		log.Infof("No NAT gateway discovered, skip automatic port forwading. You need to set up port forwarding and firewall yourself.")
		return nil
	}
	if err != nil {
		return err
	}

	err = gateway.Add(config.Parameters.NodePort, "NKN Node")
	if err != nil {
		return err
	}
	log.Infof("Mapped external port %d to internal port %d", config.Parameters.NodePort, config.Parameters.NodePort)

	err = gateway.Add(config.Parameters.HttpWsPort, "NKN Node")
	if err != nil {
		return err
	}
	log.Infof("Mapped external port %d to internal port %d", config.Parameters.HttpWsPort, config.Parameters.HttpWsPort)

	err = gateway.Add(config.Parameters.HttpWssPort, "NKN Node")
	if err != nil {
		return err
	}
	log.Infof("Mapped external port %d to internal port %d", config.Parameters.HttpWssPort, config.Parameters.HttpWssPort)

	err = gateway.Add(config.Parameters.HttpJsonPort, "NKN Node")
	if err != nil {
		return err
	}
	log.Infof("Mapped external port %d to internal port %d", config.Parameters.HttpJsonPort, config.Parameters.HttpJsonPort)

	err = gateway.Add(config.Parameters.HttpsJsonPort, "NKN Node")
	if err != nil {
		return err
	}
	log.Infof("Mapped external port %d to internal port %d", config.Parameters.HttpsJsonPort, config.Parameters.HttpsJsonPort)

	return nil
}

func ClearPortMapping() error {
	if gateway == nil {
		return nil
	}

	log.Infof("Removing added port mapping...")

	return gateway.DeleteAll()
}
