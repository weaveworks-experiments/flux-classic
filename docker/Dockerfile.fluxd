FROM gliderlabs/alpine
ENTRYPOINT ["/home/flux/fluxd"]
RUN apk add --update iptables \
  && rm -rf /var/cache/apk/*
COPY ./fluxd /home/flux/
