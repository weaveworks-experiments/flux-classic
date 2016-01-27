FROM dwragg/prometheus:etcd-sd
COPY ./prometheus-wrapper /bin/prometheus-wrapper
COPY ./prometheus.yml /etc/prometheus/prometheus.yml
ENTRYPOINT [ "/bin/prometheus-wrapper" ]
CMD        [ "-config.file=/etc/prometheus/prometheus.yml", \
             "-storage.local.path=/prometheus", \
             "-web.console.libraries=/etc/prometheus/console_libraries", \
             "-web.console.templates=/etc/prometheus/consoles" ]
