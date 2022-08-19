FROM golang:1.18 as disco-build
ADD . /go/src/github.com/m-lab/disco
RUN cd /go/src/github.com/m-lab/disco && ./build.sh

# Now copy the built image into the minimal base image
FROM alpine:3.15
COPY --from=disco-build /go/src/github.com/m-lab/disco/disco /
WORKDIR /
ENTRYPOINT ["/disco"]
