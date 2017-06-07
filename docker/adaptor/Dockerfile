FROM ubuntu

ENV  GOPATH /opt/gopath
ENV  PATH $PATH:$GOPATH/src/github.com/ChainNova/exchange/adaptor/

COPY payload/usr/lib/* /usr/lib/

COPY payload/bin/* $GOPATH/src/github.com/ChainNova/exchange/adaptor/

WORKDIR $GOPATH/src/github.com/ChainNova/exchange/adaptor

