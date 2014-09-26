FROM google/debian:wheezy

RUN apt-get update -y && apt-get install --no-install-recommends -y -q curl build-essential ca-certificates git mercurial bzr

# Install Go 1.3.2
RUN curl -s https://storage.googleapis.com/golang/go1.3.2.linux-amd64.tar.gz | tar -v -C /usr/local -xz
ENV GOPATH /go
ENV GOROOT /usr/local/go
ENV PATH /usr/local/go/bin:/go/bin:/usr/local/bin:$PATH

# Install broadcast
RUN \
  mkdir -p $GOPATH/src/github.com/nyxtom/broadcast && \
  cd $GOPATH/src/github.com/nyxtom/broadcast && \
  git clone https://github.com/nyxtom/broadcast.git && \
  cd broadcast && make

EXPOSE 7331
CMD $GOPATH/bin/broadcast-server -config=/etc/broadcast.conf
