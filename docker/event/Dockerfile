FROM ubuntu

ENV  GOPATH /opt/gopath
ENV  PATH $PATH:$GOPATH/src/github.com/ChainNova/exchange/event/

COPY payload/usr/lib/* /usr/lib/

COPY payload/bin/* $GOPATH/src/github.com/ChainNova/exchange/event/

WORKDIR $GOPATH/src/github.com/ChainNova/exchange/event

