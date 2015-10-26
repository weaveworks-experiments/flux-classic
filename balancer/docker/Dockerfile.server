FROM gliderlabs/alpine
ENTRYPOINT ["/home/ambergris/server"]
RUN apk add --update iptables \
  && rm -rf /var/cache/apk/*
COPY ./ambergris /home/ambergris/server
