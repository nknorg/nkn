FROM debian:stable-20180426
LABEL maintainer="gdmmx@nkn.org"

# apt-get
RUN apt-get update
RUN apt-get upgrade -y
RUN apt-get install curl make golang-glide -y

# for Dev only
RUN apt-get install lrzsz jq lsof psmisc -y

# golang
RUN curl https://dl.google.com/go/go1.10.1.linux-amd64.tar.gz | tar -zx -C /usr/local
ENV GOROOT=/usr/local/go
ENV PATH=$GOROOT/bin:$PATH
ENV GOPATH=/nknorg

# Set /etc/progile
RUN echo -e "\n### gOlang env" >> /etc/profile
RUN echo "export PATH=$GOROOT/bin:$PATH" >> /etc/profile
RUN echo "export GOROOT=/usr/local/go" >> /etc/profile
RUN echo "export GOPATH=/nknorg" >> /etc/profile

# nkn src & build
RUN mkdir -p /nknorg/src/github.com/nknorg/nkn
ADD ./ /nknorg/src/github.com/nknorg/nkn/
#RUN cd /nknorg/src/github.com/nknorg/nkn
#RUN make vendor
#RUN make all
