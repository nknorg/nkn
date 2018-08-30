FROM golang:1.10-stretch
LABEL maintainer="gdmmx@nkn.org"

# apt-get
RUN apt-get update && apt-get upgrade -y

# for Dev only
RUN apt-get install lrzsz jq lsof psmisc -y

# Set environment variables
ENV GOROOT=/usr/local/go
ENV PATH=$GOROOT/bin:$PATH
ENV GOPATH=/go
RUN echo -e "\n### Golang env" >> /etc/profile
RUN echo "export GOROOT=/usr/local/go" >> /etc/profile
RUN echo "export PATH=$GOROOT/bin:$PATH" >> /etc/profile
RUN echo "export GOPATH=/go" >> /etc/profile

ADD . /go/src/github.com/nknorg/nkn
WORKDIR /go/src/github.com/nknorg/nkn
RUN make glide
RUN make vendor
RUN make
RUN cp nknd nknc /usr/local/go/bin/

WORKDIR /nkn
