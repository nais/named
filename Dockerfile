FROM alpine:3.8
MAINTAINER Sten Røkke <sten.ivar.rokke@nav.no>

COPY webproxy.crt /usr/local/share/ca-certificates/
RUN apk add --no-cache ca-certificates
RUN	update-ca-certificates

WORKDIR /app

COPY named .

CMD /app/named --fasitUrl=$fasit_url --clusterName=$cluster_name --logtostderr=true
