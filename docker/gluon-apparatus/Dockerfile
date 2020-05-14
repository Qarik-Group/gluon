FROM alpine:3 AS build 

RUN apk update \
 && apk add curl git

RUN curl -sLo /usr/bin/bosh https://github.com/cloudfoundry/bosh-cli/releases/download/v6.2.1/bosh-cli-6.2.1-linux-amd64 \
 && chmod 0755 /usr/bin/bosh

RUN mkdir -p /bosh \
 && git clone https://github.com/cloudfoundry/bosh-deployment /bosh/deployment

#################################################
FROM ubuntu:18.04
COPY --from=build /usr/bin/bosh /usr/bin/bosh
COPY --from=build /bosh /bosh

RUN apt-get update \
 && apt-get install -y ca-certificates build-essential \
 && apt-get install -y git tmux tree pwgen unzip nmap build-essential ruby \
        zlibc zlib1g-dev ruby-dev openssl libxslt-dev libxml2-dev libssl-dev libyaml-dev \
        libsqlite3-dev sqlite3

RUN apt-get install -y libcurl3
RUN apt-get install -y curl
RUN curl -Lo /usr/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl \
 && chmod 0755 /usr/bin/kubectl

COPY deploy    /usr/bin/deploy
COPY teardown  /usr/bin/teardown
COPY envwrap   /usr/bin/envwrap

VOLUME /bosh/deployment
WORKDIR /bosh/deployment
CMD ["bosh"]
