FROM gliderlabs/alpine
ENTRYPOINT ["/bin/coatlctl"]

COPY command.bin /bin/coatlctl
