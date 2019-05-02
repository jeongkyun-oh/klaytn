FROM kjhman21/dev:go1.11.2-solc0.4.24
MAINTAINER Jesse Lee jesse.lee@groundx.xyz

ENV PKG_DIR /klaytn-docker-pkg
ENV SRC_DIR /go/src/github.com/ground-x/klaytn

RUN mkdir -p $PKG_DIR/bin
RUN mkdir -p $PKG_DIR/conf

ADD . $SRC_DIR
RUN cd $SRC_DIR && make all

RUN cp $SRC_DIR/build/bin/* /usr/bin/

# packaging
RUN cp $SRC_DIR/build/bin/* $PKG_DIR/bin/

RUN cp $SRC_DIR/build/packaging/linux/bin/* $PKG_DIR/bin/

RUN cp $SRC_DIR/build/packaging/linux/conf/* $PKG_DIR/conf/

EXPOSE 8551 8552 32323 61001 32323/udp
