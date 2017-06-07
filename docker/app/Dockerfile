FROM ubuntu

ENV  GOPATH /opt/gopath
ENV  PATH $PATH:$GOPATH/src/github.com/ChainNova/exchange/app/

COPY payload/usr/lib/* /usr/lib/

COPY payload/bin/* $GOPATH/src/github.com/ChainNova/exchange/app/

WORKDIR $GOPATH/src/github.com/ChainNova/exchange/app

