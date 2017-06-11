FROM ubuntu

ENV  GOPATH /opt/gopath
ENV  PATH $PATH:$GOPATH/src/github.com/ChainNova/exchange/adaptor/

COPY payload/usr/lib/* /usr/lib/

COPY payload/bin/* $GOPATH/src/github.com/ChainNova/exchange/adaptor/
COPY payload/core.yaml $GOPATH/src/github.com/hyperledger/fabric/peer/

WORKDIR $GOPATH/src/github.com/ChainNova/exchange/adaptor

